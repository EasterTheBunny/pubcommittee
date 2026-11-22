package telemetry_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/telemetry"
)

func collect(t *testing.T, r *sdkmetric.ManualReader) metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	if err := r.Collect(context.Background(), &rm); err != nil {
		t.Fatalf("collect: %v", err)
	}
	return rm
}

func find(rm metricdata.ResourceMetrics, name string) *metricdata.Metrics {
	for _, sm := range rm.ScopeMetrics {
		for i := range sm.Metrics {
			if sm.Metrics[i].Name == name {
				return &sm.Metrics[i]
			}
		}
	}
	return nil
}

func newTelemetry(t *testing.T) (*telemetry.Telemetry, *sdkmetric.ManualReader) {
	t.Helper()
	r := sdkmetric.NewManualReader()
	tel, err := telemetry.Setup(telemetry.Options{ServiceName: "test", Reader: r})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	return tel, r
}

func TestSetupRequiresServiceName(t *testing.T) {
	t.Parallel()
	if _, err := telemetry.Setup(telemetry.Options{}); err == nil {
		t.Fatal("want error for empty ServiceName")
	}
}

// TestAttributeAllowlist: a forbidden attribute key never leaves the process.
func TestAttributeAllowlist(t *testing.T) {
	t.Parallel()
	r := sdkmetric.NewManualReader()
	tel, err := telemetry.Setup(telemetry.Options{ServiceName: "test", Reader: r})
	if err != nil {
		t.Fatal(err)
	}
	// A counter created by a caller through the same provider path: use the
	// job counter's sibling by recording via a fresh meter on the global view.
	tel.Instruments.RecordData(context.Background(), "BillText", "GetVersion", "memory", "ok", time.Millisecond)
	m := find(collect(t, r), "pubcommittee.data.operation.duration")
	if m == nil {
		t.Fatal("metric missing")
	}
	hist, ok := m.Data.(metricdata.Histogram[float64])
	if !ok || len(hist.DataPoints) == 0 {
		t.Fatalf("unexpected data %T", m.Data)
	}
	allowed := map[attribute.Key]bool{}
	for _, k := range telemetry.Allowlist {
		allowed[k] = true
	}
	for _, dp := range hist.DataPoints {
		for _, kv := range dp.Attributes.ToSlice() {
			if !allowed[kv.Key] {
				t.Errorf("attribute %q not allowlisted", kv.Key)
			}
		}
	}
}

// TestAllowlistDropsUnknownKeys records through a meter with forbidden keys
// and asserts the views remove them.
func TestAllowlistDropsUnknownKeys(t *testing.T) {
	t.Parallel()
	r := sdkmetric.NewManualReader()
	tel, err := telemetry.Setup(telemetry.Options{ServiceName: "test", Reader: r})
	if err != nil {
		t.Fatal(err)
	}
	m := tel.Meter()
	c, err := m.Int64Counter("pubcommittee.test.counter")
	if err != nil {
		t.Fatal(err)
	}
	c.Add(context.Background(), 1, metric.WithAttributes(
		attribute.String("bill_id", "us/119/hr/1"),
		attribute.String("client.address", "203.0.113.9"),
		attribute.String("url.path", "/v1/bills/us/119/hr/1"),
		attribute.String("pubcommittee.outcome", "ok"),
	))
	got := find(collect(t, r), "pubcommittee.test.counter")
	if got == nil {
		t.Fatal("metric missing")
	}
	sum := got.Data.(metricdata.Sum[int64])
	attrs := sum.DataPoints[0].Attributes
	for _, k := range []attribute.Key{"bill_id", "client.address", "url.path"} {
		if attrs.HasValue(k) {
			t.Errorf("forbidden key %q survived the view", k)
		}
	}
	if !attrs.HasValue("pubcommittee.outcome") {
		t.Error("allowlisted key was dropped")
	}
}

func TestHTTPUsesRoutePattern(t *testing.T) {
	t.Parallel()
	tel, r := newTelemetry(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/bills/{bill}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTeapot) })
	srv := httptest.NewServer(tel.Instruments.HTTP(mux))
	defer srv.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+"/v1/bills/us-119-hr-1", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()

	m := find(collect(t, r), "http.server.request.duration")
	if m == nil {
		t.Fatal("metric missing")
	}
	dp := m.Data.(metricdata.Histogram[float64]).DataPoints[0]
	if v, _ := dp.Attributes.Value("http.route"); v.AsString() != "GET /v1/bills/{bill}" {
		t.Errorf("route = %q", v.AsString())
	}
	if v, _ := dp.Attributes.Value("http.response.status_code"); v.AsString() != "418" {
		t.Errorf("status = %q", v.AsString())
	}
}

func TestWrapRecordsOutcome(t *testing.T) {
	t.Parallel()
	tel, r := newTelemetry(t)
	store := memory.New()
	ports := tel.WrapPublicRead("memory", store.PublicReadPorts())
	_, err := ports.BillText.GetVersion(context.Background(), data.VersionRef{
		Bill: data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1}, Version: "ih",
	})
	if err == nil {
		t.Fatal("want not found")
	}
	m := find(collect(t, r), "pubcommittee.data.operation.duration")
	if m == nil {
		t.Fatal("metric missing")
	}
	dp := m.Data.(metricdata.Histogram[float64]).DataPoints[0]
	for k, want := range map[attribute.Key]string{
		"pubcommittee.port": "BillText", "pubcommittee.operation": "GetVersion",
		"pubcommittee.adapter": "memory", "pubcommittee.outcome": "not_found",
	} {
		if v, _ := dp.Attributes.Value(k); v.AsString() != want {
			t.Errorf("%s = %q, want %q", k, v.AsString(), want)
		}
	}
}

func TestJobLastRun(t *testing.T) {
	t.Parallel()
	tel, r := newTelemetry(t)
	tel.Instruments.RecordJob(context.Background(), "job-test", time.Now(), 3, nil)
	if find(collect(t, r), "pubcommittee.job.last_run_timestamp") == nil {
		t.Fatal("last_run_timestamp missing")
	}
}

func TestPrometheusHandler(t *testing.T) {
	t.Parallel()
	tel, err := telemetry.Setup(telemetry.Options{ServiceName: "test", Kind: telemetry.KindService})
	if err != nil {
		t.Fatal(err)
	}
	tel.Instruments.RecordStartup(context.Background(), time.Millisecond)
	rec := httptest.NewRecorder()
	tel.MetricsHandler().ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil))
	if rec.Code != http.StatusOK || rec.Body.Len() == 0 {
		t.Fatalf("code %d len %d", rec.Code, rec.Body.Len())
	}
}

func TestJobSetupNoNetwork(t *testing.T) {
	t.Parallel()
	tel, err := telemetry.Setup(telemetry.Options{ServiceName: "job-test", Kind: telemetry.KindJob, OTLPEndpoint: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("setup must not touch the network: %v", err)
	}
	_ = tel
}
