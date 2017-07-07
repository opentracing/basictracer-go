package basictracer

import (
	"testing"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/harness"
)

// newTracer creates a new tracer for each test, and returns a nil cleanup function.
func newTracer() (tracer ot.Tracer, closer func()) {
	tracer = NewWithOptions(Options{
		Recorder:     NewInMemoryRecorder(),
		ShouldSample: func(traceID uint64) bool { return true }, // always sample
	})
	return tracer, nil
}

func TestAPICheck(t *testing.T) {
	harness.RunAPIChecks(t,
		newTracer,
		harness.CheckEverything(),
		harness.UseProbe(apiCheckProbe{}),
	)
}

// implements harness.APICheckProbe
type apiCheckProbe struct{}

// SameTrace helps tests assert that this tracer's spans are from the same trace.
func (apiCheckProbe) SameTrace(first, second ot.Span) bool {
	span1, ok := first.(*spanImpl)
	if !ok { // some other tracer's span
		return false
	}
	span2, ok := second.(*spanImpl)
	if !ok { // some other tracer's span
		return false
	}
	return span1.raw.Context.TraceID == span2.raw.Context.TraceID
}

// SameSpanContext helps tests assert that a span and a context are from the same trace and span.
func (apiCheckProbe) SameSpanContext(span ot.Span, spanContext ot.SpanContext) bool {
	sp, ok := span.(*spanImpl)
	if !ok {
		return false
	}
	ctx, ok := spanContext.(SpanContext)
	if !ok {
		return false
	}
	return sp.raw.Context.TraceID == ctx.TraceID && sp.raw.Context.SpanID == ctx.SpanID
}
