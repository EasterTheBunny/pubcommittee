package lawingest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
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

// progressLine is one progress log line of Step.
type progressLine struct {
	Msg      string `json:"msg"`
	Release  string `json:"release"`
	Mode     string `json:"mode"`
	Title    int    `json:"title"`
	Method   string `json:"method"`
	Source   string `json:"source"`
	Sections int    `json:"sections"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Elapsed  int    `json:"elapsed_s"`
}

func TestStepLogsEachTitle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	b, err := os.ReadFile("../uslm/testdata/title3-sample.xml")
	if err != nil {
		t.Fatal(err)
	}
	src := &fakeSource{rps: []data.ReleasePoint{{ID: "119-142", Date: time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)}}, title3: b}
	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	tick := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { tick = tick.Add(30 * time.Second); return tick }
	s := memory.New()
	if _, err := lawingest.Step(ctx, src, s, s, []int{3}, lawingest.WithLogger(log), lawingest.WithClock(clock), lawingest.WithSource("uscode")); err != nil {
		t.Fatal(err)
	}
	want := []progressLine{
		{Msg: "law ingest started", Release: "119-142", Mode: lawingest.ModeSubset, Source: "uscode", Total: 1},
		{
			Msg: "law title stored", Release: "119-142", Title: 3, Method: lawingest.MethodFetch, Source: "uscode",
			Sections: 2, Done: 1, Total: 1, Elapsed: 30,
		},
	}
	dec := json.NewDecoder(&buf)
	for i, w := range want {
		var line progressLine
		if err := dec.Decode(&line); err != nil {
			t.Fatalf("log line %d: %v", i, err)
		}
		if line != w {
			t.Errorf("log line %d = %+v, want %+v", i, line, w)
		}
	}
	if dec.More() {
		t.Errorf("more log lines than %d", len(want))
	}
}
