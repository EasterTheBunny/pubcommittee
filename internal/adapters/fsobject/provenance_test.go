package fsobject_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fsobject"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestSourceProvenance(t *testing.T) {
	t.Parallel()
	conformance.SourceProvenance(t, func(t *testing.T) (data.Publisher, data.SourceInfoReader) {
		t.Helper()
		s, err := fsobject.New(fsobject.Config{Root: t.TempDir()})
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return s, s
	})
}
