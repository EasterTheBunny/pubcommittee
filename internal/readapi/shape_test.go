package readapi

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// shapeLine is one "client address shape" log line.
type shapeLine struct {
	Count   int               `json:"xff_count"`
	Tags    []string          `json:"xff_tags"`
	Others  map[string]string `json:"other_tags"`
	Socket  string            `json:"socket_tag"`
	Keyed   string            `json:"keyed_tag"`
	Present []string          `json:"present"`
}

// shapeLogs sends one request per X-Forwarded-For value through a debugging
// limiter and returns the raw log.
func shapeLogs(t *testing.T, xffs ...string) string {
	t.Helper()
	var buf bytes.Buffer
	lim, err := newLimiter(DefaultRateLimits(), "", func() time.Time { return time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	lim.debug, lim.log = true, slog.New(slog.NewJSONHandler(&buf, nil))
	h := lim.middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	for _, xff := range xffs {
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/v1/bills?scope=catalog", nil)
		r.RemoteAddr = "192.0.2.10:4000"
		r.Header.Set("X-Forwarded-For", xff)
		r.Header.Set("Via", "1.1 proxy.example")
		h.ServeHTTP(httptest.NewRecorder(), r)
	}
	return buf.String()
}

func decodeShapes(t *testing.T, logs string) []shapeLine {
	t.Helper()
	var out []shapeLine
	dec := json.NewDecoder(strings.NewReader(logs))
	for dec.More() {
		var l shapeLine
		if err := dec.Decode(&l); err != nil {
			t.Fatal(err)
		}
		out = append(out, l)
	}
	return out
}

func TestLogShapeNeverLogsValues(t *testing.T) {
	t.Parallel()
	logs := shapeLogs(t, "198.51.100.7, 203.0.113.9")
	for _, raw := range []string{"198.51.100", "203.0.113", "192.0.2", "proxy.example"} {
		if strings.Contains(logs, raw) {
			t.Errorf("log holds a raw value %q: %s", raw, logs)
		}
	}
}

func TestLogShapeTagsCompare(t *testing.T) {
	t.Parallel()
	lines := decodeShapes(t, shapeLogs(t, "198.51.100.7, 203.0.113.9", "198.51.100.8, 203.0.113.9"))
	if len(lines) != 2 || lines[0].Count != 2 || len(lines[0].Tags[0]) != 2*tagBytes {
		t.Fatalf("lines = %+v", lines)
	}
	// The first entry differs between the two clients; the last is the same proxy.
	if lines[0].Tags[0] == lines[1].Tags[0] || lines[0].Tags[1] != lines[1].Tags[1] {
		t.Errorf("tags do not compare: %v vs %v", lines[0].Tags, lines[1].Tags)
	}
	// No header configured: read-api keys on the socket.
	if lines[0].Keyed != lines[0].Socket || lines[0].Others["Via"] == "" {
		t.Errorf("line = %+v", lines[0])
	}
}
