package conformance

import (
	"bytes"
	"context"
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// FetcherFixture is the data a BillFetcher suite checks an adapter against.
// The factory seeds the source so these values hold.
type FetcherFixture struct {
	Fetcher      data.BillFetcher
	Bill         data.BillRef
	Title        string
	VersionCodes []string           // codes the status lists, oldest first
	Original     data.SourceVersion // a listed version whose text the source holds
	OriginalXML  []byte             // the XML of Original
	Missing      data.BillRef       // a bill the source does not hold
}

// BillFetcher runs the BillFetcher checks. factory returns a fresh fixture for
// each check.
func BillFetcher(t *testing.T, factory func(t *testing.T) FetcherFixture) {
	t.Helper()
	checks := []struct {
		name string
		run  func(t *testing.T, f FetcherFixture)
	}{
		{"FetchRecord", checkFetchRecord},
		{"FetchRecordUnknown", checkFetchRecordUnknown},
		{"FetchOriginal", checkFetchOriginal},
		{"Cancelled", checkFetchCancelled},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			f := factory(t)
			requireFetcherFixture(t, f)
			c.run(t, f)
		})
	}
}

func requireFetcherFixture(t *testing.T, f FetcherFixture) {
	t.Helper()
	if f.Fetcher == nil {
		t.Fatal("fixture Fetcher is nil")
	}
	if err := f.Bill.Validate(); err != nil {
		t.Fatalf("fixture Bill: %v", err)
	}
	if err := f.Missing.Validate(); err != nil {
		t.Fatalf("fixture Missing: %v", err)
	}
}

func checkFetchRecord(t *testing.T, f FetcherFixture) {
	t.Helper()
	rec, err := f.Fetcher.FetchRecord(context.Background(), f.Bill)
	if err != nil {
		t.Fatalf("FetchRecord: %v", err)
	}
	if rec.Bill != f.Bill {
		t.Errorf("FetchRecord Bill = %s, want %s", rec.Bill, f.Bill)
	}
	if rec.Title != f.Title {
		t.Errorf("FetchRecord Title = %q, want %q", rec.Title, f.Title)
	}
	codes := make([]string, 0, len(rec.Versions))
	for _, v := range rec.Versions {
		codes = append(codes, v.Ref.Version)
		if v.URL == "" {
			t.Errorf("FetchRecord version %q has an empty URL", v.Ref.Version)
		}
	}
	if !slices.Equal(codes, f.VersionCodes) {
		t.Errorf("FetchRecord version codes = %v, want %v", codes, f.VersionCodes)
	}
	if rec.Source.URL == "" {
		t.Error("FetchRecord Source.URL is empty")
	}
	if rec.Source.RetrievedAt.IsZero() {
		t.Error("FetchRecord Source.RetrievedAt is zero")
	}
}

func checkFetchRecordUnknown(t *testing.T, f FetcherFixture) {
	t.Helper()
	_, err := f.Fetcher.FetchRecord(context.Background(), f.Missing)
	wantErr(t, "FetchRecord unknown bill", err, data.ErrNotFound)
}

func checkFetchOriginal(t *testing.T, f FetcherFixture) {
	t.Helper()
	doc, err := f.Fetcher.FetchOriginal(context.Background(), f.Original)
	if err != nil {
		t.Fatalf("FetchOriginal: %v", err)
	}
	if !bytes.Equal(doc.XML, f.OriginalXML) {
		t.Errorf("FetchOriginal XML has %d bytes, want %d equal bytes", len(doc.XML), len(f.OriginalXML))
	}
	if doc.Info.URL == "" {
		t.Error("FetchOriginal Info.URL is empty")
	}
	if doc.Info.RetrievedAt.IsZero() {
		t.Error("FetchOriginal Info.RetrievedAt is zero")
	}
}

func checkFetchCancelled(t *testing.T, f FetcherFixture) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := f.Fetcher.FetchRecord(ctx, f.Bill)
	if err == nil {
		t.Fatal("FetchRecord with a cancelled context: err = nil, want an error")
	}
}
