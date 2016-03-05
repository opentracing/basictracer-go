package basictracer

import (
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
)

const op = "test"

func TestDebugAssertSingleGoroutine(t *testing.T) {
	opts := DefaultOptions()
	opts.Recorder = NewInMemoryRecorder()
	opts.DebugAssertSingleGoroutine = true
	tracer := NewWithOptions(opts)
	sp := tracer.StartSpan(op)
	sp.LogEvent("something on my goroutine")
	wait := make(chan struct{})
	var panicked bool
	go func() {
		defer func() {
			if r := recover(); r != nil {
				_, panicked = r.(*errAssertionFailed)
			}
			close(wait)
		}()
		sp.LogEvent("something on your goroutine")
	}()
	<-wait
	if !panicked {
		t.Fatal("expected a panic")
	}
}

func TestDebugAssertUseAfterFinish(t *testing.T) {
	opts := DefaultOptions()
	opts.Recorder = NewInMemoryRecorder()
	opts.DebugAssertUseAfterFinish = true
	tracer := NewWithOptions(opts)
	for _, double := range []bool{false, true} {
		sp := tracer.StartSpan(op)
		if double {
			sp.Finish()
		}
		var panicked bool
		func() {
			defer func() { panicked = recover() != nil }()
			sp.Finish()
		}()
		if panicked != double {
			t.Errorf("finished double = %t, but panicked = %t", double, panicked)
		}
	}
}

func TestConcurrentUsage(t *testing.T) {
	opts := DefaultOptions()
	var cr CountingRecorder
	opts.Recorder = &cr
	opts.DebugAssertSingleGoroutine = true
	tracer := NewWithOptions(opts)
	for i := 0; i < 100; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				sp := tracer.StartSpan(op)
				sp.LogEvent("test event")
				sp.SetTag("foo", "bar")
				sp.SetBaggageItem("boo", "far")
				sp.SetOperationName("x")
				csp := tracer.StartSpanWithOptions(opentracing.StartSpanOptions{
					Parent: sp,
				})
				csp.Finish()
				defer sp.Finish()
			}
		}()
	}
}
