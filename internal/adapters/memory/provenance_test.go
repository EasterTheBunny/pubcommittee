package memory_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestSourceProvenance(t *testing.T) {
	t.Parallel()
	conformance.SourceProvenance(t, func(t *testing.T) (data.Publisher, data.SourceInfoReader) {
		t.Helper()
		s := memory.New()
		return s, s
	})
}
