package claude_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/claude"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

// fakeAPI is a local stand-in for the Messages API. No test in this package
// reaches a real endpoint: the client's base URL is the test server and the
// key is a literal placeholder.
type fakeAPI struct {
	mu         sync.Mutex
	bodies     []map[string]any
	stop       string
	text       string
	batchState string
}

func (f *fakeAPI) message() string {
	return fmt.Sprintf(`{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",`+
		`"content":[{"type":"text","text":%q}],"stop_reason":%q,"stop_sequence":null,`+
		`"usage":{"input_tokens":10,"output_tokens":5,"cache_read_input_tokens":100,"cache_creation_input_tokens":0}}`,
		f.text, f.stop)
}

func (f *fakeAPI) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	var doc map[string]any
	_ = json.Unmarshal(body, &doc)
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages":
		f.bodies = append(f.bodies, doc)
		_, _ = io.WriteString(w, f.message())
	case r.Method == http.MethodPost && r.URL.Path == "/v1/messages/batches":
		f.bodies = append(f.bodies, doc)
		_, _ = io.WriteString(w, f.batch("in_progress"))
	case strings.HasSuffix(r.URL.Path, "/results"):
		w.Header().Set("Content-Type", "application/x-jsonl")
		_, _ = fmt.Fprintf(w, `{"custom_id":"a","result":{"type":"succeeded","message":%s}}`+"\n", f.message())
		_, _ = io.WriteString(w, `{"custom_id":"b","result":{"type":"expired"}}`+"\n")
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/v1/messages/batches/"):
		_, _ = io.WriteString(w, f.batch(f.batchState))
	default:
		http.NotFound(w, r)
	}
}

func (f *fakeAPI) batch(state string) string {
	return fmt.Sprintf(`{"id":"msgbatch_1","type":"message_batch","processing_status":%q,`+
		`"request_counts":{"processing":0,"succeeded":1,"errored":0,"canceled":0,"expired":0},`+
		`"created_at":"2026-01-01T00:00:00Z","expires_at":"2026-01-02T00:00:00Z","ended_at":null,`+
		`"cancel_initiated_at":null,"results_url":"http://example.test/results"}`, state)
}

func newGen(t *testing.T, f *fakeAPI) *claude.Gen {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	g, err := claude.New(claude.Config{APIKey: "test-key-not-real", BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func request() data.GenRequest {
	return data.GenRequest{
		ID: "a", Task: data.TaskBillSummary, Instructions: "be neutral", Source: "[s1-l1] text",
		Input: "<request>{}</request>", Schema: json.RawMessage(`{"type":"object"}`), Effort: "medium",
	}
}

func TestNewRequiresKey(t *testing.T) {
	t.Parallel()
	if _, err := claude.New(claude.Config{}); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("New without key: err = %v, want ErrInvalid", err)
	}
}

func TestGenerateRequestShapeAndResult(t *testing.T) {
	t.Parallel()
	f := &fakeAPI{stop: "end_turn", text: `{"headline":"h","statements":[]}`}
	res, err := newGen(t, f).Generate(t.Context(), request())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != "ok" || res.Text != f.text || res.TokensIn != 110 || res.CachedIn != 100 || res.TokensOut != 5 {
		t.Fatalf("result = %+v", res)
	}
	checkRequestBody(t, f.bodies[0])
}

func checkRequestBody(t *testing.T, b map[string]any) {
	t.Helper()
	if b["model"] != "claude-opus-5-5" {
		t.Errorf("model = %v", b["model"])
	}
	oc, _ := b["output_config"].(map[string]any)
	if oc["effort"] != "medium" {
		t.Errorf("effort = %v", oc["effort"])
	}
	if fm, _ := oc["format"].(map[string]any); fm["type"] != "json_schema" {
		t.Errorf("format = %v", oc["format"])
	}
	for _, banned := range []string{"temperature", "top_p", "top_k", "tools"} {
		if _, ok := b[banned]; ok {
			t.Errorf("request sets %s", banned)
		}
	}
	msgs, _ := b["messages"].([]any)
	content, _ := msgs[0].(map[string]any)["content"].([]any)
	first, _ := content[0].(map[string]any)
	if first["text"] != "[s1-l1] text" || first["cache_control"] == nil {
		t.Errorf("first block = %v, want the cached source", first)
	}
	if last, _ := content[len(content)-1].(map[string]any); last["cache_control"] != nil {
		t.Errorf("volatile block carries cache_control")
	}
}

func TestStopReasonsAreChecked(t *testing.T) {
	t.Parallel()
	for stop, want := range map[string]string{"refusal": "refusal", "max_tokens": "truncated", "pause_turn": "invalid"} {
		f := &fakeAPI{stop: stop, text: `{"headline":"h"}`}
		res, err := newGen(t, f).Generate(t.Context(), request())
		if err != nil {
			t.Fatal(err)
		}
		if res.Outcome != want || (want != "ok" && res.Text != "") {
			t.Errorf("stop %s: outcome %q text %q, want %q and no text", stop, res.Outcome, res.Text, want)
		}
	}
}

func TestBatchSubmitAndCollect(t *testing.T) {
	t.Parallel()
	f := &fakeAPI{stop: "end_turn", text: "{}", batchState: "in_progress"}
	g := newGen(t, f)
	id, err := g.Submit(t.Context(), []data.GenRequest{request()})
	if err != nil || id != "msgbatch_1" {
		t.Fatalf("Submit = %q, %v", id, err)
	}
	if reqs, _ := f.bodies[0]["requests"].([]any); len(reqs) != 1 {
		t.Fatalf("batch body = %v", f.bodies[0])
	}
	if _, done, err := g.Collect(t.Context(), id); err != nil || done {
		t.Fatalf("Collect while processing = done %v, err %v", done, err)
	}
	f.mu.Lock()
	f.batchState = "ended"
	f.mu.Unlock()
	res, done, err := g.Collect(t.Context(), id)
	if err != nil || !done || len(res) != 2 {
		t.Fatalf("Collect = %+v, %v, %v", res, done, err)
	}
	checkBatchResults(t, res)
}

func TestSubmitRejectsBadRequests(t *testing.T) {
	t.Parallel()
	g := newGen(t, &fakeAPI{})
	for name, reqs := range map[string][]data.GenRequest{
		"empty":     {},
		"no id":     {{Task: "t"}},
		"duplicate": {request(), request()},
	} {
		if _, err := g.Submit(t.Context(), reqs); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func checkBatchResults(t *testing.T, res []data.GenResult) {
	t.Helper()
	if res[0].ID != "a" || res[0].Outcome != "ok" || res[1].ID != "b" || res[1].Outcome != "error" {
		t.Fatalf("results = %+v", res)
	}
}
