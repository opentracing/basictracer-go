package basictracer_test

import (
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	basictracer "github.com/opentracing/basictracer-go"
	"github.com/opentracing/basictracer-go/testutils"
	opentracing "github.com/opentracing/opentracing-go"
)

type verbatimCarrier struct {
	basictracer.Context
	b map[string]string
}

var _ basictracer.DelegatingCarrier = &verbatimCarrier{}

func (vc *verbatimCarrier) SetBaggageItem(k, v string) {
	vc.b[k] = v
}

func (vc *verbatimCarrier) GetBaggage(f func(string, string)) {
	for k, v := range vc.b {
		f(k, v)
	}
}

func (vc *verbatimCarrier) SetState(tID, sID int64, sampled bool) {
	vc.Context = basictracer.Context{TraceID: tID, SpanID: sID, Sampled: sampled}
}

func (vc *verbatimCarrier) State() (traceID, spanID int64, sampled bool) {
	return vc.Context.TraceID, vc.Context.SpanID, vc.Context.Sampled
}

func TestSpanPropagator(t *testing.T) {
	const op = "test"
	recorder := testutils.NewInMemoryRecorder()
	tracer := basictracer.New(recorder)

	sp := tracer.StartSpan(op)
	sp.SetBaggageItem("foo", "bar")

	tests := []struct {
		typ, carrier interface{}
	}{
		{basictracer.Accessor, basictracer.DelegatingCarrier(&verbatimCarrier{b: map[string]string{}})},
		{opentracing.SplitBinary, opentracing.NewSplitBinaryCarrier()},
		{opentracing.SplitText, opentracing.NewSplitTextCarrier()},
		{opentracing.GoHTTPHeader, http.Header{}},
	}

	for i, test := range tests {
		inj := tracer.Injector(test.typ)
		if inj == nil {
			t.Fatalf("%d: no injector found for %T", i, test.carrier)
		}
		if err := inj.InjectSpan(sp, test.carrier); err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		child, err := tracer.Extractor(test.typ).JoinTrace(op, test.carrier)
		if err != nil {
			t.Fatalf("%d: %v", i, err)
		}
		child.Finish()
	}
	sp.Finish()

	spans := recorder.GetSpans()
	if a, e := len(spans), len(tests)+1; a != e {
		t.Fatalf("expected %d spans, got %d", e, a)
	}

	// The last span is the original one.
	exp, spans := spans[len(spans)-1], spans[:len(spans)-1]
	exp.Duration = time.Duration(123)
	exp.Start = time.Time{}.Add(1)

	for i, sp := range spans {
		if a, e := sp.ParentSpanID, exp.SpanID; a != e {
			t.Fatalf("%d: ParentSpanID %d does not match expectation %d", i, a, e)
		} else {
			// Prepare for comparison.
			sp.SpanID, sp.ParentSpanID = exp.SpanID, 0
			sp.Duration, sp.Start = exp.Duration, exp.Start
		}
		if a, e := sp.TraceID, exp.TraceID; a != e {
			t.Fatalf("%d: TraceID changed from %d to %d", i, e, a)
		}
		if !reflect.DeepEqual(exp, sp) {
			t.Fatalf("%d: wanted %+v, got %+v", i, spew.Sdump(exp), spew.Sdump(sp))
		}
	}
}
