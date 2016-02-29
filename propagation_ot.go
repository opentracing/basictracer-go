package basictracer

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	opentracing "github.com/opentracing/opentracing-go"
)

type splitTextPropagator struct {
	tracer *tracerImpl
}
type splitBinaryPropagator struct {
	tracer *tracerImpl
}
type goHTTPPropagator struct {
	*splitBinaryPropagator
}

const (
	fieldNameTraceID = "traceid"
	fieldNameSpanID  = "spanid"
	fieldNameSampled = "sampled"
)

func (p *splitTextPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc, ok := sp.(*spanImpl)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	splitTextCarrier, ok := carrier.(*opentracing.SplitTextCarrier)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	if splitTextCarrier.TracerState == nil {
		splitTextCarrier.TracerState = make(map[string]string, 3 /* see below */)
	}
	splitTextCarrier.TracerState[fieldNameTraceID] = strconv.FormatInt(sc.raw.TraceID, 10)
	splitTextCarrier.TracerState[fieldNameSpanID] = strconv.FormatInt(sc.raw.SpanID, 10)
	splitTextCarrier.TracerState[fieldNameSampled] = strconv.FormatBool(sc.raw.Sampled)

	sc.Lock()
	if l := len(sc.raw.Baggage); l > 0 && splitTextCarrier.Baggage == nil {
		splitTextCarrier.Baggage = make(map[string]string, l)
	}
	for k, v := range sc.raw.Baggage {
		splitTextCarrier.Baggage[k] = v
	}
	sc.Unlock()
	return nil
}

func (p *splitTextPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	splitTextCarrier, ok := carrier.(*opentracing.SplitTextCarrier)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	requiredFieldCount := 0
	var traceID, propagatedSpanID int64
	var sampled bool
	var err error
	for k, v := range splitTextCarrier.TracerState {
		switch strings.ToLower(k) {
		case fieldNameTraceID:
			traceID, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
		case fieldNameSpanID:
			propagatedSpanID, err = strconv.ParseInt(v, 10, 64)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
		case fieldNameSampled:
			sampled, err = strconv.ParseBool(v)
			if err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
		default:
			continue
		}
		requiredFieldCount++
	}
	const expFieldCount = 3
	if requiredFieldCount < expFieldCount {
		if len(splitTextCarrier.TracerState) == 0 {
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
	}
	sp.raw.Baggage = splitTextCarrier.Baggage

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}

func (p *splitBinaryPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	sc, ok := sp.(*spanImpl)
	if !ok {
		return opentracing.ErrInvalidSpan
	}
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return opentracing.ErrInvalidCarrier
	}
	var err error
	var sampledByte byte
	if sc.raw.Sampled {
		sampledByte = 1
	}

	// Handle the trace and span ids, and sampled status.
	contextBuf := bytes.NewBuffer(splitBinaryCarrier.TracerState[:0])
	err = binary.Write(contextBuf, binary.BigEndian, sc.raw.TraceID)
	if err != nil {
		return err
	}

	err = binary.Write(contextBuf, binary.BigEndian, sc.raw.SpanID)
	if err != nil {
		return err
	}

	err = binary.Write(contextBuf, binary.BigEndian, sampledByte)
	if err != nil {
		return err
	}

	// Handle the baggage.
	baggageBuf := bytes.NewBuffer(splitBinaryCarrier.Baggage[:0])
	err = binary.Write(baggageBuf, binary.BigEndian, int32(len(sc.raw.Baggage)))
	if err != nil {
		return err
	}
	for k, v := range sc.raw.Baggage {
		if err = binary.Write(baggageBuf, binary.BigEndian, int32(len(k))); err != nil {
			return err
		}
		baggageBuf.WriteString(k)
		if err = binary.Write(baggageBuf, binary.BigEndian, int32(len(v))); err != nil {
			return err
		}
		baggageBuf.WriteString(v)
	}

	splitBinaryCarrier.TracerState = contextBuf.Bytes()
	splitBinaryCarrier.Baggage = baggageBuf.Bytes()
	return nil
}

func (p *splitBinaryPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	splitBinaryCarrier, ok := carrier.(*opentracing.SplitBinaryCarrier)
	if !ok {
		return nil, opentracing.ErrInvalidCarrier
	}
	if len(splitBinaryCarrier.TracerState) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	// Handle the trace, span ids, and sampled status.
	contextReader := bytes.NewReader(splitBinaryCarrier.TracerState)
	var traceID, propagatedSpanID int64
	var sampledByte byte

	if err := binary.Read(contextReader, binary.BigEndian, &traceID); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	if err := binary.Read(contextReader, binary.BigEndian, &propagatedSpanID); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	if err := binary.Read(contextReader, binary.BigEndian, &sampledByte); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	// Handle the baggage.
	baggageReader := bytes.NewReader(splitBinaryCarrier.Baggage)
	var numBaggage int32
	if err := binary.Read(baggageReader, binary.BigEndian, &numBaggage); err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	iNumBaggage := int(numBaggage)
	var baggageMap map[string]string
	if iNumBaggage > 0 {
		var buf bytes.Buffer // TODO(tschottdorf): candidate for sync.Pool
		baggageMap = make(map[string]string, iNumBaggage)
		var keyLen, valLen int32
		for i := 0; i < iNumBaggage; i++ {
			if err := binary.Read(baggageReader, binary.BigEndian, &keyLen); err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
			buf.Grow(int(keyLen))
			if n, err := io.CopyN(&buf, baggageReader, int64(keyLen)); err != nil || int32(n) != keyLen {
				return nil, opentracing.ErrTraceCorrupted
			}
			key := buf.String()
			buf.Reset()

			if err := binary.Read(baggageReader, binary.BigEndian, &valLen); err != nil {
				return nil, opentracing.ErrTraceCorrupted
			}
			if n, err := io.CopyN(&buf, baggageReader, int64(valLen)); err != nil || int32(n) != valLen {
				return nil, opentracing.ErrTraceCorrupted
			}
			baggageMap[key] = buf.String()
			buf.Reset()
		}
	}

	sp := p.tracer.getSpan()
	sp.raw = RawSpan{
		Context: Context{
			TraceID:      traceID,
			SpanID:       randomID(),
			ParentSpanID: propagatedSpanID,
			Sampled:      sampledByte != 0,
		},
	}
	sp.raw.Baggage = baggageMap

	return p.tracer.startSpanInternal(
		sp,
		operationName,
		time.Now(),
		nil,
	), nil
}

const (
	tracerStateHeaderName  = "Tracer-State"
	traceBaggageHeaderName = "Trace-Baggage"
)

func (p *goHTTPPropagator) InjectSpan(
	sp opentracing.Span,
	carrier interface{},
) error {
	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := opentracing.NewSplitBinaryCarrier()
	if err := p.splitBinaryPropagator.InjectSpan(sp, splitBinaryCarrier); err != nil {
		return err
	}

	// Encode into the HTTP header as two base64 strings.
	header := carrier.(http.Header)
	header.Add(tracerStateHeaderName, base64.StdEncoding.EncodeToString(
		splitBinaryCarrier.TracerState))
	header.Add(traceBaggageHeaderName, base64.StdEncoding.EncodeToString(
		splitBinaryCarrier.Baggage))

	return nil
}

func (p *goHTTPPropagator) JoinTrace(
	operationName string,
	carrier interface{},
) (opentracing.Span, error) {
	// Decode the two base64-encoded data blobs from the HTTP header.
	header := carrier.(http.Header)
	tracerStateBase64, found := header[http.CanonicalHeaderKey(tracerStateHeaderName)]
	if !found || len(tracerStateBase64) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	traceBaggageBase64, found := header[http.CanonicalHeaderKey(traceBaggageHeaderName)]
	if !found || len(traceBaggageBase64) == 0 {
		return nil, opentracing.ErrTraceNotFound
	}
	tracerStateBinary, err := base64.StdEncoding.DecodeString(tracerStateBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}
	traceBaggageBinary, err := base64.StdEncoding.DecodeString(traceBaggageBase64[0])
	if err != nil {
		return nil, opentracing.ErrTraceCorrupted
	}

	// Defer to SplitBinary for the real work.
	splitBinaryCarrier := &opentracing.SplitBinaryCarrier{
		TracerState: tracerStateBinary,
		Baggage:     traceBaggageBinary,
	}
	return p.splitBinaryPropagator.JoinTrace(operationName, splitBinaryCarrier)
}
