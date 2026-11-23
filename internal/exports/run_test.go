//go:build memory

package exports_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/exports"
	"github.com/easterthebunny/pubcommittee/internal/readapi/seed"
)

func setup(t *testing.T) (*memory.Store, exports.Deps) {
	t.Helper()
	s := memory.New()
	if err := seed.Apply(context.Background(), s.PlatformPorts()); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, time.April, 2, 15, 4, 5, 0, time.UTC)
	return s, exports.Deps{
		Ports: s.PlatformPorts(), Log: slog.New(slog.DiscardHandler), Now: func() time.Time { return now },
	}
}

func listIDs(t *testing.T, s *memory.Store) []string {
	t.Helper()
	page, err := s.ListExports(context.Background(), data.Page{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, len(page.Items))
	for _, e := range page.Items {
		ids = append(ids, e.ID)
	}
	return ids
}

func TestRunWritesExportsForEveryTrackedVersion(t *testing.T) {
	t.Parallel()
	s, d := setup(t)
	rep, err := exports.Run(context.Background(), d, exports.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ids := listIDs(t, s)
	for _, want := range []string{
		"bill-text.us-119-hr-4127.ih.json", "bill-source.us-119-hr-4127.ih.xml",
		"change-explanations.us-119-hr-4127.ih.json", "catalog.json",
	} {
		if !slices.Contains(ids, want) {
			t.Errorf("export %s missing from %v", want, ids)
		}
	}
	if rep.Versions != 3 || rep.Bills != 2 {
		t.Errorf("report = %+v, want 2 bills 3 versions", rep)
	}
	checkEntriesAndHeaders(t, s)
}

// checkEntriesAndHeaders requires every indexed entry to state as-of, licence,
// class and hash, and every JSON file's own header to agree.
func checkEntriesAndHeaders(t *testing.T, s *memory.Store) {
	t.Helper()
	page, _ := s.ListExports(context.Background(), data.Page{Limit: 200})
	for _, e := range page.Items {
		if e.Meta.AsOf.IsZero() || e.Licence == "" || e.Meta.ContentClass == "" || e.SHA256 == "" {
			t.Errorf("entry %s lacks as-of, licence, class or hash: %+v", e.ID, e)
		}
		if e.ContentType == "application/json" {
			checkHeader(t, s, e.ID)
		}
	}
	_, body, _ := s.GetExport(context.Background(), "bill-text.us-119-hr-4127.ih.json")
	if !strings.Contains(string(body), `"content_class":"official"`) || !strings.Contains(string(body), `"licence":"public-domain"`) {
		t.Errorf("bill text lacks class or licence: %.300s", body)
	}
}

func checkHeader(t *testing.T, s *memory.Store, id string) {
	t.Helper()
	_, body, err := s.GetExport(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var h exports.Header
	if err := json.Unmarshal(body, &h); err != nil || h.AsOf.IsZero() || h.Licences.Official == "" || h.Licences.Platform == "" {
		t.Errorf("file %s header = %+v, %v", id, h, err)
	}
}

func TestRunIsIdempotentAndRebuildsIdentically(t *testing.T) {
	t.Parallel()
	s, d := setup(t)
	if _, err := exports.Run(context.Background(), d, exports.Options{}); err != nil {
		t.Fatal(err)
	}
	before, _ := s.GetFile(context.Background(), data.ExportIndexPath)
	feedBefore, _ := s.GetFile(context.Background(), data.FeedPath)
	d.Now = func() time.Time { return time.Date(2026, time.April, 9, 1, 0, 0, 0, time.UTC) }
	rep, err := exports.Run(context.Background(), d, exports.Options{})
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	after, _ := s.GetFile(context.Background(), data.ExportIndexPath)
	feedAfter, _ := s.GetFile(context.Background(), data.FeedPath)
	if string(before) != string(after) {
		t.Error("index differs after a rebuild from unchanged data")
	}
	if rep.FeedNew != 0 || string(feedBefore) != string(feedAfter) {
		t.Errorf("second run added %d feed entries", rep.FeedNew)
	}
}

func TestRunDryRunWritesNothing(t *testing.T) {
	t.Parallel()
	s, d := setup(t)
	rep, err := exports.Run(context.Background(), d, exports.Options{DryRun: true})
	if err != nil || len(rep.Written) == 0 {
		t.Fatalf("dry run = %+v, %v", rep, err)
	}
	if ids := listIDs(t, s); len(ids) != 0 {
		t.Errorf("dry run wrote %v", ids)
	}
}

func TestFeedListsEveryEventTypeWithAddresses(t *testing.T) {
	t.Parallel()
	s, d := setup(t)
	if _, err := exports.Run(context.Background(), d, exports.Options{}); err != nil {
		t.Fatal(err)
	}
	page, err := s.ListFeed(context.Background(), time.Time{}, data.Page{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]bool{}
	for _, e := range page.Items {
		kinds[e.Kind] = true
		if e.Address == "" || !e.Updated.Equal(time.Date(2026, time.April, 2, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("entry %+v lacks address or day", e)
		}
	}
	for _, k := range []string{
		data.FeedCatalogBill, data.FeedBillTracked, data.FeedVersion, data.FeedExplanation, data.FeedPolicy, data.FeedLogEntry,
	} {
		if !kinds[k] {
			t.Errorf("feed has no %s entry; kinds %v", k, kinds)
		}
	}
}

// seedExplanation returns the explanation the fixtures store for version 2 of the tracked bill.
func seedExplanation(t *testing.T, s *memory.Store) data.ChangeExplanation {
	t.Helper()
	bill, err := data.ParseBillID(seed.TrackedBill)
	if err != nil {
		t.Fatal(err)
	}
	vs, err := s.ListVersions(context.Background(), bill)
	if err != nil || len(vs) < 2 {
		t.Fatalf("seed versions = %v, %v", vs, err)
	}
	xs, err := s.GetExplanations(context.Background(), vs[1].Ref)
	if err != nil || len(xs) == 0 {
		t.Fatalf("seed explanations = %v, %v", xs, err)
	}
	return xs[0]
}

func TestCorrectedExplanationIsANewFeedEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, d := setup(t)
	if _, err := exports.Run(ctx, d, exports.Options{}); err != nil {
		t.Fatal(err)
	}
	x := seedExplanation(t, s)
	x.Meta.Version++
	x.WhatChanges += " (corrected)"
	if err := s.PutExplanation(ctx, x); err != nil {
		t.Fatal(err)
	}
	rep, err := exports.Run(ctx, d, exports.Options{})
	if err != nil || rep.FeedNew != 1 {
		t.Fatalf("run after correction = %+v, %v; want one new feed entry", rep, err)
	}
	page, _ := s.ListFeed(ctx, time.Time{}, data.Page{Limit: 1})
	if page.Items[0].Kind != data.FeedExplanationCorrected {
		t.Errorf("newest entry = %+v, want a correction", page.Items[0])
	}
}
