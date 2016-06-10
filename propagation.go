package basictracer

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

type accessorPropagator struct {
	tracer *tracerImpl
}

// DelegatingCarrier is a flexible carrier interface which can be implemented
// by types which have a means of storing the trace metadata and already know
// how to serialize themselves (for example, protocol buffers).
type DelegatingCarrier interface {
	SetState(traceID, spanID uint64, sampled bool)
	State() (traceID, spanID uint64, sampled bool)
	SetBaggageItem(key, value string)
	GetBaggage(func(key, value string))
}

func (p *accessorPropagator) Inject(
	spanContext opentracing.SpanContext,
	carrier interface{},
) error {
	dc, ok := carrier.(DelegatingCarrier)
	if !ok || dc == nil {
		return opentracing.ErrInvalidCarrier
	}
	sc, ok := spanContext.(*SpanContext)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	dc.SetState(sc.TraceID, sc.SpanID, sc.Sampled)
	for k, v := range sc.Baggage {
		dc.SetBaggageItem(k, v)
	}
	return nil
}

func (p *accessorPropagator) Join(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	dc, ok := carrier.(DelegatingCarrier)
	if !ok || dc == nil {
		return nil, opentracing.ErrInvalidCarrier
	}

	sp := p.tracer.getSpan()
	traceID, parentSpanID, sampled := dc.State()
	sp.raw.ParentSpanID = parentSpanID
	sp.raw.SpanContext = SpanContext{
		TraceID: traceID,
		SpanID:  randomID(),
		Sampled: sampled,
		Baggage: nil, // initialized just below.
	}
	dc.GetBaggage(func(k, v string) {
		if sp.raw.Baggage == nil {
			sp.raw.Baggage = map[string]string{}
		}
		sp.raw.Baggage[k] = v
	})

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}
