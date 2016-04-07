package basictracer

import (
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

const (
	// InMemory stores a copy of a Span's state.
	//
	// Useful for passing along Span information across goroutines without
	// actually passing a live Span.
	// NOTE: This would go into the opentracing-go package. The constant number
	// is abritrary to prevent any classes with the OT consts.
	InMemory = 10
)

// InMemoryCarrier holds a copy of a Span's state. The underyling value can be
// of any arbritrary type. There is no expectation for interopability between
// different languages.
type InMemoryCarrier struct {
	TracerState interface{}
}

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
	sp opentracing.Span,
	carrier interface{},
) error {
	ac, ok := carrier.(DelegatingCarrier)
	if !ok || ac == nil {
		return opentracing.ErrInvalidCarrier
	}
	si, ok := sp.(*spanImpl)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	meta := si.raw.Context
	ac.SetState(meta.TraceID, meta.SpanID, meta.Sampled)
	for k, v := range si.raw.Context.Baggage {
		ac.SetBaggageItem(k, v)
	}
	return nil
}

func (p *accessorPropagator) Join(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	ac, ok := carrier.(DelegatingCarrier)
	if !ok || ac == nil {
		return nil, opentracing.ErrInvalidCarrier
	}

	sp := p.tracer.getSpan()

	traceID, parentSpanID, sampled := ac.State()

	sp.raw.Context = Context{
		TraceID:      traceID,
		SpanID:       randomID(),
		ParentSpanID: parentSpanID,
		Sampled:      sampled,
	}

	ac.GetBaggage(func(k, v string) {
		if sp.raw.Context.Baggage == nil {
			sp.raw.Context.Baggage = map[string]string{}
		}
		sp.raw.Context.Baggage[k] = v
	})

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}
