package readapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/exports"
	"github.com/easterthebunny/pubcommittee/internal/readapi"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

// serveExported serves a seeded store after one exports run.
func serveExported(t *testing.T) string {
	t.Helper()
	store := memory.New()
	ctx := context.Background()
	if err := seed.Apply(ctx, store.PlatformPorts()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.April, 2, 15, 0, 0, 0, time.UTC)
	d := exports.Deps{
		Ports: store.PlatformPorts(), Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
	}
	if _, err := exports.Run(ctx, d, exports.Options{}); err != nil {
		t.Fatal(err)
	}
	h, err := readapi.Handler(readapi.New(store.PublicReadPorts()), readapi.Options{
		RateLimits: readapi.RateLimits{}, Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv.URL
}

type feedItem struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Updated string `json:"updated"`
	Address string `json:"address"`
}

type feedPage struct {
	Items []feedItem `json:"items"`
	Next  *string    `json:"next"`
}

// walkFeed pages through the feed 3 entries at a time and returns the entries.
func walkFeed(t *testing.T, url string) []feedItem {
	t.Helper()
	var seen []feedItem
	next := ""
	for range 20 {
		r := do(t, http.MethodGet, url+"/v1/feed?limit=3&cursor="+next, nil)
		if r.status != http.StatusOK {
			t.Fatalf("status %d: %s", r.status, r.body)
		}
		var p feedPage
		if err := json.Unmarshal(r.body, &p); err != nil {
			t.Fatal(err)
		}
		seen = append(seen, p.Items...)
		if p.Next == nil {
			break
		}
		next = *p.Next
	}
	return seen
}

func TestFeedPagesAndEveryEntryReachesItsAddress(t *testing.T) {
	t.Parallel()
	url := serveExported(t)
	seen := walkFeed(t, url)
	if len(seen) < 8 {
		t.Fatalf("walked only %v", seen)
	}
	for _, e := range seen {
		if e.Updated != "2026-04-02" {
			t.Errorf("%s: updated %q is not the day", e.ID, e.Updated)
		}
		got := do(t, http.MethodGet, url+e.Address, nil)
		if got.status != http.StatusOK {
			t.Errorf("%s: address %s answered %d", e.ID, e.Address, got.status)
		}
		if e.Kind == "catalog-bill" && !strings.Contains(string(got.body), strings.TrimPrefix(e.Address, "/v1/bills?scope=catalog&q=")) {
			t.Errorf("%s: catalog address does not list the bill", e.ID)
		}
	}
}

func TestFeedSinceAndBadCursor(t *testing.T) {
	t.Parallel()
	url := serveExported(t)
	total := len(walkFeed(t, url))
	for path, want := range map[string]int{
		"/v1/feed?since=2026-04-02T23:59:00Z&limit=200": total, // the same day keeps its entries
		"/v1/feed?since=2026-04-03T00:00:00Z&limit=200": 0,
	} {
		var p feedPage
		if err := json.Unmarshal(do(t, http.MethodGet, url+path, nil).body, &p); err != nil {
			t.Fatal(err)
		}
		if len(p.Items) != want {
			t.Errorf("%s: %d items, want %d", path, len(p.Items), want)
		}
	}
	if got := do(t, http.MethodGet, url+"/v1/feed?cursor=zz", nil).status; got != http.StatusBadRequest {
		t.Errorf("bad cursor status %d, want 400", got)
	}
}

func TestExportsIndexAndFiles(t *testing.T) {
	t.Parallel()
	url := serveExported(t)
	var idx struct {
		Items []struct {
			ID          string `json:"id"`
			Href        string `json:"href"`
			Licence     string `json:"licence"`
			ContentType string `json:"content_type"`
			AsOf        string `json:"as_of"`
		} `json:"items"`
	}
	r := do(t, http.MethodGet, url+"/v1/exports?limit=200", nil)
	if err := json.Unmarshal(r.body, &idx); err != nil || len(idx.Items) < 6 {
		t.Fatalf("index = %v, %s", err, r.body)
	}
	for _, e := range idx.Items {
		f := do(t, http.MethodGet, url+e.Href, nil)
		if f.status != http.StatusOK || f.header.Get("Content-Type") != e.ContentType {
			t.Errorf("%s: %d %q, want 200 %q", e.ID, f.status, f.header.Get("Content-Type"), e.ContentType)
		}
		if !strings.Contains(f.header.Get("Cache-Control"), "max-age=900") || e.Licence == "" || e.AsOf == "" {
			t.Errorf("%s: cache %q licence %q as_of %q", e.ID, f.header.Get("Cache-Control"), e.Licence, e.AsOf)
		}
	}
	if got := do(t, http.MethodGet, url+"/v1/exports/missing.json", nil).status; got != http.StatusNotFound {
		t.Errorf("missing export status %d, want 404", got)
	}
}

func TestRobotsAndDiscoveryNotice(t *testing.T) {
	t.Parallel()
	url := serveExported(t)
	r := do(t, http.MethodGet, url+"/robots.txt", nil)
	if r.status != http.StatusOK || !strings.Contains(string(r.body), "Allow: /") || strings.Contains(string(r.body), "Disallow: /\n") {
		t.Errorf("robots.txt = %d %q", r.status, r.body)
	}
	d := do(t, http.MethodGet, url+"/v1", nil)
	if !strings.Contains(string(d.body), `"href":"/.well-known/machine-access.json","rel":"machine-access"`) {
		t.Errorf("discovery lacks the notice link: %s", d.body)
	}
}
