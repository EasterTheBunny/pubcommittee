package main

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

// analyzeStored runs the citation and comparison steps over the stored
// versions of refs, or of every tracked catalog bill when refs is empty. It
// fetches nothing, so it works over any data directory.
func analyzeStored(
	ctx context.Context, refs []data.BillRef, d ingest.Deps, opts ingest.Options, out io.Writer,
) (int, error) {
	if len(refs) == 0 {
		var err error
		if refs, err = trackedRefs(ctx, d); err != nil {
			return 0, err
		}
	}
	failed, items := false, 0
	for _, ref := range refs {
		rep, err := ingest.Analyze(ctx, d, opts, ref)
		if encErr := encode(out, rep); encErr != nil {
			return items, encErr
		}
		if err != nil {
			d.Log.Error("analyze failed", "bill", ref.ID(), "error", err.Error())
		}
		failed = failed || err != nil || rep.Failed()
		items += rep.CitationSets + rep.Comparisons
	}
	if failed {
		return items, errFailed
	}
	return items, nil
}

// trackedRefs lists the tracked bills of the catalog, ordered by id.
func trackedRefs(ctx context.Context, d ingest.Deps) ([]data.BillRef, error) {
	cat, err := ingest.LoadCatalog(ctx, d.Ports.PublicDocs)
	if err != nil {
		return nil, fmt.Errorf("load catalog: %w", err)
	}
	var refs []data.BillRef
	for ref, e := range cat {
		if e.Tracked {
			refs = append(refs, ref)
		}
	}
	sortRefs(refs)
	return refs, nil
}

func sortRefs(refs []data.BillRef) {
	slices.SortFunc(refs, func(a, b data.BillRef) int { return strings.Compare(a.ID(), b.ID()) })
}
