package basictracer

import (
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

func TestInMemoryRecorderSpans(t *testing.T) {
	recorder := NewInMemoryRecorder()
	var apiRecorder SpanRecorder = recorder
	span := RawSpan{
		Context:   Context{},
		Operation: "test-span",
		Start:     time.Now(),
		Duration:  -1,
	}
	apiRecorder.RecordSpan(span)
	if len(recorder.GetSpans()) != 1 {
		t.Fatal("No spans recorded")
	}
	if !reflect.DeepEqual(recorder.GetSpans()[0], span) {
		t.Fatal("Span not recorded")
	}
}

type CountingRecorder int32

func (c *CountingRecorder) RecordSpan(r RawSpan) {
	atomic.AddInt32((*int32)(c), 1)
}
