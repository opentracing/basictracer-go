package basictracer

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInMemoryRecorderSpans(t *testing.T) {
	recorder := NewInMemoryRecorder()
	var apiRecorder SpanRecorder = recorder
	span := RawSpan{
		Context:   SpanContext{},
		Operation: "test-span",
		Start:     time.Now(),
		Duration:  -1,
	}
	apiRecorder.RecordSpan(span)
	assert.Equal(t, []RawSpan{span}, recorder.GetSpans())
	assert.Equal(t, []RawSpan{}, recorder.GetSampledSpans())
}

func TestMultiRecorder(t *testing.T) {
	r1 := NewInMemoryRecorder()
	r2 := NewInMemoryRecorder()
	mr := MultiRecorder(r1, r2)
	span := RawSpan{
		Context:   SpanContext{},
		Operation: "test-span",
		Start:     time.Now(),
		Duration:  -1,
	}
	mr.RecordSpan(span)
	assert.Equal(t, []RawSpan{span}, r1.GetSpans())
	assert.Equal(t, []RawSpan{span}, r2.GetSpans())
}

type CountingRecorder int32

func (c *CountingRecorder) RecordSpan(r RawSpan) {
	atomic.AddInt32((*int32)(c), 1)
}
