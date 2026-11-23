package readapi

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/billxml"
	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/versiondiff"
)

// changedSince says what became of lines from..to of section path of a
// pinned older version in the latest version, from the stored comparison of
// the two. Nil when the version is the latest, no comparison is stored, or
// the lines are not in it; an error only when storage fails.
func (s *Server) changedSince(
	ctx context.Context, res resolved, path []string, from, to int,
) (*apiv1.ChangedSince, error) {
	if s.ports.Comparisons == nil || res.n >= res.latest {
		return nil, nil //nolint:nilnil // no notice is the normal answer for the latest version
	}
	vs, err := s.listVersions(ctx, res.ref)
	if err != nil {
		return nil, err
	}
	c, err := s.ports.Comparisons.GetComparison(ctx, data.ComparisonRef{
		Bill: res.ref, From: vs[res.n-1].Ref.Version, To: vs[res.latest-1].Ref.Version,
		Method: versiondiff.Method(billxml.ParserVersion),
	})
	if errors.Is(err, data.ErrNotFound) {
		return nil, nil //nolint:nilnil // not compared yet: no notice
	}
	if err != nil {
		return nil, fmt.Errorf("get comparison: %w", err)
	}
	r, err := c.Lookup(path, from, to)
	if err != nil {
		return nil, nil //nolint:nilnil,nilerr // lines the comparison does not cover: no notice
	}
	out := &apiv1.ChangedSince{ToN: res.latest, Class: apiv1.ChangedSinceClass(r.Class)}
	if r.New != nil {
		id := sectionID(r.New.Section)
		out.NewSectionId = &id
		out.NewFrom, out.NewTo = &r.New.From, &r.New.To
		addr := versionPath(res.id, res.latest) + "/s/" + id + "/l/" + rangeOf(r.New.From, r.New.To)
		out.NewAddress = &addr
		frag := fragmentOf(r.New.Section, r.New.From, r.New.To)[1:]
		out.NewAnchor = &frag
	}
	return out, nil
}

// rangeOf is "12" for one line, "12-13" for a range.
func rangeOf(from, to int) string {
	if from == to {
		return strconv.Itoa(from)
	}
	return strconv.Itoa(from) + "-" + strconv.Itoa(to)
}
