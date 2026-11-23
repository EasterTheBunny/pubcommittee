package memory_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

// TestGenerated runs the generated-output suite over data.GeneratedStore on
// the memory adapter.
func TestGenerated(t *testing.T) {
	t.Parallel()
	conformance.Generated(t, func(t *testing.T) (data.GeneratedReader, data.GeneratedWriter) {
		t.Helper()
		s := data.NewGeneratedStore(memory.New())
		return s, s
	})
}
