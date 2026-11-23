package fakegen_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adapters/fakegen"
	"github.com/easterthebunny/pubcommittee/internal/adapters/memory"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestGenerator(t *testing.T) {
	t.Parallel()
	conformance.Generator(t, func(*testing.T) data.Generator { return fakegen.New(fakegen.Config{}) })
}

func TestGeneratorPersistedBatches(t *testing.T) {
	t.Parallel()
	files := memory.New()
	submit := fakegen.New(fakegen.Config{Files: files})
	id, err := submit.Submit(t.Context(), []data.GenRequest{{ID: "a", Task: data.TaskBillSummary, Input: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	// A later process has a new Gen over the same files.
	res, done, err := fakegen.New(fakegen.Config{Files: files}).Collect(t.Context(), id)
	if err != nil || !done || len(res) != 1 || res[0].ID != "a" {
		t.Fatalf("Collect = %+v, %v, %v", res, done, err)
	}
}
