// Package telemetry sets up OpenTelemetry metrics for every service and job.
// HTTP services expose Prometheus metrics for scraping; jobs push over OTLP
// before they exit. Metric views keep only allowlisted attribute keys, so a
// new attribute needs a change to Allowlist, reviewed like a schema change
// (architecture § Privacy rules for telemetry). Ids, paths, IPs, and signal
// values are never attributes.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

// Errors returned by Setup for invalid options.
var (
	// ErrServiceName means Options.ServiceName is empty.
	ErrServiceName = errors.New("telemetry: ServiceName is required")
	// ErrUnknownKind means Options.Kind is not a defined Kind.
	ErrUnknownKind = errors.New("telemetry: unknown kind")
)

// jobPushInterval is long on purpose: a job pushes once, at Shutdown.
const jobPushInterval = 24 * time.Hour

const meterName = "github.com/easterthebunny/pubcommittee"

// Allowlist is the complete set of metric attribute keys. Views drop every
// other key.
//
//nolint:gochecknoglobals // read-only configuration list; a function would only copy it.
var Allowlist = []attribute.Key{
	"http.request.method",
	"http.route",
	"http.response.status_code",
	"pubcommittee.port",
	"pubcommittee.operation",
	"pubcommittee.adapter",
	"pubcommittee.outcome",
	"pubcommittee.write.kind",
	"pubcommittee.provider",
	"pubcommittee.task.kind",
	"pubcommittee.job.name",
	"pubcommittee.generation.task",
	"gen_ai.operation.name",
	"gen_ai.request.model",
	"gen_ai.token.type",
}

// Kind selects how metrics leave the process.
type Kind int

// Kinds.
const (
	// KindService serves Prometheus metrics for scraping.
	KindService Kind = iota
	// KindJob pushes metrics over OTLP on Shutdown.
	KindJob
)

// Options configures Setup.
type Options struct {
	Kind        Kind
	ServiceName string
	Version     string // commit
	Environment string // dev, staging, prod
	// OTLPEndpoint is the OTLP/HTTP base URL for jobs, for example
	// http://otel-collector:4318. Empty means OTEL_EXPORTER_OTLP_ENDPOINT is
	// read by the exporter itself.
	OTLPEndpoint string
	// Reader overrides the exporter; tests pass a metric.ManualReader.
	Reader sdkmetric.Reader
}

// Telemetry owns the meter provider and the standard instruments.
type Telemetry struct {
	provider *sdkmetric.MeterProvider
	handler  http.Handler
	// Instruments are the standard instruments.
	Instruments *Instruments
}

// Setup validates opts without network calls, builds the provider with the
// attribute-allowlist views, and registers the standard instruments.
func Setup(opts Options) (*Telemetry, error) {
	if opts.ServiceName == "" {
		return nil, ErrServiceName
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		semconv.ServiceName(opts.ServiceName),
		semconv.ServiceVersion(valueOr(opts.Version, "unknown")),
		attribute.String("deployment.environment.name", valueOr(opts.Environment, "dev")),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}

	reader, handler, err := newReader(opts)
	if err != nil {
		return nil, err
	}

	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(res),
		sdkmetric.WithReader(reader),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{Name: "*"},
			sdkmetric.Stream{AttributeFilter: attribute.NewAllowKeysFilter(Allowlist...)},
		)),
	)
	if err := runtime.Start(runtime.WithMeterProvider(provider)); err != nil {
		return nil, fmt.Errorf("telemetry: runtime metrics: %w", err)
	}
	ins, err := newInstruments(provider.Meter(meterName))
	if err != nil {
		return nil, err
	}
	if handler == nil {
		handler = http.NotFoundHandler()
	}
	return &Telemetry{provider: provider, handler: handler, Instruments: ins}, nil
}

// Meter returns the project meter for service-specific instruments.
func (t *Telemetry) Meter() metric.Meter { return t.provider.Meter(meterName) }

// MetricsHandler serves Prometheus text for GET /metrics (services only).
func (t *Telemetry) MetricsHandler() http.Handler { return t.handler }

// Shutdown flushes and, for jobs, pushes metrics over OTLP.
func (t *Telemetry) Shutdown(ctx context.Context) error {
	if err := t.provider.ForceFlush(ctx); err != nil {
		return fmt.Errorf("telemetry: flush: %w", err)
	}
	if err := t.provider.Shutdown(ctx); err != nil {
		return fmt.Errorf("telemetry: shutdown: %w", err)
	}
	return nil
}

func valueOr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

// newReader builds the exporter for opts.Kind, or returns opts.Reader.
func newReader(opts Options) (sdkmetric.Reader, http.Handler, error) {
	if opts.Reader != nil {
		return opts.Reader, nil, nil
	}
	switch opts.Kind {
	case KindService:
		reg := prometheus.NewRegistry()
		exp, err := otelprom.New(otelprom.WithRegisterer(reg), otelprom.WithoutScopeInfo())
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: prometheus exporter: %w", err)
		}
		return exp, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}), nil
	case KindJob:
		var oo []otlpmetrichttp.Option
		if opts.OTLPEndpoint != "" {
			oo = append(oo, otlpmetrichttp.WithEndpointURL(opts.OTLPEndpoint+"/v1/metrics"))
		}
		// The exporter connects on first export, so setup makes no network call.
		exp, err := otlpmetrichttp.New(context.Background(), oo...)
		if err != nil {
			return nil, nil, fmt.Errorf("telemetry: otlp exporter: %w", err)
		}
		return sdkmetric.NewPeriodicReader(exp, sdkmetric.WithInterval(jobPushInterval)), nil, nil
	default:
		return nil, nil, fmt.Errorf("%w: %d", ErrUnknownKind, opts.Kind)
	}
}
