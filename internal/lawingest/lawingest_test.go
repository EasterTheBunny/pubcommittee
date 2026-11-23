package lawingest_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/lawingest"
)

type fakeSource struct {
	rps    []data.ReleasePoint
	title3 []byte
	fetchs int
}

func (f *fakeSource) ListReleasePoints(context.Context) ([]data.ReleasePoint, error) {
	return f.rps, nil
}

func (f *fakeSource) FetchTitle(_ context.Context, _ data.ReleasePoint, title int) ([]byte, error) {
	f.fetchs++
	if title != 3 {
		return nil, data.ErrNotFound
	}
	return f.title3, nil
}

func TestStep(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b, err := os.ReadFile("../uslm/testdata/title3-sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	date := time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)
	src := &fakeSource{rps: []data.ReleasePoint{{ID: "119-142", Date: date}, {ID: "119-141"}}, title3: b}
	s := memory.New()

	if _, err := lawingest.Step(ctx, src, s, s, nil); !errors.Is(err, data.ErrInvalid) {
		t.Fatalf("new release without titles: err = %v, want ErrInvalid", err)
	}
	checkFirstRun(ctx, t, src, s, date)
	checkRepeat(ctx, t, src, s)
	sec, err := s.GetLawSection(ctx, data.ProvisionRef{Release: "119-142", Title: 3, Section: "2"})
	if err != nil || sec.Status != "repealed" || !sec.Meta.AsOf.Equal(date) {
		t.Fatalf("section 2 = %+v, %v", sec, err)
	}
}

func checkFirstRun(ctx context.Context, t *testing.T, src *fakeSource, s *memory.Store, date time.Time) {
	t.Helper()
	res, err := lawingest.Step(ctx, src, s, s, []int{3})
	if err != nil || !res.New || res.Sections != 2 {
		t.Fatalf("first run = %+v, %v", res, err)
	}
	cur, err := s.CurrentRelease(ctx)
	if err != nil || cur.ID != "119-142" || !cur.Date.Equal(date) {
		t.Fatalf("current = %+v, %v", cur, err)
	}
}

func checkRepeat(ctx context.Context, t *testing.T, src *fakeSource, s *memory.Store) {
	t.Helper()
	// A weekly run with no new release point fetches nothing and changes nothing.
	src.fetchs = 0
	res, err := lawingest.Step(ctx, src, s, s, nil)
	if err != nil || res.New || res.Sections != 0 || src.fetchs != 0 {
		t.Fatalf("repeat run = %+v, %v, fetches %d", res, err, src.fetchs)
	}
	// Running the same titles again confirms the stored sections.
	if res, err = lawingest.Step(ctx, src, s, s, []int{3}); err != nil || res.Sections != 2 {
		t.Fatalf("rerun with titles = %+v, %v", res, err)
	}
	if _, err := lawingest.Step(ctx, src, s, s, []int{4}); !errors.Is(err, data.ErrNotFound) {
		t.Fatalf("missing title: err = %v, want ErrNotFound", err)
	}
}
