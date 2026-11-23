package readapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

// addPolicyFields copies the structured selection policy fields. A version
// stored with rules only (no mode) leaves them all absent.
func addPolicyFields(out *apiv1.SelectionPolicy, p data.SelectionPolicy) {
	if p.Mode == "" {
		return
	}
	out.Mode = new(apiv1.SelectionPolicyMode(p.Mode))
	out.Phase = new(apiv1.SelectionPolicyPhase(p.Phase))
	out.FollowCompanions = new(p.FollowCompanions)
	out.Scope = new(selection.Scope(p))
	out.MeasureKinds = enumPtr[apiv1.SelectionPolicyMeasureKinds](p.MeasureKinds)
	out.ExcludeKinds = enumPtr[apiv1.SelectionPolicyExcludeKinds](p.ExcludeKinds)
	out.CommitteeActivity = enumPtr[apiv1.SelectionPolicyCommitteeActivity](p.CommitteeActivity)
	topics := make(map[string]apiv1.PolicyTopic, len(p.Topics))
	for name, t := range p.Topics {
		topics[name] = apiv1.PolicyTopic{
			PolicyAreas: strsPtr(t.PolicyAreas), Subjects: strsPtr(t.Subjects), TitleTerms: strsPtr(t.TitleTerms),
		}
	}
	out.Topics = &topics
}

func enumPtr[T ~string](in []string) *[]T {
	out := make([]T, 0, len(in))
	for _, s := range in {
		out = append(out, T(s))
	}
	return &out
}

// addTracking copies the policy version and since date of a tracked entry.
func addTracking(out *apiv1.CatalogEntry, e data.CatalogEntry) {
	if !e.Tracked {
		return
	}
	if e.PolicyVersion > 0 {
		out.PolicyVersion = new(e.PolicyVersion)
	}
	if t, err := time.Parse("2006-01-02", e.TrackedSince); err == nil {
		out.TrackedSince = &openapi_types.Date{Time: t}
	}
}
