package contract_test

import (
	"net/http"
	"os"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/contract"
)

// TestLiveDiscovery checks a running read-api when CONTRACT_BASE_URL is set.
// M1-10 extends it to every route against a service seeded with fixtures.
func TestLiveDiscovery(t *testing.T) {
	t.Parallel()
	base := os.Getenv("CONTRACT_BASE_URL")
	if base == "" {
		t.Skip("CONTRACT_BASE_URL not set")
	}
	h, err := contract.New(base)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/v1", "/v1/openapi.yaml"} {
		if res := h.MustGet(t, path, nil); res.Status != http.StatusOK {
			t.Errorf("%s: status %d", path, res.Status)
		}
	}
}
