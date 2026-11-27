package adminapi_test

import (
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

const trackPath = "/admin/v1/bills/us-119-hr-3062/"

func requireTrackedByAdmin(t *testing.T, e *env, number int) {
	t.Helper()
	cat, err := e.store.PublicReadPorts().PublicDocs.ListCatalog(t.Context(), data.Page{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.Items {
		if c.Bill.Number == number && c.Tracked && slices.Contains(c.Reasons, selection.ReasonAdministrator) {
			return
		}
	}
	t.Fatalf("catalog has no entry tracked with the administrator reason: %+v", cat.Items)
}

func requireTrackedLog(t *testing.T, e *env) {
	t.Helper()
	for _, l := range e.log() {
		if l.Kind != adminapi.KindTracked {
			continue
		}
		if l.Reason != "pilot seed" || l.Actor == nil || l.Actor.ID != "op-1" || l.Actor.Name != "Operator One" {
			t.Fatalf("log entry wrong: %+v", l)
		}
		return
	}
	t.Fatal("no bill-tracked log entry")
}

func TestTrackWithReasonSelectsAndLogsWithoutPull(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	code, body := e.call(principalOp1, http.MethodPost, trackPath+"track", reason("pilot seed"))
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	requireTrackedByAdmin(t, e, 3062)
	ref := data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 3062}
	if !strings.Contains(string(body), "reingest") {
		t.Fatalf("response does not name the pull command: %s", body)
	}
	vs, err := e.store.PublicReadPorts().BillText.ListVersions(t.Context(), ref)
	if err != nil || len(vs) != 0 {
		t.Fatalf("track pulled text: %v %v", vs, err)
	}
	requireTrackedLog(t, e)
	if code, body = e.call(principalOp1, http.MethodPost, trackPath+"reingest", reason("pull text")); code != http.StatusOK {
		t.Fatalf("reingest %d: %s", code, body)
	}
	if vs, err = e.store.PublicReadPorts().BillText.ListVersions(t.Context(), ref); err != nil || len(vs) == 0 {
		t.Fatalf("text was not ingested by reingest: %v %v", vs, err)
	}
	// Tracking again changes nothing and logs nothing more.
	before := len(e.log())
	if code, body = e.call(principalOp1, http.MethodPost, trackPath+"track", reason("again")); code != http.StatusOK {
		t.Fatalf("second track %d: %s", code, body)
	}
	if len(e.log()) != before {
		t.Fatal("a repeated track wrote a log entry")
	}
}

func TestReasonRequired(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	for _, b := range []any{nil, reason(""), reason("   ")} {
		if code, _ := e.call(principalOp1, http.MethodPost, trackPath+"track", b); code != http.StatusUnprocessableEntity {
			t.Fatalf("body %v: status %d, want 422", b, code)
		}
	}
	if n := len(e.log()); n != 1 { // only the policy entry
		t.Fatalf("log has %d entries", n)
	}
}

func TestTrackRefusals(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	war := data.CatalogEntry{
		Bill:    data.BillRef{Jurisdiction: "us", Session: "119", Type: "hjres", Number: 999},
		Title:   "Directing the President to remove the Armed Forces pursuant to section 5(c) of the War Powers Resolution",
		Reasons: []string{}, Meta: data.Meta{Version: 1, AsOf: testNowFn(), Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform},
	}
	if err := e.store.PlatformPorts().PlatformDocs.PutCatalogEntry(t.Context(), war); err != nil {
		t.Fatal(err)
	}
	code, body := e.call(principalOp1, http.MethodPost, "/admin/v1/bills/us-119-hjres-999/track", reason("try"))
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("status %d: %s", code, body)
	}
	mustContain(t, body, "war powers resolution")
	if len(e.log()) != 1 {
		t.Fatal("a refused track wrote a log entry")
	}
}

func TestTrackNeedsSignoffInStewardPhase(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	p, err := selection.V1(testNowFn())
	if err != nil {
		t.Fatal(err)
	}
	p.Phase = data.PolicyPhaseStewardSignoff
	ports := e.store.PlatformPorts()
	if err := selection.Publish(t.Context(), struct {
		data.PublicDocs
		data.PlatformDocs
	}{ports.PublicDocs, ports.PlatformDocs}, p, "phase", testNowFn()); err != nil {
		t.Fatal(err)
	}
	code, body := e.call(principalOp1, http.MethodPost, trackPath+"track", reason("seed"))
	if code != http.StatusConflict {
		t.Fatalf("status %d: %s", code, body)
	}
	mustContain(t, body, "needs sign-off")
}

func TestInjectedLogFailureLeavesNoChange(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	// Pull the catalog entry first so only the tracked state is at stake.
	if code, body := e.call(principalOp1, http.MethodPatch, trackPath+"catalog", map[string]any{"reason": "x", "title": "T"}); code != http.StatusOK {
		t.Fatalf("patch %d: %s", code, body)
	}
	before := len(e.log())
	e.docs.fail = true
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{http.MethodPost, trackPath + "track", reason("seed")},
		{http.MethodPatch, trackPath + "catalog", map[string]any{"reason": "y", "title": "Other"}},
	} {
		if code, _ := e.call(principalOp1, c.method, c.path, c.body); code == http.StatusOK {
			t.Fatalf("%s %s succeeded with a failing log", c.method, c.path)
		}
	}
	e.docs.fail = false
	cat, err := e.store.PublicReadPorts().PublicDocs.ListCatalog(t.Context(), data.Page{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cat.Items {
		if c.Tracked || c.Title != "T" {
			t.Fatalf("a failed action changed the catalog: %+v", c)
		}
	}
	if len(e.log()) != before {
		t.Fatal("log changed")
	}
	// A failing log blocks a reingest before any pull.
	*e.modF = true
	if code, _ := e.call(principalOp1, http.MethodPost, trackPath+"reingest", reason("z")); code == http.StatusOK {
		t.Fatal("reingest ran with a failing log")
	}
}

func TestUntrackAndPatch(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	e.call(principalOp1, http.MethodPost, trackPath+"track", reason("seed"))
	if code, body := e.call(principalOp2, http.MethodPost, trackPath+"untrack", reason("selected in error")); code != http.StatusOK {
		t.Fatalf("untrack %d: %s", code, body)
	}
	if code, _ := e.call(principalOp1, http.MethodPatch, trackPath+"catalog", map[string]any{"reason": "r"}); code != http.StatusUnprocessableEntity {
		t.Fatalf("empty patch status %d", code)
	}
	code, body := e.call(principalOp1, http.MethodPatch, trackPath+"catalog", map[string]any{"reason": "r", "tracked": true})
	if code != http.StatusBadRequest {
		t.Fatalf("patch of tracked: %d %s", code, body)
	}
}

func TestReingestAndReparse(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	e.call(principalOp1, http.MethodPost, trackPath+"track", reason("seed"))
	code, body := e.call(principalOp1, http.MethodPost, trackPath+"reingest", reason("verify hashes"))
	if code != http.StatusOK {
		t.Fatalf("reingest %d: %s", code, body)
	}
	mustContain(t, body, `"changed":true`)
	code, body = e.call(principalOp1, http.MethodPost, trackPath+"v/ih/reparse", reason("parser check"))
	if code != http.StatusOK {
		t.Fatalf("reparse %d: %s", code, body)
	}
	if code, _ := e.call(principalOp1, http.MethodPost, trackPath+"v/zz/reparse", reason("x")); code != http.StatusNotFound {
		t.Fatalf("reparse of a missing version: %d", code)
	}
}

func TestPublishPolicy(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	p, err := selection.V1(testNowFn())
	if err != nil {
		t.Fatal(err)
	}
	doc := map[string]any{"reason": "initial", "policy": p}
	code, body := e.call(principalOp1, http.MethodPost, "/admin/v1/selection-policy", doc)
	if code != http.StatusOK {
		t.Fatalf("publish %d: %s", code, body)
	}
	if code, _ = e.call(principalOp1, http.MethodPost, "/admin/v1/selection-policy", doc); code != http.StatusOK {
		t.Fatalf("republish %d", code)
	}
	p.Version = 3
	if code, _ = e.call(principalOp1, http.MethodPost, "/admin/v1/selection-policy", map[string]any{"reason": "skip", "policy": p}); code != http.StatusConflict {
		t.Fatalf("version skip status %d", code)
	}
	var named bool
	for _, l := range e.log() {
		named = named || (l.Kind == "selection-policy" && l.Actor != nil && l.Actor.ID == "op-1")
	}
	if !named {
		t.Fatal("policy log entry does not name the operator")
	}
}

func TestGenerationsUnavailable(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	body := map[string]any{"reason": "r", "task": "summary", "bill": "us-119-hr-3062", "version": "eh"}
	if code, _ := e.call(principalOp1, http.MethodPost, "/admin/v1/generations", body); code != http.StatusNotImplemented {
		t.Fatalf("status %d", code)
	}
}

func TestFailuresAndJobs(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	e.call(principalOp1, http.MethodPost, trackPath+"track", reason("seed"))
	for _, p := range []string{"/admin/v1/ingest/failures", "/admin/v1/jobs"} {
		code, body := e.call(principalOp1, http.MethodGet, p, nil)
		if code != http.StatusOK {
			t.Fatalf("%s %d: %s", p, code, body)
		}
		mustContain(t, body, "fixtures/119")
	}
}

func TestTrackAddsAdministratorReasonToPolicyTrackedBill(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.publishV1()
	code, body := e.call(principalOp1, http.MethodPost, "/admin/v1/bills/us-119-hr-4313/track", reason("also chosen"))
	if code != http.StatusOK {
		t.Fatalf("status %d: %s", code, body)
	}
	b, err := e.store.PublicReadPorts().PublicDocs.GetBill(t.Context(), data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 4313})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(b.TrackedReasons, selection.ReasonAdministrator) || !slices.Contains(b.TrackedReasons, "committee-action:markup") {
		t.Fatalf("bill reasons %v", b.TrackedReasons)
	}
}
