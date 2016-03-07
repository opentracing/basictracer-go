package basictracer

import (
	"fmt"
	"net/http"
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
)

var tags []string

func init() {
	tags = make([]string, 1000)
	for j := 0; j < len(tags); j++ {
		tags[j] = fmt.Sprintf("%d", randomID())
	}
}

func executeOps(sp opentracing.Span, numEvent, numTag, numItems int) {
	for j := 0; j < numEvent; j++ {
		sp.LogEvent("event")
	}
	for j := 0; j < numTag; j++ {
		sp.SetTag(tags[j], nil)
	}
	for j := 0; j < numItems; j++ {
		sp.SetBaggageItem(tags[j], tags[j])
	}
}

func benchmarkWithOps(b *testing.B, numEvent, numTag, numItems int) {
	var r CountingRecorder
	t := New(&r)
	benchmarkWithOpsAndCB(b, func() opentracing.Span {
		return t.StartSpan("test")
	}, numEvent, numTag, numItems)
	if int(r) != b.N {
		b.Fatalf("missing traces: expected %d, got %d", b.N, r)
	}
}

func benchmarkWithOpsAndCB(b *testing.B, create func() opentracing.Span,
	numEvent, numTag, numItems int) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp := create()
		executeOps(sp, numEvent, numTag, numItems)
		sp.Finish()
	}
	b.StopTimer()
}

func BenchmarkSpan_Empty(b *testing.B) {
	benchmarkWithOps(b, 0, 0, 0)
}

func BenchmarkSpan_100Events(b *testing.B) {
	benchmarkWithOps(b, 100, 0, 0)
}

func BenchmarkSpan_1000Events(b *testing.B) {
	benchmarkWithOps(b, 100, 0, 0)
}

func BenchmarkSpan_100Tags(b *testing.B) {
	benchmarkWithOps(b, 0, 100, 0)
}

func BenchmarkSpan_1000Tags(b *testing.B) {
	benchmarkWithOps(b, 0, 100, 0)
}

func BenchmarkSpan_100BaggageItems(b *testing.B) {
	benchmarkWithOps(b, 0, 0, 100)
}

func BenchmarkTrimmedSpan_100Events_100Tags_100BaggageItems(b *testing.B) {
	var r CountingRecorder
	opts := DefaultOptions()
	opts.TrimUnsampledSpans = true
	opts.ShouldSample = func(_ int64) bool { return false }
	opts.Recorder = &r
	t := NewWithOptions(opts)
	benchmarkWithOpsAndCB(b, func() opentracing.Span {
		sp := t.StartSpan("test")
		return sp
	}, 100, 100, 100)
	if int(r) != b.N {
		b.Fatalf("missing traces: expected %d, got %d", b.N, r)
	}
}

func benchmarkInject(b *testing.B, format opentracing.BuiltinFormat, numItems int) {
	var r CountingRecorder
	tracer := New(&r)
	sp := tracer.StartSpan("testing")
	executeOps(sp, 0, 0, numItems)
	var carrier interface{}
	switch format {
	case opentracing.SplitText:
		carrier = opentracing.NewSplitTextCarrier()
	case opentracing.SplitBinary:
		carrier = opentracing.NewSplitBinaryCarrier()
	case opentracing.GoHTTPHeader:
		carrier = http.Header{}
	default:
		b.Fatalf("unhandled format %d", format)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := tracer.Inject(sp, format, carrier)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func benchmarkJoin(b *testing.B, format opentracing.BuiltinFormat, numItems int) {
	var r CountingRecorder
	tracer := New(&r)
	sp := tracer.StartSpan("testing")
	executeOps(sp, 0, 0, numItems)
	var carrier interface{}
	switch format {
	case opentracing.SplitText:
		carrier = opentracing.NewSplitTextCarrier()
	case opentracing.SplitBinary:
		carrier = opentracing.NewSplitBinaryCarrier()
	case opentracing.GoHTTPHeader:
		carrier = http.Header{}
	default:
		b.Fatalf("unhandled format %d", format)
	}
	if err := tracer.Inject(sp, format, carrier); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sp, err := tracer.Join("benchmark", format, carrier)
		if err != nil {
			b.Fatal(err)
		}
		sp.Finish() // feed back into buffer pool
	}
}

func BenchmarkInject_SplitText_Empty(b *testing.B) {
	benchmarkInject(b, opentracing.SplitText, 0)
}

func BenchmarkInject_SplitText_100BaggageItems(b *testing.B) {
	benchmarkInject(b, opentracing.SplitText, 100)
}

func BenchmarkInject_GoHTTPHeader_Empty(b *testing.B) {
	benchmarkInject(b, opentracing.GoHTTPHeader, 0)
}

func BenchmarkInject_GoHTTPHeader_100BaggageItems(b *testing.B) {
	benchmarkInject(b, opentracing.GoHTTPHeader, 100)
}

func BenchmarkInject_SplitBinary_Empty(b *testing.B) {
	benchmarkInject(b, opentracing.SplitBinary, 0)
}

func BenchmarkInject_SplitBinary_100BaggageItems(b *testing.B) {
	benchmarkInject(b, opentracing.SplitBinary, 100)
}

func BenchmarkJoin_SplitText_Empty(b *testing.B) {
	benchmarkJoin(b, opentracing.SplitText, 0)
}

func BenchmarkJoin_SplitText_100BaggageItems(b *testing.B) {
	benchmarkJoin(b, opentracing.SplitText, 100)
}

func BenchmarkJoin_GoHTTPHeader_Empty(b *testing.B) {
	benchmarkJoin(b, opentracing.GoHTTPHeader, 0)
}

func BenchmarkJoin_GoHTTPHeader_100BaggageItems(b *testing.B) {
	benchmarkJoin(b, opentracing.GoHTTPHeader, 100)
}

func BenchmarkJoin_SplitBinary_Empty(b *testing.B) {
	benchmarkJoin(b, opentracing.SplitBinary, 0)
}

func BenchmarkJoin_SplitBinary_100BaggageItems(b *testing.B) {
	benchmarkJoin(b, opentracing.SplitBinary, 100)
}
