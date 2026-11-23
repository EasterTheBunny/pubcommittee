package conformance

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// sampleKey returns the fixed generation key of the conformance outputs.
func sampleKey() data.GenKey {
	return data.GenKey{
		Task: data.TaskBillSummary,
		Version: data.VersionRef{
			Bill:    data.BillRef{Jurisdiction: "us", Session: "119", Type: "hr", Number: 1},
			Version: "ih",
		},
		PromptVersion: "p1",
		Model:         "m1",
	}
}

// sampleOutput returns a ready, unreviewed output with one statement for key.
func sampleOutput(key data.GenKey, headline string) data.GeneratedOutput {
	return data.GeneratedOutput{
		Key:      key,
		Status:   data.GenReady,
		Headline: headline,
		Statements: []data.Statement{{
			Role:    data.RoleChanges,
			Text:    "The bill adds a section.",
			Anchors: []string{data.AnchorID([]string{"s1"}, 1, 2)},
		}},
		ReviewState: data.ReviewUnreviewed,
		TokensIn:    10,
		TokensOut:   20,
		Meta: data.Meta{
			Version:      1,
			Provenance:   data.ProvenanceAuto,
			ContentClass: data.ContentPlatform,
		},
	}
}

// sampleGrade returns a grade of the output with key.
func sampleGrade(key data.GenKey, neutral bool) data.Grade {
	return data.Grade{
		Key:           key,
		GraderModel:   "g1",
		PromptVersion: key.PromptVersion,
		Statements:    []data.StatementGrade{{Index: 0, Label: data.GradeSupported}},
		Neutral:       neutral,
		At:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
	}
}

// sampleBatch returns a batch manifest with one item.
func sampleBatch(id string, done bool) data.GenBatch {
	return data.GenBatch{
		ID:          id,
		Items:       []data.GenBatchItem{{Key: sampleKey(), RequestID: "r-" + id}},
		SubmittedAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Done:        done,
	}
}

// batchIDs returns the IDs of bs in order.
func batchIDs(bs []data.GenBatch) []string {
	ids := make([]string, 0, len(bs))
	for _, b := range bs {
		ids = append(ids, b.ID)
	}
	return ids
}

// Generated runs the conformance checks for GeneratedReader and
// GeneratedWriter. factory returns a fresh pair for each subtest; both
// values may be the same adapter. Not checked: concurrent writers, and the
// layout of stored files (the suite never assumes an object path).
func Generated(t *testing.T, factory func(t *testing.T) (data.GeneratedReader, data.GeneratedWriter)) {
	t.Helper()
	cases := []struct {
		name string
		run  func(t *testing.T, r data.GeneratedReader, w data.GeneratedWriter)
	}{
		{"put get round trip", checkGenPutGet},
		{"put identical is no-op", checkGenPutIdentical},
		{"put different conflicts", checkGenPutDifferent},
		{"put invalid key", checkGenPutInvalidKey},
		{"get unknown", checkGenGetUnknown},
		{"current pointer", checkGenCurrent},
		{"set current not ready", checkGenSetCurrentNotReady},
		{"grade round trip", checkGenGrade},
		{"batch pending", checkGenBatches},
		{"batch invalid id", checkGenBatchInvalid},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			r, w := factory(t)
			c.run(t, r, w)
		})
	}
}

func checkGenPutGet(t *testing.T, r data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	o := sampleOutput(sampleKey(), "Summary")
	if err := w.PutGenerated(ctx, o); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetGenerated(ctx, o.Key)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, o) {
		t.Errorf("round trip:\n got %+v\nwant %+v", got, o)
	}
}

func checkGenPutIdentical(t *testing.T, _ data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	o := sampleOutput(sampleKey(), "Summary")
	if err := w.PutGenerated(ctx, o); err != nil {
		t.Fatal(err)
	}
	if err := w.PutGenerated(ctx, o); err != nil {
		t.Errorf("identical put: err = %v, want nil", err)
	}
}

func checkGenPutDifferent(t *testing.T, r data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	o := sampleOutput(sampleKey(), "Summary")
	if err := w.PutGenerated(ctx, o); err != nil {
		t.Fatal(err)
	}
	other := sampleOutput(sampleKey(), "Other headline")
	if err := w.PutGenerated(ctx, other); !errors.Is(err, data.ErrConflict) {
		t.Errorf("different put: err = %v, want ErrConflict", err)
	}
	got, err := r.GetGenerated(ctx, o.Key)
	if err != nil || !reflect.DeepEqual(got, o) {
		t.Errorf("after conflict: got %+v, err %v; want the first write", got, err)
	}
}

func checkGenPutInvalidKey(t *testing.T, _ data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	o := sampleOutput(sampleKey(), "Summary")
	o.Key.Task = ""
	if err := w.PutGenerated(context.Background(), o); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("empty task: err = %v, want ErrInvalid", err)
	}
}

func checkGenGetUnknown(t *testing.T, r data.GeneratedReader, _ data.GeneratedWriter) {
	t.Helper()
	if _, err := r.GetGenerated(context.Background(), sampleKey()); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetGenerated unknown: err = %v, want ErrNotFound", err)
	}
}

func checkGenCurrent(t *testing.T, r data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	key := sampleKey()
	if _, err := r.GetCurrent(ctx, key.Task, key.Version, key.Target); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetCurrent before SetCurrent: err = %v, want ErrNotFound", err)
	}
	first := sampleOutput(key, "First")
	if err := w.PutGenerated(ctx, first); err != nil {
		t.Fatal(err)
	}
	if err := w.SetCurrent(ctx, first); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetCurrent(ctx, key.Task, key.Version, key.Target)
	if err != nil || !reflect.DeepEqual(got, first) {
		t.Errorf("GetCurrent after SetCurrent: got %+v, err %v; want the first output", got, err)
	}
	nextKey := sampleKey()
	nextKey.PromptVersion = "p2"
	next := sampleOutput(nextKey, "Second")
	if err := w.PutGenerated(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err := w.SetCurrent(ctx, next); err != nil {
		t.Fatal(err)
	}
	got, err = r.GetCurrent(ctx, key.Task, key.Version, key.Target)
	if err != nil || !reflect.DeepEqual(got, next) {
		t.Errorf("GetCurrent after second SetCurrent: got %+v, err %v; want the second output", got, err)
	}
}

func checkGenSetCurrentNotReady(t *testing.T, _ data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	for _, status := range []string{
		data.GenUnverified, data.GenWithheld, data.GenRefusal, data.GenTruncated, data.GenInvalid,
	} {
		o := sampleOutput(sampleKey(), "Summary")
		o.Status = status
		if err := w.SetCurrent(ctx, o); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("SetCurrent status %q: err = %v, want ErrInvalid", status, err)
		}
	}
}

func checkGenGrade(t *testing.T, r data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	key := sampleKey()
	if _, err := r.GetGrade(ctx, key); !errors.Is(err, data.ErrNotFound) {
		t.Errorf("GetGrade unknown: err = %v, want ErrNotFound", err)
	}
	g := sampleGrade(key, false)
	if err := w.PutGrade(ctx, g); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetGrade(ctx, key)
	if err != nil || !reflect.DeepEqual(got, g) {
		t.Errorf("grade round trip: got %+v, err %v; want %+v", got, err, g)
	}
	g.Neutral = true
	if err := w.PutGrade(ctx, g); err != nil {
		t.Fatalf("replace grade: %v", err)
	}
	got, err = r.GetGrade(ctx, key)
	if err != nil || !got.Neutral {
		t.Errorf("after replace: neutral = %v, err %v; want true", got.Neutral, err)
	}
}

func checkGenBatches(t *testing.T, _ data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	ctx := context.Background()
	pending, err := w.PendingBatches(ctx)
	if err != nil || len(pending) != 0 {
		t.Fatalf("PendingBatches on empty store: %d batches, err %v; want none", len(pending), err)
	}
	b1, b2 := sampleBatch("b1", false), sampleBatch("b2", false)
	for _, b := range []data.GenBatch{b1, b2} {
		if err := w.SaveBatch(ctx, b); err != nil {
			t.Fatal(err)
		}
	}
	pending, err = w.PendingBatches(ctx)
	if err != nil || !slices.Equal(batchIDs(pending), []string{"b1", "b2"}) {
		t.Errorf("pending after two saves: ids %v, err %v; want [b1 b2]", batchIDs(pending), err)
	}
	b1.Done = true
	if err := w.SaveBatch(ctx, b1); err != nil {
		t.Fatal(err)
	}
	pending, err = w.PendingBatches(ctx)
	if err != nil || !slices.Equal(batchIDs(pending), []string{"b2"}) {
		t.Errorf("pending after b1 done: ids %v, err %v; want [b2]", batchIDs(pending), err)
	}
}

func checkGenBatchInvalid(t *testing.T, _ data.GeneratedReader, w data.GeneratedWriter) {
	t.Helper()
	if err := w.SaveBatch(context.Background(), data.GenBatch{}); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("empty batch ID: err = %v, want ErrInvalid", err)
	}
}
