package fsobject_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func newStore(t *testing.T, root string) *fsobject.Store {
	t.Helper()
	s, err := fsobject.New(fsobject.Config{Root: root})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s
}

func TestBillTextAndPublisher(t *testing.T) {
	t.Parallel()
	conformance.BillTextAndPublisher(t, func(t *testing.T) (data.BillText, data.Publisher) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s
	})
}

func TestNewRejectsBadRoot(t *testing.T) {
	t.Parallel()
	if _, err := fsobject.New(fsobject.Config{}); !errors.Is(err, data.ErrInvalid) {
		t.Errorf("empty root: err = %v, want ErrInvalid", err)
	}
	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := fsobject.New(fsobject.Config{Root: file}); err == nil {
		t.Error("root that is a file: want error")
	}
}

func TestRejectsEscapingPaths(t *testing.T) {
	t.Parallel()
	s := newStore(t, t.TempDir())
	for _, p := range []string{"../x", "a/../../x", "/abs", "a//b", "x.meta.json", ".locks/a"} {
		if err := s.PutImmutable(context.Background(), p, []byte("x")); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("PutImmutable(%q): err = %v, want ErrInvalid", p, err)
		}
		if _, err := s.GetFile(context.Background(), p); !errors.Is(err, data.ErrInvalid) {
			t.Errorf("GetFile(%q): err = %v, want ErrInvalid", p, err)
		}
	}
}

func TestSidecarAndETag(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	s := newStore(t, root)
	ctx := context.Background()
	etag, err := s.PutReplaceable(ctx, "feeds/us.atom", []byte("v1"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.ETag(ctx, "feeds/us.atom")
	if err != nil || got != etag {
		t.Errorf("ETag = %q, %v; want %q", got, err, etag)
	}
	if _, err := os.Stat(filepath.Join(root, "feeds", "us.atom.meta.json")); err != nil {
		t.Errorf("sidecar missing: %v", err)
	}
}

func TestReplaceableIfStaleWriter(t *testing.T) {
	t.Parallel()
	s := newStore(t, t.TempDir())
	ctx := context.Background()
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("1"), ""); err != nil {
		t.Fatalf("create with empty ifMatch: %v", err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("2"), ""); !errors.Is(err, data.ErrConflict) {
		t.Errorf("stale create: err = %v, want ErrConflict", err)
	}
	etag, err := s.ETag(ctx, "feeds/a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("2"), etag); err != nil {
		t.Errorf("fresh writer: %v", err)
	}
	if _, err := s.PutReplaceableIf(ctx, "feeds/a", []byte("3"), etag); !errors.Is(err, data.ErrConflict) {
		t.Errorf("stale writer: err = %v, want ErrConflict", err)
	}
}

// TestHelperProcess is a child process, not a test: it creates one
// write-once path in FSOBJECT_ROOT and exits 0 on success, 3 on ErrConflict.
func TestHelperProcess(t *testing.T) {
	t.Parallel()
	root := os.Getenv("FSOBJECT_ROOT")
	if root == "" {
		t.Skip("helper process only")
	}
	s := newStore(t, root)
	err := s.PutImmutable(context.Background(), "exports/race.txt", []byte(os.Getenv("FSOBJECT_BODY")))
	switch {
	case err == nil:
		os.Exit(0)
	case errors.Is(err, data.ErrConflict):
		os.Exit(3)
	default:
		os.Exit(1)
	}
}

func TestConcurrentProcessesOneWinner(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	const procs = 8
	cmds := make([]*exec.Cmd, procs)
	for i := range cmds {
		cmds[i] = exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestHelperProcess$") //nolint:gosec // re-executes this test binary
		cmds[i].Env = append(os.Environ(), "FSOBJECT_ROOT="+root, "FSOBJECT_BODY="+string(rune('a'+i)))
		if err := cmds[i].Start(); err != nil {
			t.Fatal(err)
		}
	}
	wins, conflicts := 0, 0
	for _, c := range cmds {
		err := c.Wait()
		var ee *exec.ExitError
		switch {
		case err == nil:
			wins++
		case errors.As(err, &ee) && ee.ExitCode() == 3:
			conflicts++
		default:
			t.Fatalf("child failed: %v", err)
		}
	}
	if wins != 1 || conflicts != procs-1 {
		t.Errorf("wins = %d, conflicts = %d; want 1 and %d", wins, conflicts, procs-1)
	}
}

func TestWriterVisibleToSecondStore(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	w, r := newStore(t, root), newStore(t, root)
	ctx := context.Background()
	if err := w.PutImmutable(ctx, "exports/x.txt", []byte("hi")); err != nil {
		t.Fatal(err)
	}
	got, err := r.GetFile(ctx, "exports/x.txt")
	if err != nil || string(got) != "hi" {
		t.Errorf("reader got %q, %v", got, err)
	}
}
