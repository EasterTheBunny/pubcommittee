package readapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/citations"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
	"github.com/easterthebunny/pubcommittee/internal/readapi"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

// citationsStore starts a seeded memory store and a read-api server over it.
func citationsStore(t *testing.T) (*memory.Store, string) {
	t.Helper()
	store := memory.New()
	ctx := context.Background()
	if err := seed.Apply(ctx, store.PlatformPorts()); err != nil {
		t.Fatal(err)
	}
	h, err := readapi.Handler(readapi.New(store.PublicReadPorts()), readapi.Options{
		Logger: slog.New(slog.NewJSONHandler(&bytes.Buffer{}, nil)), RateLimits: readapi.RateLimits{},
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return store, srv.URL
}

// storeCitations stores a release and a citation set for version 1 of the
// tracked bill, as the citation step would.
func storeCitations(t *testing.T, store *memory.Store) {
	t.Helper()
	ctx := context.Background()
	const release = "119-142"
	if err := store.PutRelease(ctx, conformance.SampleRelease(release, 10)); err != nil {
		t.Fatal(err)
	}
	if err := store.SetCurrentRelease(ctx, release); err != nil {
		t.Fatal(err)
	}
	vs, err := store.ListVersions(ctx, billRef(t, store))
	if err != nil || len(vs) == 0 {
		t.Fatalf("ListVersions = %d, %v", len(vs), err)
	}
	set := conformance.SampleCitationSet(1, citations.Method(billxml.ParserVersion, release))
	set.Ref.Version = vs[0].Ref
	for i := range set.Links {
		set.Links[i].Version = vs[0].Ref
	}
	if err := store.PutCitations(ctx, set); err != nil {
		t.Fatal(err)
	}
}

func TestCitations(t *testing.T) {
	t.Parallel()
	store, base := citationsStore(t)
	bill := base + "/v1/bills/" + seed.TrackedBill + "/v/1/citations"

	if got := do(t, http.MethodGet, bill, nil); got.status != http.StatusNotFound {
		t.Fatalf("no release stored: status %d, want 404", got.status)
	}
	storeCitations(t, store)

	t.Run("list", func(t *testing.T) {
		t.Parallel()
		checkCitationList(t, bill)
	})
	t.Run("state filter", func(t *testing.T) {
		t.Parallel()
		page := citationPage(t, do(t, http.MethodGet, bill+"?state=resolved", nil))
		if len(page.Items) != 1 || page.Items[0].State != apiv1.Resolved {
			t.Errorf("resolved items = %+v", page.Items)
		}
	})
	t.Run("unresolved", func(t *testing.T) {
		t.Parallel()
		page := citationPage(t, do(t, http.MethodGet, bill+"/unresolved", nil))
		if len(page.Items) != 1 || page.Items[0].Text != "Public Law 111-163" {
			t.Errorf("unresolved items = %+v", page.Items)
		}
	})
	t.Run("paging", func(t *testing.T) {
		t.Parallel()
		checkCitationPaging(t, bill)
	})
	t.Run("other version and bills", func(t *testing.T) {
		t.Parallel()
		for _, u := range []string{
			base + "/v1/bills/" + seed.TrackedBill + "/v/2/citations",
			base + "/v1/bills/" + seed.TrackedBill + "/v/2/citations/unresolved",
			base + "/v1/bills/us-119-hr-9999/v/1/citations",
		} {
			if r := do(t, http.MethodGet, u, nil); r.status != http.StatusNotFound {
				t.Errorf("%s: status %d, want 404", u, r.status)
			}
		}
	})
	t.Run("bad cursor", func(t *testing.T) {
		t.Parallel()
		if r := do(t, http.MethodGet, bill+"?cursor=!!!", nil); r.status != http.StatusBadRequest {
			t.Errorf("status %d, want 400", r.status)
		}
	})
}

// citationPage decodes a 200 reply holding a citation page.
func citationPage(t *testing.T, got reply) apiv1.CitationPage {
	t.Helper()
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var page apiv1.CitationPage
	if err := json.Unmarshal(got.body, &page); err != nil {
		t.Fatal(err)
	}
	return page
}

func checkCitationList(t *testing.T, bill string) {
	t.Helper()
	page := citationPage(t, do(t, http.MethodGet, bill, nil))
	if len(page.Items) != 2 || page.Next != nil {
		t.Fatalf("items = %d, next = %v; want 2 and none", len(page.Items), page.Next)
	}
	checkResolvedCitation(t, page.Items[0])
	checkUnresolvedCitation(t, page.Items[1])
	for _, c := range page.Items {
		if c.Bill != seed.TrackedBill || c.N != 1 {
			t.Errorf("bill %q n %d; want %q and 1", c.Bill, c.N, seed.TrackedBill)
		}
	}
}

func checkResolvedCitation(t *testing.T, c apiv1.Citation) {
	t.Helper()
	if c.Text != "42 U.S.C. 1395cc-7(a)" || c.State != apiv1.Resolved ||
		len(c.ProvisionIds) != 1 || c.ProvisionIds[0] != "usc/42/1395cc-7/a" ||
		c.Line == nil || *c.Line != 3 || c.SectionId != "s2" {
		t.Errorf("resolved = %+v", c)
	}
}

func checkUnresolvedCitation(t *testing.T, c apiv1.Citation) {
	t.Helper()
	if c.Text != "Public Law 111-163" || c.State != apiv1.CitationState(data.CitationUnresolved) ||
		c.ProvisionIds == nil || len(c.ProvisionIds) != 0 ||
		c.Line == nil || *c.Line != 5 || c.SectionId != "s3" {
		t.Errorf("unresolved = %+v", c)
	}
}

func checkCitationPaging(t *testing.T, bill string) {
	t.Helper()
	p1 := citationPage(t, do(t, http.MethodGet, bill+"?limit=1", nil))
	if len(p1.Items) != 1 || p1.Items[0].State != apiv1.Resolved || p1.Next == nil {
		t.Fatalf("first page = %+v", p1)
	}
	p2 := citationPage(t, do(t, http.MethodGet, bill+"?limit=1&cursor="+*p1.Next, nil))
	if len(p2.Items) != 1 || p2.Items[0].Text != "Public Law 111-163" || p2.Next != nil {
		t.Errorf("second page = %+v", p2)
	}
}
