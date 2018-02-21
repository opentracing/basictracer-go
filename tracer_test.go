package basictracer

import (
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/harness"
)

type apiCheckProbe struct{}

func (apiCheckProbe) SameTrace(first, second opentracing.Span) bool {
	sp1, ok := first.(*spanImpl)
	if !ok {
		return false
	}
	sp2, ok := second.(*spanImpl)
	if !ok {
		return false
	}
	if sp1.raw.Context.TraceID == sp2.raw.Context.TraceID {
		return true
	}
	return false
}

func (apiCheckProbe) SameSpanContext(otSpan opentracing.Span, otCtx opentracing.SpanContext) bool {
	sp, ok := otSpan.(*spanImpl)
	if !ok {
		return false
	}
	sc, ok := otCtx.(SpanContext)
	if !ok {
		return false
	}
	if sp.raw.Context.TraceID == sc.TraceID {
		return true
	}
	return false
}

func TestTracerAPIChecks(t *testing.T) {
	harness.RunAPIChecks(t, func() (tracer opentracing.Tracer, closer func()) {
		return New(NewInMemoryRecorder()), nil
	},
		harness.CheckEverything(),
		harness.UseProbe(apiCheckProbe{}),
	)
}
