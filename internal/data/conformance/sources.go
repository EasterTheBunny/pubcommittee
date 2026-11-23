package conformance

import (
	"bytes"
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// Sizes and sample values the suites rely on.
const (
	minBillFixtures   = 3
	minReleasePoints  = 2
	pageLimit         = 2
	unknownBillNumber = 987654
	unknownTitle      = 9999
	batchResultCount  = 2
	genTask           = "summarize"
	genInput          = "bill text"
)

// BillSourceFixture is the seeded data a BillSource suite checks an adapter
// against. The factory seeds the adapter so these values hold.
type BillSourceFixture struct {
	Source  data.BillSource
	Changed []data.ChangedBill // ordered by (UpdatedAt, bill key); at least 3 entries
	Status  data.BillStatus    // Status.Bill == Changed[0].Bill
	Version data.VersionRef    // Version.Bill == Status.Bill
	XML     []byte             // the XML of Version
}

// BillSource runs the BillSource checks. factory returns a fresh fixture for
// each check.
func BillSource(t *testing.T, factory func(t *testing.T) BillSourceFixture) {
	t.Helper()
	checks := []struct {
		name string
		run  func(t *testing.T, f BillSourceFixture)
	}{
		{"ListChanged", checkBillListAll},
		{"ListChangedPages", checkBillPages},
		{"ListChangedSince", checkBillSince},
		{"ListChangedBadCursor", checkBillBadCursor},
		{"FetchStatus", checkBillStatus},
		{"FetchVersionXML", checkBillVersionXML},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			f := factory(t)
			requireBillFixture(t, f)
			c.run(t, f)
		})
	}
}

func requireBillFixture(t *testing.T, f BillSourceFixture) {
	t.Helper()
	if len(f.Changed) < minBillFixtures {
		t.Fatalf("fixture Changed has %d entries, want at least %d", len(f.Changed), minBillFixtures)
	}
	if f.Status.Bill != f.Changed[0].Bill {
		t.Fatalf("fixture Status.Bill = %s, want Changed[0].Bill %s", f.Status.Bill, f.Changed[0].Bill)
	}
	if f.Version.Bill != f.Status.Bill {
		t.Fatalf("fixture Version.Bill = %s, want Status.Bill %s", f.Version.Bill, f.Status.Bill)
	}
}

func checkBillListAll(t *testing.T, f BillSourceFixture) {
	t.Helper()
	page := listChanged(t, f.Source, "", data.Page{Cursor: "", Limit: 0})
	if !reflect.DeepEqual(page.Items, f.Changed) {
		t.Fatalf("ListChanged items = %+v, want %+v", page.Items, f.Changed)
	}
	if page.Next != "" {
		t.Fatalf("ListChanged Next = %q on the last page, want empty", page.Next)
	}
}

func checkBillPages(t *testing.T, f BillSourceFixture) {
	t.Helper()
	var all []data.ChangedBill
	cursor := ""
	for i := 0; ; i++ {
		if i > len(f.Changed) {
			t.Fatalf("ListChanged paging did not end after %d pages", i)
		}
		page := listChanged(t, f.Source, "", data.Page{Cursor: cursor, Limit: pageLimit})
		if len(page.Items) > pageLimit {
			t.Fatalf("ListChanged page has %d items with Limit 2", len(page.Items))
		}
		all = append(all, page.Items...)
		if page.Next == "" {
			break
		}
		first := listChanged(t, f.Source, "", data.Page{Cursor: page.Next, Limit: pageLimit})
		second := listChanged(t, f.Source, "", data.Page{Cursor: page.Next, Limit: pageLimit})
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("cursor %q returned different pages on re-read", page.Next)
		}
		cursor = page.Next
	}
	if !reflect.DeepEqual(all, f.Changed) {
		t.Fatalf("paged ListChanged items = %+v, want %+v", all, f.Changed)
	}
}

func checkBillSince(t *testing.T, f BillSourceFixture) {
	t.Helper()
	since := f.Changed[1].UpdatedAt
	from := parseRFC3339(t, since)
	var want []data.ChangedBill
	for _, c := range f.Changed {
		if !parseRFC3339(t, c.UpdatedAt).Before(from) {
			want = append(want, c)
		}
	}
	page := listChanged(t, f.Source, since, data.Page{Cursor: "", Limit: 0})
	if !reflect.DeepEqual(page.Items, want) {
		t.Fatalf("ListChanged since %s = %+v, want %+v", since, page.Items, want)
	}
}

func checkBillBadCursor(t *testing.T, f BillSourceFixture) {
	t.Helper()
	_, err := f.Source.ListChanged(context.Background(), "", data.Page{Cursor: "!!", Limit: 0})
	wantErr(t, "ListChanged bad cursor", err, data.ErrInvalid)
}

func checkBillStatus(t *testing.T, f BillSourceFixture) {
	t.Helper()
	ctx := context.Background()
	got, err := f.Source.FetchStatus(ctx, f.Status.Bill)
	if err != nil {
		t.Fatalf("FetchStatus: %v", err)
	}
	if !reflect.DeepEqual(got, f.Status) {
		t.Fatalf("FetchStatus = %+v, want %+v", got, f.Status)
	}
	_, err = f.Source.FetchStatus(ctx, SampleBill(unknownBillNumber))
	wantErr(t, "FetchStatus unknown bill", err, data.ErrNotFound)
	var noBill data.BillRef
	_, err = f.Source.FetchStatus(ctx, noBill)
	wantErr(t, "FetchStatus invalid ref", err, data.ErrInvalid)
}

func checkBillVersionXML(t *testing.T, f BillSourceFixture) {
	t.Helper()
	ctx := context.Background()
	got, err := f.Source.FetchVersionXML(ctx, f.Version)
	if err != nil {
		t.Fatalf("FetchVersionXML: %v", err)
	}
	if !bytes.Equal(got, f.XML) {
		t.Fatalf("FetchVersionXML = %q, want %q", got, f.XML)
	}
	if len(got) > 0 {
		got[0] ^= 0xff // the caller's copy must not reach the stored XML
		again, err := f.Source.FetchVersionXML(ctx, f.Version)
		if err != nil {
			t.Fatalf("FetchVersionXML re-read: %v", err)
		}
		if !bytes.Equal(again, f.XML) {
			t.Fatalf("mutating a FetchVersionXML result changed the stored XML: %q", again)
		}
	}
	_, err = f.Source.FetchVersionXML(ctx, data.VersionRef{Bill: f.Version.Bill, Version: "zz"})
	wantErr(t, "FetchVersionXML unknown version", err, data.ErrNotFound)
}

// LawSourceFixture is the seeded data a LawSource suite checks an adapter
// against. The factory seeds the adapter so these values hold.
type LawSourceFixture struct {
	Source data.LawSource
	Points []data.ReleasePoint // newest first; at least 2
	Title  int                 // a valid US Code title number
	XML    []byte              // the XML FetchTitle(Points[0], Title) returns
}

// LawSource runs the LawSource checks. factory returns a fresh fixture for
// each check.
func LawSource(t *testing.T, factory func(t *testing.T) LawSourceFixture) {
	t.Helper()
	checks := []struct {
		name string
		run  func(t *testing.T, f LawSourceFixture)
	}{
		{"ListReleasePoints", checkLawPoints},
		{"FetchTitle", checkLawTitle},
		{"FetchTitleErrors", checkLawErrors},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			f := factory(t)
			requireLawFixture(t, f)
			c.run(t, f)
		})
	}
}

func requireLawFixture(t *testing.T, f LawSourceFixture) {
	t.Helper()
	if len(f.Points) < minReleasePoints {
		t.Fatalf("fixture Points has %d entries, want at least %d", len(f.Points), minReleasePoints)
	}
	if f.Title < 1 {
		t.Fatalf("fixture Title = %d, want at least 1", f.Title)
	}
}

func checkLawPoints(t *testing.T, f LawSourceFixture) {
	t.Helper()
	got, err := f.Source.ListReleasePoints(context.Background())
	if err != nil {
		t.Fatalf("ListReleasePoints: %v", err)
	}
	if !reflect.DeepEqual(got, f.Points) {
		t.Fatalf("ListReleasePoints = %+v, want %+v", got, f.Points)
	}
}

func checkLawTitle(t *testing.T, f LawSourceFixture) {
	t.Helper()
	got, err := f.Source.FetchTitle(context.Background(), f.Points[0], f.Title)
	if err != nil {
		t.Fatalf("FetchTitle: %v", err)
	}
	if !bytes.Equal(got, f.XML) {
		t.Fatalf("FetchTitle = %q, want %q", got, f.XML)
	}
}

func checkLawErrors(t *testing.T, f LawSourceFixture) {
	t.Helper()
	ctx := context.Background()
	_, err := f.Source.FetchTitle(ctx, data.ReleasePoint{ID: "no-such-release"}, f.Title)
	wantErr(t, "FetchTitle unknown release point", err, data.ErrNotFound)
	_, err = f.Source.FetchTitle(ctx, f.Points[0], unknownTitle)
	wantErr(t, "FetchTitle unknown title", err, data.ErrNotFound)
	_, err = f.Source.FetchTitle(ctx, f.Points[0], 0)
	wantErr(t, "FetchTitle title 0", err, data.ErrInvalid)
}

// Generator runs the Generator checks. factory returns a fresh Generator for
// each check.
func Generator(t *testing.T, factory func(t *testing.T) data.Generator) {
	t.Helper()
	checks := []struct {
		name string
		run  func(t *testing.T, g data.Generator)
	}{
		{"SubmitEmpty", checkGenSubmitEmpty},
		{"SubmitCollect", checkGenSubmitCollect},
		{"CollectUnknown", checkGenCollectUnknown},
		{"Generate", checkGenGenerate},
		{"GenerateInvalid", checkGenGenerateInvalid},
		{"SubmitDuplicateIDs", checkGenSubmitDuplicate},
		{"CollectIdempotent", checkGenCollectTwice},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, factory(t))
		})
	}
}

func genRequests() []data.GenRequest {
	return []data.GenRequest{
		{ID: "a", Task: genTask, Input: "first bill text"},
		{ID: "b", Task: genTask, Input: "second bill text"},
	}
}

func checkGenSubmitEmpty(t *testing.T, g data.Generator) {
	t.Helper()
	_, err := g.Submit(context.Background(), []data.GenRequest{})
	wantErr(t, "Submit empty", err, data.ErrInvalid)
}

func checkGenSubmitCollect(t *testing.T, g data.Generator) {
	t.Helper()
	id := submitBatch(t, g)
	results, done := collectBatch(t, g, id)
	if !done {
		t.Fatal("Collect done = false, want true")
	}
	if len(results) != batchResultCount {
		t.Fatalf("Collect returned %d results, want %d", len(results), batchResultCount)
	}
	ids := make([]string, 0, len(results))
	for _, r := range results {
		ids = append(ids, r.ID)
		if r.Outcome == "" {
			t.Errorf("result %q has an empty Outcome", r.ID)
		}
	}
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"a", "b"}) {
		t.Fatalf("result IDs = %v, want [a b]", ids)
	}
}

func checkGenCollectUnknown(t *testing.T, g data.Generator) {
	t.Helper()
	_, _, err := g.Collect(context.Background(), "no-such-batch")
	wantErr(t, "Collect unknown batch", err, data.ErrNotFound)
}

func checkGenGenerate(t *testing.T, g data.Generator) {
	t.Helper()
	res, err := g.Generate(context.Background(), data.GenRequest{ID: "g1", Task: genTask, Input: genInput})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if res.Outcome != "ok" {
		t.Fatalf("Generate Outcome = %q, want ok", res.Outcome)
	}
	if res.Text == "" {
		t.Fatal("Generate Text is empty")
	}
}

func checkGenGenerateInvalid(t *testing.T, g data.Generator) {
	t.Helper()
	ctx := context.Background()
	_, err := g.Generate(ctx, data.GenRequest{ID: "", Task: genTask, Input: genInput})
	wantErr(t, "Generate empty ID", err, data.ErrInvalid)
	_, err = g.Generate(ctx, data.GenRequest{ID: "g2", Task: "", Input: genInput})
	wantErr(t, "Generate empty Task", err, data.ErrInvalid)
}

func checkGenSubmitDuplicate(t *testing.T, g data.Generator) {
	t.Helper()
	reqs := []data.GenRequest{
		{ID: "a", Task: genTask, Input: "x"},
		{ID: "a", Task: genTask, Input: "y"},
	}
	_, err := g.Submit(context.Background(), reqs)
	wantErr(t, "Submit duplicate request IDs", err, data.ErrInvalid)
}

func checkGenCollectTwice(t *testing.T, g data.Generator) {
	t.Helper()
	id := submitBatch(t, g)
	first, done1 := collectBatch(t, g, id)
	second, done2 := collectBatch(t, g, id)
	if !done1 || !done2 {
		t.Fatalf("Collect done = %t then %t, want true both times", done1, done2)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("second Collect = %+v, want %+v", second, first)
	}
}

func submitBatch(t *testing.T, g data.Generator) string {
	t.Helper()
	id, err := g.Submit(context.Background(), genRequests())
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if id == "" {
		t.Fatal("Submit returned an empty batch ID")
	}
	return id
}

func collectBatch(t *testing.T, g data.Generator, id string) ([]data.GenResult, bool) {
	t.Helper()
	results, done, err := g.Collect(context.Background(), id)
	if err != nil {
		t.Fatalf("Collect(%q): %v", id, err)
	}
	return results, done
}

func listChanged(t *testing.T, src data.BillSource, since string, p data.Page) data.Paged[data.ChangedBill] {
	t.Helper()
	page, err := src.ListChanged(context.Background(), since, p)
	if err != nil {
		t.Fatalf("ListChanged(%q, %+v): %v", since, p, err)
	}
	return page
}

func parseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	at, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("fixture timestamp %q: %v", s, err)
	}
	return at
}

func wantErr(t *testing.T, what string, err, want error) {
	t.Helper()
	if !errors.Is(err, want) {
		t.Fatalf("%s: err = %v, want %v", what, err, want)
	}
}
