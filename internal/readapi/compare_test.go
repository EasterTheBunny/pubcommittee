package readapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/readapi"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

func TestCompareVersions(t *testing.T) {
	t.Parallel()
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
	path := srv.URL + "/v1/bills/" + seed.TrackedBill + "/v/1/compare/2"

	if got := do(t, http.MethodGet, path, nil); got.status != http.StatusNotFound {
		t.Fatalf("before the step: status %d, want 404", got.status)
	}
	n, err := versiondiff.Step(ctx, store, store, billRef(t, store), billxml.ParserVersion)
	if err != nil || n != 1 {
		t.Fatalf("Step = %d, %v; want 1 comparison", n, err)
	}
	checkCompareBody(t, do(t, http.MethodGet, path, nil))
	for _, p := range []string{"/v/2/compare/1", "/v/1/compare/1", "/v/1/compare/9"} {
		if r := do(t, http.MethodGet, srv.URL+"/v1/bills/"+seed.TrackedBill+p, nil); r.status != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", p, r.status)
		}
	}
}

func checkCompareBody(t *testing.T, got reply) {
	t.Helper()
	if got.status != http.StatusOK {
		t.Fatalf("status %d: %s", got.status, got.body)
	}
	var vc apiv1.VersionComparison
	if err := json.Unmarshal(got.body, &vc); err != nil {
		t.Fatal(err)
	}
	if vc.FromN != 1 || vc.ToN != 2 || len(vc.Sections) == 0 {
		t.Errorf("comparison = %+v", vc)
	}
	for _, s := range vc.Sections {
		for _, c := range s.Changes {
			if c.Before == nil && c.After == nil {
				t.Errorf("change without text: %+v", c)
			}
		}
	}
	checkCompareHeaders(t, got)
}

func billRef(t *testing.T, s *memory.Store) data.BillRef {
	t.Helper()
	page, err := s.ListCatalog(context.Background(), data.Page{})
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range page.Items {
		if e.Tracked {
			return e.Bill
		}
	}
	t.Fatal("no tracked bill in seed")
	return data.BillRef{}
}

func checkCompareHeaders(t *testing.T, got reply) {
	t.Helper()
	if cc := got.header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("Cache-Control = %q, want immutable", cc)
	}
}
