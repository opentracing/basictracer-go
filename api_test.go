package basictracer

import (
	"testing"

	opentracing "github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/harness"
	"github.com/stretchr/testify/suite"
)

func TestAPICheck(t *testing.T) {
	apiSuite := harness.NewAPICheckSuite(func() (tracer opentracing.Tracer, closer func()) {
		tracer = NewWithOptions(Options{
			Recorder:     NewInMemoryRecorder(),
			ShouldSample: func(traceID uint64) bool { return true }, // always sample
		})
		return tracer, nil
	}, harness.APICheckCapabilities{
		CheckBaggageValues: true,
		CheckInject:        true,
		CheckExtract:       true,
	})
	suite.Run(t, apiSuite)
}
