package readapi

import (
	"testing"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

func TestToPolicyStructured(t *testing.T) {
	t.Parallel()
	p, err := selection.V1(time.Date(2026, time.November, 22, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	out := toPolicy(p)
	if out.Mode == nil || string(*out.Mode) != data.PolicyModePilot || out.Scope == nil {
		t.Errorf("mode or scope missing: %+v", out)
	}
	if out.Topics == nil || len(*out.Topics) != 3 || out.MeasureKinds == nil || len(*out.MeasureKinds) != 2 {
		t.Errorf("topics or measure kinds missing: %+v", out)
	}
}

func TestToPolicyRulesOnly(t *testing.T) {
	t.Parallel()
	out := toPolicy(data.SelectionPolicy{Version: 1, Rules: []string{"x"}})
	if out.Mode != nil || out.Topics != nil || out.Scope != nil {
		t.Errorf("rules-only version has structured fields: %+v", out)
	}
}
