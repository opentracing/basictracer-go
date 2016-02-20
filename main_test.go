package basictracer_test

import (
	"sync"

	"github.com/opentracing/basictracer-go"
)

// InMemoryRecorder is a simple thread-safe implementation of
// SpanRecorder that stores all reported spans in memory, accessible
// via reporter.GetSpans()
type InMemoryRecorder struct {
	spans []basictracer.RawSpan
	lock  sync.Mutex
}

// NewInMemoryRecorder instantiates a new InMemoryRecorder for testing purposes.
func NewInMemoryRecorder() *InMemoryRecorder {
	return &InMemoryRecorder{
		spans: make([]basictracer.RawSpan, 0),
	}
}

// RecordSpan implements RecordSpan() of SpanRecorder.
//
// The recorded spans can be retrieved via recorder.Spans slice.
func (recorder *InMemoryRecorder) RecordSpan(span basictracer.RawSpan) {
	recorder.lock.Lock()
	defer recorder.lock.Unlock()
	recorder.spans = append(recorder.spans, span)
}

// GetSpans returns a snapshot of spans recorded so far.
func (recorder *InMemoryRecorder) GetSpans() []basictracer.RawSpan {
	recorder.lock.Lock()
	defer recorder.lock.Unlock()
	spans := make([]basictracer.RawSpan, len(recorder.spans))
	copy(spans, recorder.spans)
	return spans
}
