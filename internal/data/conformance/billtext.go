package conformance

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"reflect"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// BillTextAndPublisher runs the conformance checks for the BillText and
// Publisher ports. factory returns a fresh pair for each subtest; both values
// may be the same adapter.
func BillTextAndPublisher(t *testing.T, factory func(t *testing.T) (data.BillText, data.Publisher)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, bt data.BillText, pub data.Publisher)
	}{
		{"missing refs", checkMissingRefs},
		{"empty version list", checkEmptyList},
		{"round trip", checkRoundTrip},
		{"idempotent put", checkIdempotentPut},
		{"invalid put", checkInvalidPut},
		{"list order", checkListOrder},
		{"get lines", checkGetLines},
		{"copies", checkCopies},
		{"immutable files", checkImmutable},
		{"replaceable files", checkReplaceable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			bt, pub := factory(t)
			c.run(t, bt, pub)
		})
	}
}

func checkMissingRefs(t *testing.T, bt data.BillText, _ data.Publisher) {
	t.Helper()
	ctx := context.Background()
	ref := data.VersionRef{Bill: SampleBill(1), Version: "ih"}
	if _, err := bt.GetVersion(ctx, ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetVersion unknown: err = %v, want ErrNotFound", err)
	}
	if _, err := bt.GetSource(ctx, ref); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetSource unknown: err = %v, want ErrNotFound", err)
	}
	a := data.Anchor{Version: ref, Section: []string{"s1"}, From: 1, To: 1, SourceID: ""}
	if _, err := bt.GetLines(ctx, a); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetLines unknown: err = %v, want ErrNotFound", err)
	}
}

func checkEmptyList(t *testing.T, bt data.BillText, _ data.Publisher) {
	t.Helper()
	got, err := bt.ListVersions(context.Background(), SampleBill(1))
	if err != nil {
		t.Fatalf("ListVersions of bill with no versions: err = %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("ListVersions of bill with no versions = %v, want empty non-nil list", got)
	}
}

func checkRoundTrip(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	want := SampleVersion(1, "ih")
	src := []byte("<bill>one</bill>")
	if err := pub.PutVersion(ctx, want, src); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	got, err := bt.GetVersion(ctx, want.Ref)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetVersion = %+v, want %+v", got, want)
	}
	gotSrc, err := bt.GetSource(ctx, want.Ref)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	if !bytes.Equal(gotSrc, src) {
		t.Errorf("GetSource = %q, want %q", gotSrc, src)
	}
	list, err := bt.ListVersions(ctx, SampleBill(1))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(list) != 1 || list[0].Ref != want.Ref {
		t.Errorf("ListVersions = %+v, want one entry for %s", list, want.Ref)
	}
}

func checkIdempotentPut(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	src := []byte("<bill>one</bill>")
	if err := pub.PutVersion(ctx, SampleVersion(1, "ih"), src); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	if err := pub.PutVersion(ctx, SampleVersion(1, "ih"), src); err != nil {
		t.Errorf("PutVersion identical content: err = %v, want nil", err)
	}
	changed := SampleVersion(1, "ih")
	changed.Sections[0].Lines[1].Text = "changed"
	if err := pub.PutVersion(ctx, changed, src); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutVersion changed line: err = %v, want ErrConflict", err)
	}
	if err := pub.PutVersion(ctx, SampleVersion(1, "ih"), []byte("other")); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutVersion changed source: err = %v, want ErrConflict", err)
	}
	got, err := bt.GetVersion(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetVersion after conflict: %v", err)
	}
	if want := SampleVersion(1, "ih"); !reflect.DeepEqual(got, want) {
		t.Errorf("stored version changed after conflict: got %+v, want %+v", got, want)
	}
	gotSrc, err := bt.GetSource(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetSource after conflict: %v", err)
	}
	if !bytes.Equal(gotSrc, src) {
		t.Errorf("stored source changed after conflict: got %q, want %q", gotSrc, src)
	}
}

func checkInvalidPut(t *testing.T, _ data.BillText, pub data.Publisher) {
	t.Helper()
	v := SampleVersion(1, "")
	if err := pub.PutVersion(context.Background(), v, []byte("x")); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("PutVersion empty version code: err = %v, want ErrInvalid", err)
	}
}

func checkListOrder(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	early := SampleVersion(1, "ih")
	tie := SampleVersion(1, "eh")
	tie.PublishedAt = early.PublishedAt
	late := SampleVersion(1, "rh")
	late.PublishedAt = early.PublishedAt.AddDate(0, 1, 0)
	for _, v := range []data.OfficialVersion{early, late, tie} {
		if err := pub.PutVersion(ctx, v, []byte("x"+v.Ref.Version)); err != nil {
			t.Fatalf("PutVersion %s: %v", v.Ref, err)
		}
	}
	list, err := bt.ListVersions(ctx, SampleBill(1))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	codes := make([]string, 0, len(list))
	for _, s := range list {
		codes = append(codes, s.Ref.Version)
	}
	want := []string{"ih", "rh", "eh"}
	if !reflect.DeepEqual(codes, want) {
		t.Errorf("ListVersions order = %v, want %v (stage rank only)", codes, want)
	}
	checkUnknownAndReferral(t, bt, pub)
}

// listedCodes returns the version codes ListVersions gives for the sample
// bill, with the summaries.
func listedCodes(t *testing.T, bt data.BillText) ([]string, []data.VersionSummary) {
	t.Helper()
	list, err := bt.ListVersions(context.Background(), SampleBill(1))
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	codes := make([]string, 0, len(list))
	for _, s := range list {
		codes = append(codes, s.Ref.Version)
	}
	return codes, list
}

// checkUnknownAndReferral: a code outside the stage rank table is stored but
// not listed, and a referral named in the bill's referral set is listed as a
// note on the version it collapsed into.
func checkUnknownAndReferral(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	rfs := SampleVersion(1, "rfs")
	for _, v := range []data.OfficialVersion{SampleVersion(1, "zz"), rfs} {
		if err := pub.PutVersion(ctx, v, []byte("x"+v.Ref.Version)); err != nil {
			t.Fatalf("PutVersion %s: %v", v.Ref, err)
		}
	}
	if codes, _ := listedCodes(t, bt); !reflect.DeepEqual(codes, []string{"ih", "rh", "eh", "rfs"}) {
		t.Errorf("ListVersions with an unknown code = %v, want [ih rh eh rfs] (unknown left out)", codes)
	}
	set := data.ReferralSet{Bill: SampleBill(1), Collapsed: []data.Referral{
		{Code: "rfs", Into: "eh", Type: "Referred in Senate", PublishedAt: rfs.PublishedAt},
	}}
	body, err := data.MarshalReferrals(set)
	if err != nil {
		t.Fatalf("MarshalReferrals: %v", err)
	}
	if _, err := pub.PutReplaceable(ctx, data.ReferralsPath(SampleBill(1)), body); err != nil {
		t.Fatalf("PutReplaceable referrals: %v", err)
	}
	checkCollapsedList(t, bt)
	if _, err := bt.GetVersion(ctx, rfs.Ref); err != nil {
		t.Errorf("GetVersion of a collapsed referral: %v (the original stays stored)", err)
	}
}

func checkCollapsedList(t *testing.T, bt data.BillText) {
	t.Helper()
	codes, list := listedCodes(t, bt)
	if !reflect.DeepEqual(codes, []string{"ih", "rh", "eh"}) {
		t.Fatalf("ListVersions with a collapsed referral = %v, want [ih rh eh]", codes)
	}
	if got := list[2].Referrals; len(got) != 1 || got[0].Code != "rfs" || got[0].Type != "Referred in Senate" {
		t.Errorf("eh referrals = %+v, want the collapsed rfs", got)
	}
}

func checkGetLines(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	v := SampleVersion(1, "ih")
	if err := pub.PutVersion(ctx, v, []byte("x")); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	const from, to = 2, 3
	a := data.Anchor{Version: v.Ref, Section: []string{"s1"}, From: from, To: to, SourceID: ""}
	lines, err := bt.GetLines(ctx, a)
	if err != nil {
		t.Fatalf("GetLines %+v: %v", a, err)
	}
	if want := v.Sections[0].Lines[from-1 : to]; !reflect.DeepEqual(lines, want) {
		t.Errorf("GetLines = %+v, want %+v", lines, want)
	}

	const pastFrom, pastTo = 3, 5 // sample section ends at line 4
	otherRef := data.VersionRef{Bill: SampleBill(1), Version: "rh"}
	notFound := map[string]data.Anchor{
		"unknown section": {Version: v.Ref, Section: []string{"s9"}, From: 1, To: 1, SourceID: ""},
		"range past end":  {Version: v.Ref, Section: []string{"s1"}, From: pastFrom, To: pastTo, SourceID: ""},
		"unknown version": {Version: otherRef, Section: []string{"s1"}, From: 1, To: 1, SourceID: ""},
	}
	for name, na := range notFound {
		if _, err := bt.GetLines(ctx, na); !errors.Is(err, data.ErrNotFound) {
			t.Errorf("GetLines %s: err = %v, want ErrNotFound", name, err)
		}
	}

	bad := data.Anchor{Version: v.Ref, Section: []string{"s1"}, From: 0, To: 1, SourceID: ""}
	if err := bad.Validate(); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("test anchor %+v: Validate err = %v, want ErrInvalid", bad, err)
	}
	if _, err := bt.GetLines(ctx, bad); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("GetLines from 0: err = %v, want ErrInvalid", err)
	}
}

func checkCopies(t *testing.T, bt data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	want := SampleVersion(1, "ih")
	src := []byte("<bill>one</bill>")
	if err := pub.PutVersion(ctx, want, src); err != nil {
		t.Fatalf("PutVersion: %v", err)
	}
	// Mutating the inputs after the write must not change the stored data.
	want.Sections[0].Lines[0].Text = "caller mutated"
	src[0] = 'X'

	checkVersionCopy(t, bt)
	checkSourceCopy(t, bt)
	checkLinesCopy(t, bt)
}

// checkVersionCopy mutates a returned version and reads it again.
func checkVersionCopy(t *testing.T, bt data.BillText) {
	t.Helper()
	ctx := context.Background()
	got, err := bt.GetVersion(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetVersion: %v", err)
	}
	got.Sections[0].Path[0] = "zz"
	got.Sections[0].Lines[0].Text = "reader mutated"
	again, err := bt.GetVersion(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetVersion again: %v", err)
	}
	if !reflect.DeepEqual(again, SampleVersion(1, "ih")) {
		t.Errorf("stored version changed through a returned copy: %+v", again)
	}
}

// checkSourceCopy mutates returned source bytes and reads them again.
func checkSourceCopy(t *testing.T, bt data.BillText) {
	t.Helper()
	ctx := context.Background()
	gotSrc, err := bt.GetSource(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetSource: %v", err)
	}
	gotSrc[0] = 'Y'
	againSrc, err := bt.GetSource(ctx, SampleVersion(1, "ih").Ref)
	if err != nil {
		t.Fatalf("GetSource again: %v", err)
	}
	if !bytes.Equal(againSrc, []byte("<bill>one</bill>")) {
		t.Errorf("stored source changed through a returned copy: %q", againSrc)
	}
}

// checkLinesCopy mutates returned lines and reads them again.
func checkLinesCopy(t *testing.T, bt data.BillText) {
	t.Helper()
	ctx := context.Background()
	a := data.Anchor{Version: SampleVersion(1, "ih").Ref, Section: []string{"s1"}, From: 1, To: 1, SourceID: ""}
	lines, err := bt.GetLines(ctx, a)
	if err != nil {
		t.Fatalf("GetLines: %v", err)
	}
	lines[0].Text = "reader mutated"
	lines, err = bt.GetLines(ctx, a)
	if err != nil {
		t.Fatalf("GetLines again: %v", err)
	}
	if want := SampleVersion(1, "ih").Sections[0].Lines[0]; len(lines) != 1 || lines[0] != want {
		t.Errorf("stored line changed through a returned copy: %+v", lines)
	}
}

func checkImmutable(t *testing.T, _ data.BillText, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	const path = "exports/hr1234.txt"
	if err := pub.PutImmutable(ctx, path, []byte("one")); err != nil {
		t.Fatalf("PutImmutable: %v", err)
	}
	if err := pub.PutImmutable(ctx, path, []byte("one")); err != nil {
		t.Errorf("PutImmutable identical bytes: err = %v, want nil", err)
	}
	if err := pub.PutImmutable(ctx, path, []byte("two")); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutImmutable different bytes: err = %v, want ErrConflict", err)
	}
	got, err := pub.GetFile(ctx, path)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if !bytes.Equal(got, []byte("one")) {
		t.Errorf("GetFile = %q, want %q", got, "one")
	}
	if _, err := pub.GetFile(ctx, "exports/missing.txt"); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetFile unknown path: err = %v, want ErrNotFound", err)
	}
	if err := pub.PutImmutable(ctx, "", []byte("x")); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("PutImmutable empty path: err = %v, want ErrInvalid", err)
	}
}

func checkReplaceable(t *testing.T, _ data.BillText, pub data.Publisher) {
	t.Helper()
	checkReplaceEtags(t, pub)
	checkReplaceConflicts(t, pub)
}

// checkReplaceEtags replaces a file and checks the content and ETag of each write.
func checkReplaceEtags(t *testing.T, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	const path = "feeds/us.atom"
	etag1, err := pub.PutReplaceable(ctx, path, []byte("v1"))
	if err != nil {
		t.Fatalf("PutReplaceable: %v", err)
	}
	if want := sha256Hex([]byte("v1")); etag1 != want {
		t.Errorf("etag = %q, want lowercase hex sha256 %q", etag1, want)
	}
	etag2, err := pub.PutReplaceable(ctx, path, []byte("v2"))
	if err != nil {
		t.Fatalf("PutReplaceable replace: %v", err)
	}
	if etag2 == etag1 {
		t.Errorf("etag unchanged after different content: %q", etag2)
	}
	if want := sha256Hex([]byte("v2")); etag2 != want {
		t.Errorf("etag = %q, want %q", etag2, want)
	}
	got, err := pub.GetFile(ctx, path)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}
	if !bytes.Equal(got, []byte("v2")) {
		t.Errorf("GetFile after replace = %q, want %q", got, "v2")
	}
	etag3, err := pub.PutReplaceable(ctx, path, []byte("v1"))
	if err != nil {
		t.Fatalf("PutReplaceable back to v1: %v", err)
	}
	if etag3 != etag1 {
		t.Errorf("etag for same content = %q, want %q", etag3, etag1)
	}
}

// checkReplaceConflicts checks that replaceable and write-once paths do not mix.
func checkReplaceConflicts(t *testing.T, pub data.Publisher) {
	t.Helper()
	ctx := context.Background()
	const immutable = "exports/fixed.txt"
	if err := pub.PutImmutable(ctx, immutable, []byte("x")); err != nil {
		t.Fatalf("PutImmutable: %v", err)
	}
	if _, err := pub.PutReplaceable(ctx, immutable, []byte("y")); !errors.Is(err, data.ErrConflict) {
		t.Errorf("PutReplaceable on immutable path: err = %v, want ErrConflict", err)
	}
	if _, err := pub.PutReplaceable(ctx, "", []byte("x")); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("PutReplaceable empty path: err = %v, want ErrInvalid", err)
	}
}

// sha256Hex returns the lowercase hex SHA-256 of b.
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
