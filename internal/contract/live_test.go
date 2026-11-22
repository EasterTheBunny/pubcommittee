package contract_test

import (
	"os"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/contract"
)

// TestLiveRoutes checks every route of a running read-api that was seeded by
// internal/readapi/seed, when CONTRACT_BASE_URL is set. Start the service
// with the memory target and SEED_FIXTURES=1, or on the local target after
// the acceptance seeding step.
func TestLiveRoutes(t *testing.T) {
	t.Parallel()
	base := os.Getenv("CONTRACT_BASE_URL")
	if base == "" {
		t.Skip("CONTRACT_BASE_URL not set")
	}
	h, err := contract.New(base)
	if err != nil {
		t.Fatal(err)
	}
	contract.CheckSeededRoutes(t, h)
}
