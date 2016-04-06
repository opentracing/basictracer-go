package basictracer

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

// Tracer extends the opentracing.Tracer interface with methods to
// probe implementation state, for use by basictracer consumers.
type Tracer interface {
	opentracing.Tracer

	// Options gets the Options used in New() or NewWithOptions().
	Options() Options
}

// Options allows creating a customized Tracer via NewWithOptions. The object
// must not be updated when there is an active tracer using it.
type Options struct {
	// ShouldSample is a function which is called when creating a new Span and
	// determines whether that Span is sampled. The randomized TraceID is supplied
	// to allow deterministic sampling decisions to be made across different nodes.
	// For example,
	//
	//   func(traceID uint64) { return traceID % 64 == 0 }
	//
	// samples every 64th trace on average.
	ShouldSample func(traceID uint64) bool
	// TrimUnsampledSpans turns potentially expensive operations on unsampled
	// Spans into no-ops. More precisely, tags, baggage items, and log events
	// are silently discarded. If NewSpanEventListener is set, the callbacks
	// will still fire in that case.
	TrimUnsampledSpans bool
	// Recorder receives Spans which have been finished.
	Recorder SpanRecorder
	// NewSpanEventListener can be used to enhance the tracer by effectively
	// attaching external code to trace events. See NetTraceIntegrator for a
	// practical example, and event.go for the list of possible events.
	NewSpanEventListener func() func(SpanEvent)
	// DebugAssertSingleGoroutine internally records the ID of the goroutine
	// creating each Span and verifies that no operation is carried out on
	// it on a different goroutine.
	// Provided strictly for development purposes.
	// Passing Spans between goroutine without proper synchronization often
	// results in use-after-Finish() errors. For a simple example, consider the
	// following pseudocode:
	//
	//  func (s *Server) Handle(req http.Request) error {
	//    sp := s.StartSpan("server")
	//    defer sp.Finish()
	//    wait := s.queueProcessing(opentracing.ContextWithSpan(context.Background(), sp), req)
	//    select {
	//    case resp := <-wait:
	//      return resp.Error
	//    case <-time.After(10*time.Second):
	//      sp.LogEvent("timed out waiting for processing")
	//      return ErrTimedOut
	//    }
	//  }
	//
	// This looks reasonable at first, but a request which spends more than ten
	// seconds in the queue is abandoned by the main goroutine and its trace
	// finished, leading to use-after-finish when the request is finally
	// processed. Note also that even joining on to a finished Span via
	// StartSpanWithOptions constitutes an illegal operation.
	//
	// Code bases which do not require (or decide they do not want) Spans to
	// be passed across goroutine boundaries can run with this flag enabled in
	// tests to increase their chances of spotting wrong-doers.
	DebugAssertSingleGoroutine bool
	// DebugAssertUseAfterFinish is provided strictly for development purposes.
	// When set, it attempts to exacerbate issues emanating from use of Spans
	// after calling Finish by running additional assertions.
	DebugAssertUseAfterFinish bool
}

// DefaultOptions returns an Options object with a 1 in 64 sampling rate and
// all options disabled. A Recorder needs to be set manually before using the
// returned object with a Tracer.
func DefaultOptions() Options {
	var opts Options
	opts.ShouldSample = func(traceID uint64) bool { return traceID%64 == 0 }
	opts.NewSpanEventListener = func() func(SpanEvent) { return nil }
	return opts
}

// NewWithOptions creates a customized Tracer.
func NewWithOptions(opts Options) opentracing.Tracer {
	rval := &tracerImpl{options: opts}
	rval.textPropagator = &textMapPropagator{rval}
	rval.binaryPropagator = &binaryPropagator{rval}
	rval.accessorPropagator = &accessorPropagator{rval}
	return rval
}

// New creates and returns a standard Tracer which defers completed Spans to
// `recorder`.
// Spans created by this Tracer support the ext.SamplingPriority tag: Setting
// ext.SamplingPriority causes the Span to be Sampled from that point on.
func New(recorder SpanRecorder) opentracing.Tracer {
	opts := DefaultOptions()
	opts.Recorder = recorder
	return NewWithOptions(opts)
}

// Implements the `Tracer` interface.
type tracerImpl struct {
	options            Options
	textPropagator     *textMapPropagator
	binaryPropagator   *binaryPropagator
	accessorPropagator *accessorPropagator
}

func (t *tracerImpl) StartSpan(
	operationName string,
) opentracing.Span {
	return t.StartSpanWithOptions(
		opentracing.StartSpanOptions{
			OperationName: operationName,
		})
}

func (t *tracerImpl) getSpan() *spanImpl {
	sp := spanPool.Get().(*spanImpl)
	sp.reset()
	return sp
}

func (t *tracerImpl) StartSpanWithOptions(
	opts opentracing.StartSpanOptions,
) opentracing.Span {
	// Start time.
	startTime := opts.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	// Tags.
	tags := opts.Tags

	// Build the new span. This is the only allocation: We'll return this as
	// a opentracing.Span.
	sp := t.getSpan()
	if opts.Parent == nil {
		sp.raw.TraceID, sp.raw.SpanID = randomID2()
		sp.raw.Sampled = t.options.ShouldSample(sp.raw.TraceID)
	} else {
		pr := opts.Parent.(*spanImpl)
		sp.raw.TraceID = pr.raw.TraceID
		sp.raw.SpanID = randomID()
		sp.raw.ParentSpanID = pr.raw.SpanID
		sp.raw.Sampled = pr.raw.Sampled

		pr.Lock()
		if l := len(pr.raw.Baggage); l > 0 {
			sp.raw.Baggage = make(map[string]string, len(pr.raw.Baggage))
			for k, v := range pr.raw.Baggage {
				sp.raw.Baggage[k] = v
			}
		}
		pr.Unlock()
	}

	return t.startSpanInternal(
		sp,
		opts.OperationName,
		startTime,
		tags,
	)
}

func (t *tracerImpl) startSpanInternal(
	sp *spanImpl,
	operationName string,
	startTime time.Time,
	tags opentracing.Tags,
) opentracing.Span {
	sp.tracer = t
	sp.event = t.options.NewSpanEventListener()
	sp.raw.Operation = operationName
	sp.raw.Start = startTime
	sp.raw.Duration = -1
	sp.raw.Tags = tags
	if t.options.DebugAssertSingleGoroutine {
		sp.SetTag(debugGoroutineIDTag, curGoroutineID())
	}
	defer sp.onCreate(operationName)
	return sp
}

type delegatorType struct{}

// Delegator is the format to use for DelegatingCarrier.
var Delegator delegatorType

func (t *tracerImpl) Inject(sp opentracing.Span, format interface{}, carrier interface{}) error {
	switch format {
	case opentracing.TextMap:
		return t.textPropagator.Inject(sp, carrier)
	case opentracing.Binary:
		return t.binaryPropagator.Inject(sp, carrier)
	}
	if _, ok := format.(delegatorType); ok {
		return t.accessorPropagator.Inject(sp, carrier)
	}
	return opentracing.ErrUnsupportedFormat
}

func (t *tracerImpl) Join(operationName string, format interface{}, carrier interface{}) (opentracing.Span, error) {
	switch format {
	case opentracing.TextMap:
		return t.textPropagator.Join(operationName, carrier)
	case opentracing.Binary:
		return t.binaryPropagator.Join(operationName, carrier)
	}
	if _, ok := format.(delegatorType); ok {
		return t.accessorPropagator.Join(operationName, carrier)
	}
	return nil, opentracing.ErrUnsupportedFormat
}

func (t *tracerImpl) Options() Options {
	return t.options
}
