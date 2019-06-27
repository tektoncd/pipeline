package tracing

import (
	"errors"
	"sync"

	"github.com/knative/pkg/tracing/config"
	zipkinmodel "github.com/openzipkin/zipkin-go/model"
	zipkinreporter "github.com/openzipkin/zipkin-go/reporter"
	"go.opencensus.io/exporter/zipkin"
	"go.opencensus.io/trace"
)

// ConfigOption is the interface for adding additional exporters and configuring opencensus tracing.
type ConfigOption func(*config.Config)

// OpenCensusTracer is responsible for managing and updating configuration of OpenCensus tracing
type OpenCensusTracer struct {
	curCfg         *config.Config
	configOptions  []ConfigOption
	zipkinReporter zipkinreporter.Reporter
	zipkinExporter trace.Exporter
}

// OpenCensus tracing keeps state in globals and therefore we can only run one OpenCensusTracer
var (
	octMutex  sync.Mutex
	globalOct *OpenCensusTracer
)

func NewOpenCensusTracer(configOptions ...ConfigOption) *OpenCensusTracer {
	return &OpenCensusTracer{
		configOptions: configOptions,
	}
}

func (oct *OpenCensusTracer) ApplyConfig(cfg *config.Config) error {
	err := oct.acquireGlobal()
	defer octMutex.Unlock()
	if err != nil {
		return err
	}

	// Short circuit if our config hasnt changed
	if oct.curCfg != nil && oct.curCfg.Equals(cfg) {
		return nil
	}

	// Apply config options
	for _, configOpt := range oct.configOptions {
		configOpt(cfg)
	}

	// Set config
	trace.ApplyConfig(*createOCTConfig(cfg))

	return nil
}

func (oct *OpenCensusTracer) Finish() error {
	err := oct.acquireGlobal()
	defer octMutex.Unlock()
	if err != nil {
		return errors.New("Finish called on OpenTracer which is not the global OpenCensusTracer.")
	}

	for _, configOpt := range oct.configOptions {
		configOpt(nil)
	}
	globalOct = nil

	return nil
}

func (oct *OpenCensusTracer) acquireGlobal() error {
	octMutex.Lock()

	if globalOct == nil {
		globalOct = oct
	} else if globalOct != oct {
		return errors.New("A OpenCensusTracer already exists and only one can be run at a time.")
	}

	return nil
}

func createOCTConfig(cfg *config.Config) *trace.Config {
	octCfg := trace.Config{}

	if cfg.Enable {
		if cfg.Debug {
			octCfg.DefaultSampler = trace.AlwaysSample()
		} else {
			octCfg.DefaultSampler = trace.ProbabilitySampler(cfg.SampleRate)
		}
	} else {
		octCfg.DefaultSampler = trace.NeverSample()
	}

	return &octCfg
}

func WithZipkinExporter(reporterFact ZipkinReporterFactory, endpoint *zipkinmodel.Endpoint) ConfigOption {
	return func(cfg *config.Config) {
		var (
			reporter zipkinreporter.Reporter
			exporter trace.Exporter
		)

		if cfg != nil && cfg.Enable {
			// Initialize our reporter / exporter
			// do this before cleanup to minimize time where we have duplicate exporters
			reporter, err := reporterFact(cfg)
			if err != nil {
				// TODO(greghaynes) log this error
				return
			}
			exporter := zipkin.NewExporter(reporter, endpoint)
			trace.RegisterExporter(exporter)
		}

		// We know this is set because we are called with acquireGlobal lock held
		oct := globalOct
		if oct.zipkinExporter != nil {
			trace.UnregisterExporter(oct.zipkinExporter)
		}

		if oct.zipkinReporter != nil {
			// TODO(greghaynes) log this error
			_ = oct.zipkinReporter.Close()
		}

		oct.zipkinReporter = reporter
		oct.zipkinExporter = exporter
	}
}
