package conformance

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// SampleComparison returns a valid comparison of version codes "ih" and "rh"
// of bill n; method varies the ref's Method.
func SampleComparison(n int, method string) data.Comparison {
	return data.Comparison{
		Ref:        data.ComparisonRef{Bill: SampleBill(n), From: "ih", To: "rh", Method: method},
		FromSHA256: "00", ToSHA256: "01",
		LineCountOld: 2, LineCountNew: 2,
		Sections: []data.SectionComparison{{
			Path: []string{"s1"}, NewPath: []string{"s1"}, Status: data.SectionChanged,
			Lines: []data.LineMapping{
				{N: 1, Class: data.LineUnchanged, New: &data.LineLoc{Section: []string{"s1"}, N: 1}},
				{N: 2, Class: data.LineRemoved},
			},
			Changes: []data.ComparisonChange{
				{Kind: data.ChangeRemoved, BeforeSection: []string{"s1"}, BeforeFrom: 2, BeforeTo: 2},
			},
		}},
		Meta: data.Meta{
			Version: 1, AsOf: SampleMeta().AsOf,
			Provenance: data.ProvenanceAuto, ContentClass: data.ContentPlatform,
		},
	}
}

// Comparisons runs the conformance checks for ComparisonReader and
// ComparisonWriter. factory returns a fresh pair for each subtest; both
// values may be the same adapter. Not checked: concurrent writers.
func Comparisons(t *testing.T, factory func(t *testing.T) (data.ComparisonReader, data.ComparisonWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.ComparisonReader, w data.ComparisonWriter)
	}{
		{"missing", checkComparisonMissing},
		{"round trip", checkComparisonRoundTrip},
		{"write once", checkComparisonWriteOnce},
		{"invalid", checkComparisonInvalid},
		{"keys differ", checkComparisonKeys},
		{"lookup after storage", checkComparisonLookup},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func checkComparisonMissing(t *testing.T, r data.ComparisonReader, _ data.ComparisonWriter) {
	t.Helper()
	_, err := r.GetComparison(context.Background(), SampleComparison(1, "m/1").Ref)
	if !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetComparison unknown: err = %v, want ErrNotFound", err)
	}
}

func checkComparisonRoundTrip(t *testing.T, r data.ComparisonReader, w data.ComparisonWriter) {
	t.Helper()
	ctx := context.Background()
	c := SampleComparison(1, "m/1")
	if err := w.PutComparison(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetComparison(ctx, c.Ref)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, c) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, c)
	}
	// Mutating the returned value must not change what is stored.
	got.Sections[0].Lines[0].Class = data.LineChanged
	again, err := r.GetComparison(ctx, c.Ref)
	if err != nil || again.Sections[0].Lines[0].Class != data.LineUnchanged {
		t.Errorf("stored value changed through a returned copy: %v", err)
	}
}

func checkComparisonWriteOnce(t *testing.T, r data.ComparisonReader, w data.ComparisonWriter) {
	t.Helper()
	ctx := context.Background()
	c := SampleComparison(1, "m/1")
	if err := w.PutComparison(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := w.PutComparison(ctx, c); err != nil {
		t.Errorf("identical put: err = %v, want nil", err)
	}
	other := SampleComparison(1, "m/1")
	other.Sections[0].Lines[1].Class = data.LineChanged
	if err := w.PutComparison(ctx, other); !errors.Is(err, data.ErrConflict) {
		t.Errorf("different put: err = %v, want ErrConflict", err)
	}
	got, err := r.GetComparison(ctx, c.Ref)
	if err != nil || !reflect.DeepEqual(got, c) {
		t.Errorf("after conflict: got %+v, err %v; want the first write", got, err)
	}
}

func checkComparisonInvalid(t *testing.T, _ data.ComparisonReader, w data.ComparisonWriter) {
	t.Helper()
	for name, mut := range map[string]func(*data.Comparison){
		"empty bill":   func(c *data.Comparison) { c.Ref.Bill = data.BillRef{} },
		"empty from":   func(c *data.Comparison) { c.Ref.From = "" },
		"slash in to":  func(c *data.Comparison) { c.Ref.To = "r/h" },
		"empty method": func(c *data.Comparison) { c.Ref.Method = "" },
	} {
		c := SampleComparison(1, "m/1")
		mut(&c)
		if err := w.PutComparison(context.Background(), c); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("%s: err = %v, want ErrInvalid", name, err)
		}
	}
}

func checkComparisonKeys(t *testing.T, r data.ComparisonReader, w data.ComparisonWriter) {
	t.Helper()
	ctx := context.Background()
	a, b := SampleComparison(1, "m/1"), SampleComparison(1, "m/2")
	c := SampleComparison(2, "m/1")
	for _, x := range []data.Comparison{a, b, c} {
		if err := w.PutComparison(ctx, x); err != nil {
			t.Fatal(err)
		}
	}
	for _, x := range []data.Comparison{a, b, c} {
		got, err := r.GetComparison(ctx, x.Ref)
		if err != nil || got.Ref != x.Ref {
			t.Errorf("GetComparison(%s) = %v, %v", x.Ref, got.Ref, err)
		}
	}
	rev := a.Ref
	rev.From, rev.To = rev.To, rev.From
	if _, err := r.GetComparison(ctx, rev); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("reversed pair: err = %v, want ErrNotFound", err)
	}
}

func checkComparisonLookup(t *testing.T, r data.ComparisonReader, w data.ComparisonWriter) {
	t.Helper()
	ctx := context.Background()
	c := SampleComparison(1, "m/1")
	if err := w.PutComparison(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetComparison(ctx, c.Ref)
	if err != nil {
		t.Fatal(err)
	}
	res, err := got.Lookup([]string{"s1"}, 1, 2)
	if err != nil || res.Class != data.LineChanged {
		t.Errorf("Lookup 1-2 = %+v, %v; want changed", res, err)
	}
}
