package conformance

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func sampleExport(id string) data.ExportEntry {
	return data.ExportEntry{
		ID: id, Kind: data.ExportOther, Title: "Sample " + id, ContentType: "application/json",
		Licence: data.LicenceCC0,
		Meta: data.Meta{
			Version: 1, AsOf: SampleMeta().AsOf, Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform,
		},
	}
}

func sampleFeed(id string, day time.Time) data.FeedEntry {
	return data.FeedEntry{
		ID: id, Updated: day, Title: "Sample " + id, Kind: data.FeedCatalogBill, Address: "/v1/bills/" + id,
		Meta: data.Meta{Version: 1, AsOf: day, Provenance: data.ProvenanceSource, ContentClass: data.ContentPlatform},
	}
}

// Exports runs the conformance checks for ExportReader and ExportWriter.
// factory returns a fresh pair for each subtest. Not checked: concurrent
// writers.
func Exports(t *testing.T, factory func(t *testing.T) (data.ExportReader, data.ExportWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.ExportReader, w data.ExportWriter)
	}{
		{"empty index", func(t *testing.T, r data.ExportReader, _ data.ExportWriter) {
			t.Helper()
			page, err := r.ListExports(context.Background(), data.Page{})
			if err != nil || len(page.Items) != 0 {
				t.Fatalf("ListExports = %v, %v; want empty page", page, err)
			}
			if _, _, err := r.GetExport(context.Background(), "nothing.json"); !errors.Is(err, data.ErrNotFound) {
				t.Fatalf("GetExport = %v, want ErrNotFound", err)
			}
		}},
		{"round trip and hash", checkExportRoundTrip},
		{"write once and replace", checkExportReplace},
		{"invalid", checkExportInvalid},
		{"paging by id", checkExportPaging},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func checkExportRoundTrip(t *testing.T, r data.ExportReader, w data.ExportWriter) {
	t.Helper()
	ctx := context.Background()
	body := []byte(`{"a":1}`)
	e, err := w.PutExport(ctx, sampleExport("one.json"), body, false)
	if err != nil || e.SHA256 == "" || e.SizeBytes != len(body) {
		t.Fatalf("PutExport = %+v, %v", e, err)
	}
	if _, _, err := r.GetExport(ctx, "one.json"); !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("GetExport before index = %v; want ErrNotFound", err)
	}
	if err := w.PutExportIndex(ctx, []data.ExportEntry{e}); err != nil {
		t.Fatal(err)
	}
	got, gotBody, err := r.GetExport(ctx, "one.json")
	if err != nil || string(gotBody) != string(body) || got.SHA256 != e.SHA256 || got.Licence != data.LicenceCC0 {
		t.Fatalf("GetExport = %+v, %q, %v", got, gotBody, err)
	}
	checkExportStates(t, got)
}

func checkExportStates(t *testing.T, got data.ExportEntry) {
	t.Helper()
	if got.Meta.AsOf.IsZero() || got.Meta.ContentClass == "" {
		t.Fatalf("as-of or content class lost: %+v", got.Meta)
	}
}

func checkExportReplace(t *testing.T, _ data.ExportReader, w data.ExportWriter) {
	t.Helper()
	ctx := context.Background()
	if _, err := w.PutExport(ctx, sampleExport("once.json"), []byte("1"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := w.PutExport(ctx, sampleExport("once.json"), []byte("1"), false); err != nil {
		t.Fatalf("identical write-once = %v, want no-op", err)
	}
	if _, err := w.PutExport(ctx, sampleExport("once.json"), []byte("2"), false); !errors.Is(err, data.ErrConflict) {
		t.Fatalf("different write-once = %v, want ErrConflict", err)
	}
	for _, body := range []string{"1", "2"} {
		if _, err := w.PutExport(ctx, sampleExport("again.json"), []byte(body), true); err != nil {
			t.Fatalf("replace %s = %v", body, err)
		}
	}
}

func checkExportInvalid(t *testing.T, _ data.ExportReader, w data.ExportWriter) {
	t.Helper()
	ctx := context.Background()
	bad := sampleExport("Bad Id")
	if _, err := w.PutExport(ctx, bad, []byte("x"), false); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("bad id = %v, want ErrInvalid", err)
	}
	noLicence := sampleExport("nolicence.json")
	noLicence.Licence = ""
	if _, err := w.PutExport(ctx, noLicence, []byte("x"), false); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("no licence = %v, want ErrInvalid", err)
	}
	dup := []data.ExportEntry{sampleExport("d.json"), sampleExport("d.json")}
	if err := w.PutExportIndex(ctx, dup); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("duplicate ids = %v, want ErrInvalid", err)
	}
}

func checkExportPaging(t *testing.T, r data.ExportReader, w data.ExportWriter) {
	t.Helper()
	ctx := context.Background()
	entries := make([]data.ExportEntry, 0, 3)
	for _, id := range []string{"c.json", "a.json", "b.json"} {
		e, err := w.PutExport(ctx, sampleExport(id), []byte(id), true)
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, e)
	}
	if err := w.PutExportIndex(ctx, entries); err != nil {
		t.Fatal(err)
	}
	var ids []string
	cursor := ""
	for range 5 {
		page, err := r.ListExports(ctx, data.Page{Cursor: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Items {
			ids = append(ids, e.ID)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	if fmt.Sprint(ids) != "[a.json b.json c.json]" {
		t.Fatalf("ids = %v, want ordered by id without repeats", ids)
	}
	if _, err := r.ListExports(ctx, data.Page{Cursor: "!!not a cursor"}); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("bad cursor = %v, want ErrInvalid", err)
	}
}

// Feed runs the conformance checks for FeedReader and FeedWriter. factory
// returns a fresh pair for each subtest. Not checked: concurrent writers.
func Feed(t *testing.T, factory func(t *testing.T) (data.FeedReader, data.FeedWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.FeedReader, w data.FeedWriter)
	}{
		{"empty", func(t *testing.T, r data.FeedReader, _ data.FeedWriter) {
			t.Helper()
			page, err := r.ListFeed(context.Background(), time.Time{}, data.Page{})
			if err != nil || len(page.Items) != 0 || page.Next != "" {
				t.Fatalf("ListFeed = %v, %v; want empty page", page, err)
			}
		}},
		{"append is idempotent and newest first", checkFeedAppend},
		{"since keeps the day", checkFeedSince},
		{"paging", checkFeedPaging},
		{"invalid", func(t *testing.T, r data.FeedReader, w data.FeedWriter) {
			t.Helper()
			ctx := context.Background()
			bad := sampleFeed("x", SampleMeta().AsOf)
			bad.Address = ""
			if _, err := w.AppendFeed(ctx, []data.FeedEntry{bad}); !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("no address = %v, want ErrInvalid", err)
			}
			if _, err := r.ListFeed(ctx, time.Time{}, data.Page{Cursor: "zz"}); !errors.Is(err, data.ErrInvalid) {
				t.Fatalf("bad cursor = %v, want ErrInvalid", err)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func feedIDs(t *testing.T, r data.FeedReader, since time.Time) []string {
	t.Helper()
	page, err := r.ListFeed(context.Background(), since, data.Page{})
	if err != nil {
		t.Fatal(err)
	}
	ids := make([]string, len(page.Items))
	for i, e := range page.Items {
		ids[i] = e.ID
	}
	return ids
}

func checkFeedAppend(t *testing.T, r data.FeedReader, w data.FeedWriter) {
	t.Helper()
	ctx := context.Background()
	day := SampleMeta().AsOf.UTC().Truncate(24 * time.Hour)
	in := []data.FeedEntry{sampleFeed("a", day), sampleFeed("b", day)}
	if n, err := w.AppendFeed(ctx, in); err != nil || n != 2 {
		t.Fatalf("AppendFeed = %d, %v; want 2", n, err)
	}
	if n, err := w.AppendFeed(ctx, append(in, sampleFeed("c", day))); err != nil || n != 1 {
		t.Fatalf("second AppendFeed = %d, %v; want 1 (only the new id)", n, err)
	}
	if got := fmt.Sprint(feedIDs(t, r, time.Time{})); got != "[c b a]" {
		t.Fatalf("ids = %s, want newest first [c b a]", got)
	}
	page, _ := r.ListFeed(ctx, time.Time{}, data.Page{})
	if !page.Items[0].Updated.Equal(day) {
		t.Fatalf("updated = %v, want the day %v", page.Items[0].Updated, day)
	}
}

func checkFeedSince(t *testing.T, r data.FeedReader, w data.FeedWriter) {
	t.Helper()
	day := SampleMeta().AsOf.UTC().Truncate(24 * time.Hour)
	for i, id := range []string{"old", "mid", "new"} {
		if _, err := w.AppendFeed(context.Background(), []data.FeedEntry{sampleFeed(id, day.AddDate(0, 0, i))}); err != nil {
			t.Fatal(err)
		}
	}
	noon := day.AddDate(0, 0, 1).Add(12 * time.Hour)
	if got := fmt.Sprint(feedIDs(t, r, noon)); got != "[new mid]" {
		t.Fatalf("since mid-day = %s, want [new mid]", got)
	}
	if got := fmt.Sprint(feedIDs(t, r, day.AddDate(0, 0, 3))); got != "[]" {
		t.Fatalf("since after all = %s, want []", got)
	}
}

func checkFeedPaging(t *testing.T, r data.FeedReader, w data.FeedWriter) {
	t.Helper()
	ctx := context.Background()
	day := SampleMeta().AsOf.UTC().Truncate(24 * time.Hour)
	in := make([]data.FeedEntry, 0, 5)
	for i := range 5 {
		in = append(in, sampleFeed(fmt.Sprintf("e%d", i), day))
	}
	if _, err := w.AppendFeed(ctx, in); err != nil {
		t.Fatal(err)
	}
	var ids []string
	cursor := ""
	for range 10 {
		page, err := r.ListFeed(ctx, time.Time{}, data.Page{Cursor: cursor, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page.Items {
			ids = append(ids, e.ID)
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
		// An entry appended mid-walk does not disturb the pages after the cursor.
		if _, err := w.AppendFeed(ctx, []data.FeedEntry{sampleFeed("late"+cursor, day)}); err != nil {
			t.Fatal(err)
		}
	}
	if got := fmt.Sprint(ids); got != "[e4 e3 e2 e1 e0]" {
		t.Fatalf("walk = %s, want [e4 e3 e2 e1 e0]", got)
	}
}
