package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Instruments are the standard instruments every service gets.
type Instruments struct {
	httpDuration metric.Float64Histogram
	httpActive   metric.Int64UpDownCounter
	dataDuration metric.Float64Histogram
	startup      metric.Float64Histogram
	jobRuns      metric.Int64Counter
	jobDuration  metric.Float64Histogram
	jobItems     metric.Int64Counter
	jobLastRun   metric.Float64Gauge
	genTokens    metric.Int64Histogram
	genResults   metric.Int64Counter
}

func newInstruments(m metric.Meter) (*Instruments, error) {
	var (
		ins = &Instruments{}
		err error
	)
	fail := func(name string, e error) (*Instruments, error) {
		return nil, fmt.Errorf("telemetry: instrument %s: %w", name, e)
	}
	if ins.httpDuration, err = m.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"), metric.WithDescription("Duration of HTTP server requests.")); err != nil {
		return fail("http.server.request.duration", err)
	}
	if ins.httpActive, err = m.Int64UpDownCounter("http.server.active_requests",
		metric.WithUnit("{request}")); err != nil {
		return fail("http.server.active_requests", err)
	}
	if ins.dataDuration, err = m.Float64Histogram("pubcommittee.data.operation.duration",
		metric.WithUnit("s"), metric.WithDescription("Duration of port operations.")); err != nil {
		return fail("pubcommittee.data.operation.duration", err)
	}
	if ins.startup, err = m.Float64Histogram("pubcommittee.startup.duration",
		metric.WithUnit("s"), metric.WithDescription("Time from process start to ready.")); err != nil {
		return fail("pubcommittee.startup.duration", err)
	}
	if ins.jobRuns, err = m.Int64Counter("pubcommittee.job.runs"); err != nil {
		return fail("pubcommittee.job.runs", err)
	}
	if ins.jobDuration, err = m.Float64Histogram("pubcommittee.job.duration", metric.WithUnit("s")); err != nil {
		return fail("pubcommittee.job.duration", err)
	}
	if ins.jobItems, err = m.Int64Counter("pubcommittee.job.items"); err != nil {
		return fail("pubcommittee.job.items", err)
	}
	if ins.jobLastRun, err = m.Float64Gauge("pubcommittee.job.last_run_timestamp",
		metric.WithUnit("s"), metric.WithDescription("Unix seconds of the last run.")); err != nil {
		return fail("pubcommittee.job.last_run_timestamp", err)
	}
	if err = ins.newGenInstruments(m); err != nil {
		return nil, err
	}
	return ins, nil
}

// RecordStartup records the cold-start cost.
func (i *Instruments) RecordStartup(ctx context.Context, d time.Duration) {
	i.startup.Record(ctx, d.Seconds())
}

// RecordData records one port operation: port and operation names, adapter
// package name, and the outcome from data.Outcome(err).
func (i *Instruments) RecordData(ctx context.Context, port, operation, adapter, outcome string, d time.Duration) {
	i.dataDuration.Record(ctx, d.Seconds(), metric.WithAttributes(
		attribute.String("pubcommittee.port", port),
		attribute.String("pubcommittee.operation", operation),
		attribute.String("pubcommittee.adapter", adapter),
		attribute.String("pubcommittee.outcome", outcome),
	))
}

// RecordJob records one job run and sets the last-run timestamp, so one push
// per run is enough to detect a failed or missing run.
func (i *Instruments) RecordJob(ctx context.Context, name string, start time.Time, items int, err error) {
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}
	attrs := metric.WithAttributes(
		attribute.String("pubcommittee.job.name", name),
		attribute.String("pubcommittee.outcome", outcome),
	)
	i.jobRuns.Add(ctx, 1, attrs)
	i.jobDuration.Record(ctx, time.Since(start).Seconds(), attrs)
	i.jobItems.Add(ctx, int64(items), attrs)
	i.jobLastRun.Record(ctx, float64(time.Now().Unix()), attrs)
}

// HTTP wraps next with the standard request instruments. The route is the
// matched mux pattern (never the concrete path), set by the mux on the
// request before the inner handler returns.
func (i *Instruments) HTTP(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method := metric.WithAttributes(attribute.String("http.request.method", r.Method))
		i.httpActive.Add(r.Context(), 1, method)
		defer i.httpActive.Add(r.Context(), -1, method)

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		start := time.Now()
		next.ServeHTTP(rec, r)

		route := r.Pattern
		if route == "" {
			route = "unmatched"
		}
		i.httpDuration.Record(r.Context(), time.Since(start).Seconds(), metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.String("http.route", route),
			attribute.String("http.response.status_code", strconv.Itoa(rec.status)),
		))
	})
}

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// newGenInstruments creates the model-call instruments.
func (i *Instruments) newGenInstruments(m metric.Meter) (err error) {
	if i.genTokens, err = m.Int64Histogram("gen_ai.client.token.usage",
		metric.WithUnit("{token}"), metric.WithDescription("Input and output tokens per model call.")); err != nil {
		return fmt.Errorf("telemetry: instrument gen_ai.client.token.usage: %w", err)
	}
	if i.genResults, err = m.Int64Counter("pubcommittee.generation.results"); err != nil {
		return fmt.Errorf("telemetry: instrument pubcommittee.generation.results: %w", err)
	}
	return nil
}
