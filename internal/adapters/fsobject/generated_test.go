package fsobject_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// TestGenerated runs the generated-output suite over data.GeneratedStore on
// the fsobject adapter.
func TestGenerated(t *testing.T) {
	t.Parallel()
	conformance.Generated(t, func(t *testing.T) (data.GeneratedReader, data.GeneratedWriter) {
		t.Helper()
		s := data.NewGeneratedStore(newStore(t, t.TempDir()))
		return s, s
	})
}
