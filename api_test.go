package basictracer

import (
	"testing"

	ot "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/harness"
	"github.com/stretchr/testify/suite"
)

func TestAPICheck(t *testing.T) {
	apiSuite := harness.NewAPICheckSuite(func() (tracer ot.Tracer, closer func()) {
		tracer = NewWithOptions(Options{
			Recorder:     NewInMemoryRecorder(),
			ShouldSample: func(traceID uint64) bool { return true }, // always sample
		})
		return tracer, nil
	}, harness.APICheckCapabilities{
		CheckBaggageValues: true,
		CheckInject:        true,
		CheckExtract:       true,
		Probe:              apiCheckProbe{},
	})
	suite.Run(t, apiSuite)
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

// SameTraceContext helps tests assert that a span and a span context are from the same trace.
func (apiCheckProbe) SameTraceContext(span ot.Span, spanContext ot.SpanContext) bool {
	sp, ok := span.(*spanImpl)
	if !ok {
		return false
	}
	ctx, ok := spanContext.(SpanContext)
	if !ok {
		return false
	}
	return sp.raw.Context.TraceID == ctx.TraceID
}
