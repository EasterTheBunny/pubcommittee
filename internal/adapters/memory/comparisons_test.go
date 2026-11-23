package memory_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestComparisons(t *testing.T) {
	t.Parallel()
	conformance.Comparisons(t, func(t *testing.T) (data.ComparisonReader, data.ComparisonWriter) {
		t.Helper()
		s := memory.New()
		return s, s
	})
}
