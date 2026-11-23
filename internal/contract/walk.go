package contract

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

const (
	billsPath   = "/v1/bills"
	scopeParam  = "scope"
	latestN     = 2 // the fixture bill has two versions
	catalogSize = 3 // fixture catalog entries
	trackedSize = 2 // fixture tracked bills
)

func catalogQuery() url.Values { return url.Values{scopeParam: {"catalog"}} }

// route is one request of the seeded walk and the status it must answer.
type route struct {
	path  string
	query url.Values
	want  int
}

// seededRoutes lists a request for every M1 route against a service seeded
// by internal/readapi/seed, including the error answers the spec declares.
func seededRoutes() []route {
	bill := "/v1/bills/" + seed.TrackedBill
	one := "/v1/bills/" + seed.TrackedOneBill
	catalog := catalogQuery()
	return []route{
		{"/v1", nil, http.StatusOK},
		{"/v1/openapi.yaml", nil, http.StatusOK},
		{billsPath, nil, http.StatusOK},
		{billsPath, url.Values{"limit": {"1"}}, http.StatusOK},
		{billsPath, catalog, http.StatusOK},
		{billsPath, url.Values{
			scopeParam: {"catalog"}, "chamber": {"house"}, "committee_activity": {"true"},
			"subject": {"Health"}, "status": {"reported"}, "q": {"clinic"},
		}, http.StatusOK},
		{billsPath, url.Values{scopeParam: {"nope"}}, http.StatusBadRequest},
		{billsPath, url.Values{"cursor": {"!!"}}, http.StatusBadRequest},
		{bill, nil, http.StatusOK},
		{"/v1/bills/us-119-hr-1", nil, http.StatusNotFound},
		{bill + "/v/1", nil, http.StatusOK},
		{bill + "/v/2", nil, http.StatusOK},
		{bill + "/v/latest", nil, http.StatusOK},
		{bill + "/v/3", nil, http.StatusNotFound},
		{bill + "/v/2/s/s4", nil, http.StatusOK},
		{bill + "/v/2/s/s4.b", nil, http.StatusOK},
		{bill + "/v/2/s/s9", nil, http.StatusNotFound},
		{bill + "/v/2/s/s4/l/3", nil, http.StatusOK},
		{bill + "/v/2/s/s4/l/1-3", nil, http.StatusOK},
		{bill + "/v/2/s/s4/l/3-1", nil, http.StatusBadRequest},
		{bill + "/v/2/s/s4/l/99", nil, http.StatusNotFound},
		{bill + "/s/s4", nil, http.StatusOK},
		{bill + "/s/s4/l/2", nil, http.StatusOK},
		{bill + "/s/s4/l/x", nil, http.StatusBadRequest},
		{one + "/v/1", nil, http.StatusOK},
		{bill + "/v/2/explanations", nil, http.StatusOK},
		{bill + "/v/1/explanations", nil, http.StatusOK},
		{bill + "/v/2/summary", nil, http.StatusNotFound},
		{bill + "/v/2/changes", nil, http.StatusNotFound},
		{bill + "/v/2/citations", nil, http.StatusNotFound},
		{bill + "/v/2/citations/unresolved", nil, http.StatusNotFound},
		{bill + "/v/1/compare/2", nil, http.StatusNotFound},
		{"/v1/selection-policy", nil, http.StatusOK},
		{"/v1/selection-policy/1", nil, http.StatusOK},
		{"/v1/selection-policy/9", nil, http.StatusNotFound},
		{"/v1/moderation-log", nil, http.StatusOK},
		{"/v1/feed", nil, http.StatusOK},
		{"/v1/exports", nil, http.StatusOK},
		{"/v1/exports/none", nil, http.StatusNotFound},
	}
}

// CheckSeededRoutes requests every route against a read-api seeded by
// internal/readapi/seed. The harness checks each response against the
// spec; this function checks the status and the facts the spec cannot.
func CheckSeededRoutes(t *testing.T, h *Harness) {
	t.Helper()
	for _, rt := range seededRoutes() {
		t.Run(rt.path+"?"+rt.query.Encode(), func(t *testing.T) {
			res := h.MustGet(t, rt.path, rt.query)
			if res.Status != rt.want {
				t.Fatalf("status %d, want %d: %s", res.Status, rt.want, res.Body)
			}
		})
	}
	checkFacts(t, h)
}

// decodeBody unmarshals a response body into v.
func decodeBody(t *testing.T, res Result, v any) {
	t.Helper()
	if err := json.Unmarshal(res.Body, v); err != nil {
		t.Fatal(err)
	}
}

type itemsPage struct {
	Items []map[string]any `json:"items"`
	Next  string           `json:"next"`
}

func checkFacts(t *testing.T, h *Harness) {
	t.Helper()
	for name, check := range map[string]func(*testing.T, *Harness){
		"latest resolves and says so":                  checkLatest,
		"old version links to its comparison":          checkOldVersion,
		"pinned text is immutable":                     checkImmutable,
		"untracked catalog entry carries no bill text": checkUntracked,
		"tracked scope lists only tracked bills":       checkTrackedScope,
		"cursor walk covers the catalog once":          checkCursorWalk,
	} {
		t.Run(name, func(t *testing.T) { check(t, h) })
	}
}

func checkLatest(t *testing.T, h *Harness) {
	t.Helper()
	res := h.MustGet(t, "/v1/bills/"+seed.TrackedBill+"/s/s4/l/2", nil)
	var body struct {
		N       int `json:"n"`
		Address struct {
			ResolvedFromLatest bool   `json:"resolved_from_latest"`
			IsLatest           bool   `json:"is_latest"`
			Canonical          string `json:"canonical"`
		} `json:"address"`
	}
	decodeBody(t, res, &body)
	if body.N != latestN || !body.Address.ResolvedFromLatest || !body.Address.IsLatest {
		t.Fatalf("latest answer = %+v", body)
	}
	if loc := res.Header.Get("Content-Location"); loc != body.Address.Canonical || !strings.Contains(loc, "/v/2/") {
		t.Fatalf("Content-Location %q, want pinned %q", loc, body.Address.Canonical)
	}
	if cc := res.Header.Get("Cache-Control"); strings.Contains(cc, "immutable") {
		t.Fatalf("latest answer is cached as immutable: %q", cc)
	}
}

func checkOldVersion(t *testing.T, h *Harness) {
	t.Helper()
	bill := "/v1/bills/" + seed.TrackedBill
	res := h.MustGet(t, bill+"/v/1", nil)
	var body struct {
		Address struct {
			IsLatest bool   `json:"is_latest"`
			Compare  string `json:"compare"`
		} `json:"address"`
	}
	decodeBody(t, res, &body)
	if body.Address.IsLatest || body.Address.Compare != bill+"/v/1/compare/2" {
		t.Fatalf("address = %+v", body.Address)
	}
}

func checkImmutable(t *testing.T, h *Harness) {
	t.Helper()
	res := h.MustGet(t, "/v1/bills/"+seed.TrackedBill+"/v/2/s/s4/l/1-3", nil)
	cc := res.Header.Get("Cache-Control")
	if !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
		t.Fatalf("Cache-Control %q", cc)
	}
}

func checkUntracked(t *testing.T, h *Harness) {
	t.Helper()
	var page itemsPage
	decodeBody(t, h.MustGet(t, billsPath, catalogQuery()), &page)
	if len(page.Items) != catalogSize {
		t.Fatalf("catalog has %d entries, want %d", len(page.Items), catalogSize)
	}
	for _, it := range page.Items {
		if it["tracked"] != false {
			continue
		}
		for _, banned := range []string{"sections", "versions", "address"} {
			if _, has := it[banned]; has {
				t.Fatalf("untracked entry has %q: %v", banned, it)
			}
		}
	}
}

func checkTrackedScope(t *testing.T, h *Harness) {
	t.Helper()
	var page itemsPage
	decodeBody(t, h.MustGet(t, billsPath, nil), &page)
	if len(page.Items) != trackedSize {
		t.Fatalf("tracked list has %d bills, want %d", len(page.Items), trackedSize)
	}
}

func checkCursorWalk(t *testing.T, h *Harness) {
	t.Helper()
	seen := map[string]bool{}
	cursor := ""
	for range catalogSize + 2 {
		q := catalogQuery()
		q.Set("limit", "1")
		if cursor != "" {
			q.Set("cursor", cursor)
		}
		var page itemsPage
		decodeBody(t, h.MustGet(t, billsPath, q), &page)
		for _, it := range page.Items {
			id, _ := it["id"].(string)
			if seen[id] {
				t.Fatalf("%s listed twice", id)
			}
			seen[id] = true
		}
		if cursor = page.Next; cursor == "" {
			break
		}
	}
	if len(seen) != catalogSize {
		t.Fatalf("walk saw %d entries, want %d", len(seen), catalogSize)
	}
}
