package basictracer

import (
	"sync/atomic"
	"testing"
	"time"
	"github.com/uber/ringpop-go/Godeps/_workspace/src/github.com/stretchr/testify/assert"
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
	assert.Equal(t, []RawSpan{span}, recorder.GetSpans())
	assert.Equal(t, []RawSpan{}, recorder.GetSampledSpans())
	//if len(recorder.GetSpans()) != 1 {
	//	t.Fatal("No spans recorded")
	//}
	//if !reflect.DeepEqual(recorder.GetSpans()[0], span) {
	//	t.Fatal("Span not recorded")
	//}
	//if len(recorder.GetSampledSpans()) != 0 {
	//	t.Fatal("Non-sampled span returned by GetSampledSpans")
	//}
}

type CountingRecorder int32

func (c *CountingRecorder) RecordSpan(r RawSpan) {
	atomic.AddInt32((*int32)(c), 1)
}
