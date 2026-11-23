package adminapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
	"github.com/easterthebunny/pubcommittee/internal/selection"
)

var errGenerator = errors.New("generation is unavailable: no Generator is wired until M1-09")

// policyStore joins the two ports selection.PublishAs needs.
type policyStore struct {
	data.PublicDocs
	data.PlatformDocs
}

// publishPolicy stores the next immutable selection policy version with its
// public log entry, which names the operator. The version must be the latest
// plus one; the latest again is an idempotent no-op.
func (s *Server) publishPolicy(w http.ResponseWriter, r *http.Request) {
	var req PolicyRequest
	if !decode(w, r, &req, func() string { return req.Reason }) {
		return
	}
	ctx := r.Context()
	p, err := selection.Parse(req.Policy, s.Now())
	if err != nil {
		s.fail(w, err)
		return
	}
	latest, err := s.Ports.PublicDocs.GetSelectionPolicy(ctx)
	switch {
	case err == nil && p.Version != latest.Version && p.Version != latest.Version+1:
		s.fail(w, fmt.Errorf("the next policy version is %d, not %d: %w", latest.Version+1, p.Version, data.ErrConflict))
		return
	case errors.Is(err, data.ErrNotFound):
		latest = data.SelectionPolicy{}
	case err != nil:
		s.fail(w, err)
		return
	}
	store := policyStore{s.Ports.PublicDocs, s.Ports.PlatformDocs}
	if err := selection.PublishAs(ctx, store, p, req.Reason, s.Now(), operatorOf(ctx).logActor()); err != nil {
		s.fail(w, err)
		return
	}
	res := PolicyResult{Version: p.Version, Changed: p.Version != latest.Version}
	if res.Changed {
		res.LogID = fmt.Sprintf("selection-policy-%d", p.Version)
	}
	writeJSON(w, res)
}

// generate would submit a generation task under a new key. It reports
// unavailable until a Generator exists (M1-09).
func (s *Server) generate(w http.ResponseWriter, r *http.Request) {
	var req GenerationRequest
	if !decode(w, r, &req, func() string { return req.Reason }) {
		return
	}
	if _, err := data.ParseBillID(req.Bill); err != nil || req.Task == "" || req.Version < 1 {
		s.fail(w, fmt.Errorf("task, bill and version are required: %w", data.ErrInvalid))
		return
	}
	httpx.WriteProblem(w, http.StatusNotImplemented, "", errGenerator.Error())
}
