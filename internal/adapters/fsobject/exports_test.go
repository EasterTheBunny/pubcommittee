package fsobject_test

import (
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/data/conformance"
)

func TestExports(t *testing.T) {
	t.Parallel()
	conformance.Exports(t, func(t *testing.T) (data.ExportReader, data.ExportWriter) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s
	})
}

func TestFeed(t *testing.T) {
	t.Parallel()
	conformance.Feed(t, func(t *testing.T) (data.FeedReader, data.FeedWriter) {
		t.Helper()
		s := newStore(t, t.TempDir())
		return s, s
	})
}
