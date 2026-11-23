package generate

import (
	"slices"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

func TestChangeAnchors(t *testing.T) {
	t.Parallel()
	now := data.Statement{Role: data.RoleNow, Anchors: []string{"s2-l1"}}
	changes := data.Statement{Role: data.RoleChanges, Anchors: []string{"s2-l4", "s2-l5"}}
	again := data.Statement{Role: data.RoleChanges, Anchors: []string{"s2-l5", "s2-l6"}}
	why := data.Statement{Role: data.RoleWhy, Anchors: []string{"s2-l9"}}
	if got := changeAnchors([]data.Statement{now, changes, why, again}); !slices.Equal(got, []string{"s2-l4", "s2-l5", "s2-l6"}) {
		t.Errorf("changes anchors = %v", got)
	}
	if got := changeAnchors([]data.Statement{{Anchors: []string{"s2-l2"}}, now}); !slices.Equal(got, []string{"s2-l2", "s2-l1"}) {
		t.Errorf("fallback anchors = %v", got)
	}
	if got := changeAnchors(nil); got != nil {
		t.Errorf("no statements = %v", got)
	}
}
