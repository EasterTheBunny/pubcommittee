package generate_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// A placeholder statement is no answer: invalid, not unverified.
func TestParseOutputRefusesPlaceholders(t *testing.T) {
	t.Parallel()
	for _, text := range []string{"placeholder", "Placeholder.", " TBD ", "N/A", "...", "…", "—", "12"} {
		doc := `{"headline":"","statements":[{"text":` + quote(text) + `,"anchors":[],"role":"now"}]}`
		if _, err := generate.ParseOutput(doc); !errors.Is(err, generate.ErrOutputInvalid) {
			t.Errorf("%q: err = %v, want ErrOutputInvalid", text, err)
		}
	}
	for _, text := range []string{"Repealed.", "Section 2 strikes “2025” and inserts “2030”."} {
		doc := `{"headline":"","statements":[{"text":` + quote(text) + `,"anchors":["s2-l2"],"role":"changes"}]}`
		if _, err := generate.ParseOutput(doc); err != nil {
			t.Errorf("%q: err = %v, want none", text, err)
		}
	}
}

func quote(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		panic(err)
	}
	return string(b)
}
