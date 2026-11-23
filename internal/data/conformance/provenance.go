package conformance

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// SourceProvenance runs the conformance checks for where an original document
// came from: the SourceInfo a Publisher records beside source.xml, and the
// SourceInfoReader that returns it. factory returns a fresh pair for each
// subtest; both values may be the same adapter.
//
// It checks that the URL and retrieval time of a stored version read back, that
// a retried PutVersion with identical bytes is a no-op that keeps the first
// SourceInfo, and that a version never stored is ErrNotFound.
//
// It cannot check: concurrent first writes racing to record a SourceInfo;
// persistence across reopening an adapter (for example a restarted process);
// a version written with an empty SourceInfo (zero URL and time); that
// SourceInfo stays out of the stored parsed text; that a changed source is
// rejected with ErrConflict (BillTextAndPublisher checks that); or how a
// corrupt or unreadable sidecar maps to an error beyond the not-found case.
func SourceProvenance(t *testing.T, factory func(t *testing.T) (data.Publisher, data.SourceInfoReader)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, pub data.Publisher, info data.SourceInfoReader)
	}{
		{"RoundTrip", checkSourceRoundTrip},
		{"RetryKeepsFirst", checkSourceRetryKeepsFirst},
		{"Unknown", checkSourceUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			pub, info := factory(t)
			c.run(t, pub, info)
		})
	}
}

// sourceRetrievedAt is a fixed UTC instant truncated to seconds, so that
// adapters which store whole seconds still round-trip it exactly.
func sourceRetrievedAt() time.Time {
	return time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
}

func checkSourceRoundTrip(t *testing.T, pub data.Publisher, info data.SourceInfoReader) {
	t.Helper()
	ctx := context.Background()
	v := SampleVersion(1, "ih")
	want := data.SourceInfo{URL: "https://example.test/a.xml", RetrievedAt: sourceRetrievedAt()}
	v.Source = want
	if err := pub.PutVersion(ctx, v, []byte("<bill>one</bill>")); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	got, err := info.GetSourceInfo(ctx, v.Ref)
	if err != nil {
		t.Fatalf("GetSourceInfo: %v", err)
	}
	if got.URL != want.URL || !got.RetrievedAt.Equal(want.RetrievedAt) {
		t.Errorf("GetSourceInfo = %+v, want %+v", got, want)
	}
}

func checkSourceRetryKeepsFirst(t *testing.T, pub data.Publisher, info data.SourceInfoReader) {
	t.Helper()
	ctx := context.Background()
	src := []byte("<bill>one</bill>")
	first := data.SourceInfo{URL: "https://example.test/a.xml", RetrievedAt: sourceRetrievedAt()}
	v := SampleVersion(1, "ih")
	v.Source = first
	if err := pub.PutVersion(ctx, v, src); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	retry := SampleVersion(1, "ih")
	retry.Source = data.SourceInfo{URL: "https://example.test/b.xml", RetrievedAt: first.RetrievedAt.Add(time.Hour)}
	if err := pub.PutVersion(ctx, retry, src); err != nil {
		t.Errorf("PutVersion retry with identical bytes and different source: err = %v, want nil", err)
	}
	got, err := info.GetSourceInfo(ctx, v.Ref)
	if err != nil {
		t.Fatalf("GetSourceInfo after retry: %v", err)
	}
	if got.URL != first.URL || !got.RetrievedAt.Equal(first.RetrievedAt) {
		t.Errorf("GetSourceInfo after retry = %+v, want first source %+v", got, first)
	}
}

func checkSourceUnknown(t *testing.T, _ data.Publisher, info data.SourceInfoReader) {
	t.Helper()
	ref := data.VersionRef{Bill: SampleBill(2), Version: "ih"}
	if _, err := info.GetSourceInfo(context.Background(), ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetSourceInfo unknown: err = %v, want ErrNotFound", err)
	}
}
