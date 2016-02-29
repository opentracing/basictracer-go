package basictracer

import (
	"fmt"
	"sync"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
)

// Implements the `Span` interface. Created via tracerImpl (see
// `basictracer.New()`).
type spanImpl struct {
	tracer     *tracerImpl
	sync.Mutex // protects the fields below
	raw        RawSpan
}

func (s *spanImpl) reset() {
	s.tracer = nil
	// Note: Would like to do the following, but then the consumer of RawSpan
	// (the recorder) needs to make sure that they're not holding on to the
	// baggage or logs when they return (i.e. they need to copy if they care):
	//
	// logs, baggage := s.raw.Logs[:0], s.raw.Baggage
	// for k := range baggage {
	// 	delete(baggage, k)
	// }
	// s.raw.Logs, s.raw.Baggage = logs, baggage
	//
	// That's likely too much to ask for. But there is some magic we should
	// be able to do with `runtime.SetFinalizer` to reclaim that memory into
	// a buffer pool when GC considers them unreachable, which should ease
	// some of the load. Hard to say how quickly that would be in practice
	// though.
	s.raw = RawSpan{}
}

func (s *spanImpl) SetOperationName(operationName string) opentracing.Span {
	s.Lock()
	defer s.Unlock()
	s.raw.Operation = operationName
	return s
}

func (s *spanImpl) trim() bool {
	return !s.raw.Sampled && s.tracer.TrimUnsampledSpans
}

func (s *spanImpl) SetTag(key string, value interface{}) opentracing.Span {
	s.Lock()
	defer s.Unlock()
	if key == string(ext.SamplingPriority) {
		s.raw.Sampled = true
		return s
	}
	if s.trim() {
		return s
	}

	if s.raw.Tags == nil {
		s.raw.Tags = opentracing.Tags{}
	}
	s.raw.Tags[key] = value
	return s
}

func (s *spanImpl) LogEvent(event string) {
	s.Log(opentracing.LogData{
		Event: event,
	})
}

func (s *spanImpl) LogEventWithPayload(event string, payload interface{}) {
	s.Log(opentracing.LogData{
		Event:   event,
		Payload: payload,
	})
}

func (s *spanImpl) Log(ld opentracing.LogData) {
	s.Lock()
	defer s.Unlock()
	if s.trim() {
		return
	}

	if ld.Timestamp.IsZero() {
		ld.Timestamp = time.Now()
	}

	s.raw.Logs = append(s.raw.Logs, ld)
}

func (s *spanImpl) Finish() {
	s.FinishWithOptions(opentracing.FinishOptions{})
}

func (s *spanImpl) FinishWithOptions(opts opentracing.FinishOptions) {
	finishTime := opts.FinishTime
	if finishTime.IsZero() {
		finishTime = time.Now()
	}
	duration := finishTime.Sub(s.raw.Start)

	s.Lock()
	if opts.BulkLogData != nil {
		s.raw.Logs = append(s.raw.Logs, opts.BulkLogData...)
	}
	s.raw.Duration = duration
	s.Unlock()

	s.tracer.Recorder.RecordSpan(s.raw)
	s.tracer.spanPool.Put(s)
}

func (s *spanImpl) SetBaggageItem(restrictedKey, val string) opentracing.Span {
	canonicalKey, valid := opentracing.CanonicalizeBaggageKey(restrictedKey)
	if !valid {
		panic(fmt.Errorf("Invalid key: %q", restrictedKey))
	}

	s.Lock()
	defer s.Unlock()
	if s.trim() {
		return s
	}

	if s.raw.Baggage == nil {
		s.raw.Baggage = make(map[string]string)
	}
	s.raw.Baggage[canonicalKey] = val
	return s
}

func (s *spanImpl) BaggageItem(restrictedKey string) string {
	canonicalKey, valid := opentracing.CanonicalizeBaggageKey(restrictedKey)
	if !valid {
		panic(fmt.Errorf("Invalid key: %q", restrictedKey))
	}

	s.Lock()
	defer s.Unlock()

	return s.raw.Baggage[canonicalKey]
}

func (s *spanImpl) Tracer() opentracing.Tracer {
	return s.tracer
}
