package memory_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestDocs(t *testing.T) {
	t.Parallel()
	conformance.Docs(t, func(t *testing.T) (data.PublicDocs, data.PlatformDocs, data.Moderation) {
		t.Helper()
		s := memory.New()
		return s, s, s
	})
}
