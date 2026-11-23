package adminapi

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/httpx"
)

// Roster log entry kinds.
const (
	KindOperatorAdded   = "operator_added"
	KindOperatorRenamed = "operator_renamed"
	KindOperatorRemoved = "operator_removed"
)

const rosterIDPrefix = "roster-"

func isRosterKind(k string) bool {
	return k == KindOperatorAdded || k == KindOperatorRenamed || k == KindOperatorRemoved
}

// rosterFromLog rebuilds the current roster from the roster entries of the
// public log, in the order they were written, and counts them.
func (s *Server) rosterFromLog(r *http.Request) (map[string]string, int, error) {
	var entries []data.LogEntry
	cursor := ""
	for {
		page, err := s.Ports.PublicDocs.ListLog(r.Context(), data.Page{Cursor: cursor})
		if err != nil {
			return nil, 0, fmt.Errorf("list log: %w", err)
		}
		for _, e := range page.Items {
			if isRosterKind(e.Kind) && e.Operator != nil {
				entries = append(entries, e)
			}
		}
		if page.Next == "" {
			break
		}
		cursor = page.Next
	}
	// Ids carry a zero-padded sequence, so the id order is the write order.
	slices.SortFunc(entries, func(a, b data.LogEntry) int { return strings.Compare(a.ID, b.ID) })
	roster := map[string]string{}
	for _, e := range entries {
		if e.Kind == KindOperatorRemoved {
			delete(roster, e.Operator.ID)
			continue
		}
		roster[e.Operator.ID] = e.Operator.Name
	}
	return roster, len(entries), nil
}

// roster applies the public part of the new operator roster, called by the
// environment's deploy service account during a deploy. It writes one log
// entry per change and nothing when nothing changed. Principals never arrive.
func (s *Server) roster(w http.ResponseWriter, r *http.Request) {
	var req RosterRequest
	if !decode(w, r, &req, func() string { return req.Reason }) {
		return
	}
	want, err := validRoster(req.Operators)
	if err != nil {
		s.fail(w, err)
		return
	}
	current, n, err := s.rosterFromLog(r)
	if err != nil {
		s.fail(w, err)
		return
	}
	entries := s.rosterEntries(current, want, req)
	if len(entries) == 0 {
		writeJSON(w, RosterResult{Written: []string{}})
		return
	}
	bootstrap := n == 0
	var approver *data.LogOperator
	if !bootstrap {
		name, ok := current[req.Approver]
		if !ok {
			httpx.WriteProblem(w, http.StatusForbidden, "", "the approver is not on the roster before this change")
			return
		}
		approver = &data.LogOperator{ID: req.Approver, Name: name}
	}
	res := RosterResult{Written: []string{}, Bootstrap: bootstrap}
	for i, e := range entries {
		e.ID = fmt.Sprintf("%s%08d-%s-%s", rosterIDPrefix, n+i+1, e.Kind, e.Operator.ID)
		e.Actor, e.Bootstrap = approver, bootstrap
		if err := s.Ports.Moderation.Record(r.Context(), e); err != nil {
			s.fail(w, fmt.Errorf("roster entry %s: %w", e.ID, err))
			return
		}
		res.Written = append(res.Written, e.ID)
	}
	writeJSON(w, res)
}

func validRoster(in []RosterEntry) (map[string]string, error) {
	if len(in) == 0 {
		return nil, fmt.Errorf("the roster has no operators: %w", data.ErrInvalid)
	}
	out := map[string]string{}
	for _, o := range in {
		if strings.TrimSpace(o.ID) == "" || strings.TrimSpace(o.Name) == "" {
			return nil, fmt.Errorf("every roster entry needs an id and a name: %w", data.ErrInvalid)
		}
		if _, dup := out[o.ID]; dup {
			return nil, fmt.Errorf("duplicate operator id %q: %w", o.ID, data.ErrInvalid)
		}
		out[o.ID] = o.Name
	}
	return out, nil
}

// rosterEntries lists the changes from current to want, ordered by operator
// id with additions and renames before removals, so a partly written call
// resumes from the log on the next one.
func (s *Server) rosterEntries(current, want map[string]string, req RosterRequest) []data.LogEntry {
	var adds, removes []data.LogEntry
	mk := func(kind, id, name string) data.LogEntry {
		return data.LogEntry{
			At: day(s.Now()), Kind: kind, Subject: "operator/" + id, Reason: strings.TrimSpace(req.Reason),
			Operator: &data.LogOperator{ID: id, Name: name}, Meta: s.meta(),
		}
	}
	for _, id := range sortedKeys(want) {
		switch old, ok := current[id]; {
		case !ok:
			adds = append(adds, mk(KindOperatorAdded, id, want[id]))
		case old != want[id]:
			adds = append(adds, mk(KindOperatorRenamed, id, want[id]))
		}
	}
	for _, id := range sortedKeys(current) {
		if _, ok := want[id]; !ok {
			removes = append(removes, mk(KindOperatorRemoved, id, current[id]))
		}
	}
	return append(adds, removes...)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
