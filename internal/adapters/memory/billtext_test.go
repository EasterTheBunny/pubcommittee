package memory_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestBillTextAndPublisher(t *testing.T) {
	t.Parallel()
	conformance.BillTextAndPublisher(t, func(t *testing.T) (data.BillText, data.Publisher) {
		t.Helper()
		s := memory.New()
		return s, s
	})
}
