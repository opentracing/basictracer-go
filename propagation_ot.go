package basictracer

import (
	"encoding/binary"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/opentracing/basictracer-go/wire"
	opentracing "github.com/opentracing/opentracing-go"
)

type textMapPropagator struct {
	tracer *tracerImpl
}
type binaryPropagator struct {
	tracer *tracerImpl
}

const (
	prefixTracerState = "ot-tracer-"
	prefixBaggage     = "ot-baggage-"

	tracerStateFieldCount = 3
	fieldNameTraceID      = prefixTracerState + "traceid"
	fieldNameSpanID       = prefixTracerState + "spanid"
	fieldNameSampled      = prefixTracerState + "sampled"
)

func (p *textMapPropagator) Inject(
	sp opentracing.Span,
	opaqueCarrier interface{},
) error {
	sc, ok := sp.(*spanImpl)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	carrier, ok := opaqueCarrier.(opentracing.TextMapWriter)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	carrier.Set(fieldNameTraceID, strconv.FormatUint(sc.raw.TraceID, 16))
	carrier.Set(fieldNameSpanID, strconv.FormatUint(sc.raw.SpanID, 16))
	carrier.Set(fieldNameSampled, strconv.FormatBool(sc.raw.Sampled))

	sc.Lock()
	for k, v := range sc.raw.Baggage {
		carrier.Set(prefixBaggage+k, v)
	}
	sc.Unlock()
	return nil
}

func (p *textMapPropagator) Join(
	operationName string,
	opaqueCarrier interface{},
) (opentracing.Span, error) {
	carrier, ok := opaqueCarrier.(opentracing.TextMapReader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	requiredFieldCount := 0
	var traceID, propagatedSpanID uint64
	var sampled bool
	var err error
	decodedBaggage := make(map[string]string)
	err = carrier.ForeachKey(func(k, v string) error {
		switch strings.ToLower(k) {
		case fieldNameTraceID:
			traceID, err = strconv.ParseUint(v, 16, 64)
			if err != nil {
				return opentracing.ErrTraceCorrupted
			}
		case fieldNameSpanID:
			propagatedSpanID, err = strconv.ParseUint(v, 16, 64)
			if err != nil {
				return opentracing.ErrTraceCorrupted
			}
		case fieldNameSampled:
			sampled, err = strconv.ParseBool(v)
			if err != nil {
				return opentracing.ErrTraceCorrupted
			}
		default:
			lowercaseK := strings.ToLower(k)
			if strings.HasPrefix(lowercaseK, prefixBaggage) {
				decodedBaggage[strings.TrimPrefix(lowercaseK, prefixBaggage)] = v
			}
			// Balance off the requiredFieldCount++ just below...
			requiredFieldCount--
		}
		requiredFieldCount++
		return nil
	})
	if err != nil {
		return nil, err
	}
	if requiredFieldCount < tracerStateFieldCount {
		if requiredFieldCount == 0 {
			return nil, opentracing.ErrTraceNotFound
		}
		return nil, opentracing.ErrTraceCorrupted
	}

	sp := p.tracer.getSpan()
	sp.raw = RawSpan{
		Context: Context{
			TraceID:      traceID,
			SpanID:       randomID(),
			ParentSpanID: propagatedSpanID,
			Sampled:      sampled,
		},
		Baggage: decodedBaggage,
	}

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}

func (p *binaryPropagator) Inject(
	sp opentracing.Span,
	opaqueCarrier interface{},
) error {
	sc, ok := sp.(*spanImpl)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	carrier, ok := opaqueCarrier.(io.Writer)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}

	state := wire.TracerState{}
	state.TraceId = sc.raw.TraceID
	state.SpanId = sc.raw.SpanID
	state.Sampled = sc.raw.Sampled
	state.BaggageItems = sc.raw.Baggage

	b, err := proto.Marshal(&state)
	if err != nil {
		return err
	}

	// Write the length of the marshalled binary to the writer.
	length := uint32(len(b))
	if err := binary.Write(carrier, binary.BigEndian, &length); err != nil {
		return err
	}

	_, err = carrier.Write(b)
	return err
}

func (p *binaryPropagator) Join(
	operationName string,
	opaqueCarrier interface{},
) (opentracing.Span, error) {
	carrier, ok := opaqueCarrier.(io.Reader)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}

	// Read the length of marshalled binary. io.ReadAll isn't that performant
	// since it keeps resizing the underlying buffer as it encounters more bytes
	// to read. By reading the length, we can allocate a fixed sized buf and read
	// the exact amount of bytes into it.
	var length uint32
	if err := binary.Read(carrier, binary.BigEndian, &length); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	buf := make([]byte, length)
	if n, err := carrier.Read(buf); err != nil {
		if n > 0 {
			return nil, opentracing.ErrTraceCorrupted
		}
		return nil, opentracing.ErrTraceNotFound
	}

	ctx := wire.TracerState{}
	if err := proto.Unmarshal(buf, &ctx); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	sp := p.tracer.getSpan()
	sp.raw = RawSpan{
		Context: Context{
			TraceID:      ctx.TraceId,
			SpanID:       randomID(),
			ParentSpanID: ctx.SpanId,
			Sampled:      ctx.Sampled,
		},
	}

	sp.raw.Baggage = ctx.BaggageItems

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}
