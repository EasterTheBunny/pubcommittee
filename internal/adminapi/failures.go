package adminapi

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/ingest"
)

// states reads the ingest state of every session in the catalog, or of the
// sources named by ?source=<source>/<session>.
func (s *Server) states(r *http.Request) ([]SourceState, error) {
	ctx := r.Context()
	sources := r.URL.Query()["source"]
	if len(sources) == 0 {
		cat, err := ingest.LoadCatalog(ctx, s.Ports.PublicDocs)
		if err != nil {
			return nil, fmt.Errorf("catalog: %w", err)
		}
		seen := map[string]bool{}
		for b := range cat {
			if k := s.Source + "/" + b.Session; !seen[k] {
				seen[k] = true
				sources = append(sources, k)
			}
		}
		slices.Sort(sources)
	}
	out := []SourceState{}
	for _, src := range sources {
		st, err := s.Ports.PlatformDocs.GetIngestState(ctx, src)
		if errors.Is(err, data.ErrNotFound) {
			out = append(out, SourceState{Source: src})
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("ingest state %s: %w", src, err)
		}
		out = append(out, sourceState(st))
	}
	return out, nil
}

func sourceState(st data.IngestState) SourceState {
	o := SourceState{Source: st.Source, FailedCount: len(st.Failed)}
	if !st.Watermark.IsZero() {
		o.Watermark = st.Watermark.UTC().Format(time.RFC3339)
	}
	if !st.Meta.AsOf.IsZero() {
		o.AsOf = st.Meta.AsOf.UTC().Format(time.RFC3339)
	}
	for _, f := range st.Failed {
		o.Failed = append(o.Failed, FailedItem{Item: f.Item, Error: f.Error, At: f.At.UTC().Format(time.RFC3339)})
	}
	return o
}

// failures lists the items the last pulls could not store.
func (s *Server) failures(w http.ResponseWriter, r *http.Request) {
	list, err := s.states(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	writeJSON(w, StateList{Sources: list})
}

// jobs lists the last run of each source. M1 has no scheduled jobs: a source
// has a state only after a manual pull recorded a failure.
func (s *Server) jobs(w http.ResponseWriter, r *http.Request) {
	list, err := s.states(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	for i := range list {
		list[i].Failed = nil
	}
	writeJSON(w, StateList{Sources: list})
}
