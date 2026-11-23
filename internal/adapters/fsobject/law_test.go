package fsobject_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestLaw(t *testing.T) {
	t.Parallel()
	conformance.Law(t, func(t *testing.T) (data.LawReader, data.LawWriter) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s
	})
}

func TestCitations(t *testing.T) {
	t.Parallel()
	conformance.Citations(t, func(t *testing.T) (data.CitationReader, data.CitationWriter) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s
	})
}
