package adminapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/easterthebunny/pubcommittee/internal/adminapi"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const rosterPath = "/admin/v1/operators/roster"

func rosterBody(approver, why string, ops ...adminapi.RosterEntry) adminapi.RosterRequest {
	return adminapi.RosterRequest{Reason: why, Operators: ops, Approver: approver}
}

func rosterEntries(e *env) []data.LogEntry {
	var out []data.LogEntry
	for _, l := range e.log() {
		if l.Operator != nil {
			out = append(out, l)
		}
	}
	return out
}

func written(t *testing.T, body []byte) adminapi.RosterResult {
	t.Helper()
	var r adminapi.RosterResult
	if err := json.Unmarshal(body, &r); err != nil {
		t.Fatal(err)
	}
	return r
}

// postRoster posts a roster as the deploy account and requires the call to
// succeed with want entries written and total roster entries in the log.
func postRoster(t *testing.T, e *env, req adminapi.RosterRequest, want, total int) adminapi.RosterResult {
	t.Helper()
	code, body := e.call(principalDeploy, http.MethodPost, rosterPath, req)
	got := written(t, body)
	if code != http.StatusOK || len(got.Written) != want || len(rosterEntries(e)) != total {
		t.Fatalf("roster %+v: status %d, %d written, %d in log: %s", req, code, len(got.Written), len(rosterEntries(e)), body)
	}
	return got
}

func TestRosterBootstrap(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	one := adminapi.RosterEntry{ID: "op-1", Name: "Operator One"}
	two := adminapi.RosterEntry{ID: "op-2", Name: "Operator Two"}
	if !postRoster(t, e, rosterBody("", "first roster", one, two), 2, 2).Bootstrap {
		t.Fatal("the first roster is not labelled bootstrap")
	}
	for _, l := range rosterEntries(e) {
		if !l.Bootstrap || l.Actor != nil || l.Kind != adminapi.KindOperatorAdded {
			t.Fatalf("bootstrap entry wrong: %+v", l)
		}
	}
	postRoster(t, e, rosterBody("", "first roster", one, two), 0, 2)
}

func TestRosterChangeWritesOneEntryEachAndIsIdempotent(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	one := adminapi.RosterEntry{ID: "op-1", Name: "Operator One"}
	two := adminapi.RosterEntry{ID: "op-2", Name: "Operator Two"}
	three := adminapi.RosterEntry{ID: "op-3", Name: "Operator Three"}
	postRoster(t, e, rosterBody("", "first roster", one, two), 2, 2)
	change := rosterBody("op-1", "swap op-2 for op-3", one, three)
	postRoster(t, e, change, 2, 4)
	kinds := map[string]string{}
	for _, l := range rosterEntries(e)[:2] { // newest first
		kinds[l.Operator.ID] = l.Kind
		if l.Actor == nil || l.Actor.ID != "op-1" || l.Actor.Name != "Operator One" || l.Bootstrap {
			t.Fatalf("entry wrong: %+v", l)
		}
		if l.Reason != "swap op-2 for op-3" {
			t.Fatalf("reason %q", l.Reason)
		}
	}
	if kinds["op-3"] != adminapi.KindOperatorAdded || kinds["op-2"] != adminapi.KindOperatorRemoved {
		t.Fatalf("kinds %v", kinds)
	}
	postRoster(t, e, change, 0, 4)
	one.Name = "Operator Uno"
	postRoster(t, e, rosterBody("op-1", "rename", one, three), 1, 5)
	if rosterEntries(e)[0].Kind != adminapi.KindOperatorRenamed {
		t.Fatalf("kind %s", rosterEntries(e)[0].Kind)
	}
}

func TestRosterRefusals(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	one := adminapi.RosterEntry{ID: "op-1", Name: "Operator One"}
	two := adminapi.RosterEntry{ID: "op-2", Name: "Operator Two"}
	if code, body := e.call(principalDeploy, http.MethodPost, rosterPath, rosterBody("", "first", one)); code != http.StatusOK {
		t.Fatalf("bootstrap %d: %s", code, body)
	}
	before := len(rosterEntries(e))
	// An approver not on the prior roster, including the operator being added.
	for _, approver := range []string{"op-2", "nobody", ""} {
		code, _ := e.call(principalDeploy, http.MethodPost, rosterPath, rosterBody(approver, "add op-2", one, two))
		if code != http.StatusForbidden {
			t.Fatalf("approver %q: status %d, want 403", approver, code)
		}
	}
	// No reason, empty roster, duplicate id.
	for _, b := range []adminapi.RosterRequest{
		rosterBody("op-1", "", one, two), rosterBody("op-1", "x"), rosterBody("op-1", "x", one, one),
	} {
		if code, _ := e.call(principalDeploy, http.MethodPost, rosterPath, b); code != http.StatusUnprocessableEntity {
			t.Fatalf("bad body %+v: status %d", b, code)
		}
	}
	if len(rosterEntries(e)) != before {
		t.Fatal("a refused call wrote roster entries")
	}
}

func TestRosterLogNeverHoldsPrincipals(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	e.call(principalDeploy, http.MethodPost, rosterPath, rosterBody("", "first", adminapi.RosterEntry{ID: "op-1", Name: "Operator One"}))
	e.publishV1()
	e.call(principalOp1, http.MethodPost, trackPath+"track", reason("seed"))
	raw, err := json.Marshal(e.log())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{principalOp1, principalOp2, principalDeploy} {
		if bytes.Contains(raw, []byte(p)) || bytes.Contains(raw, []byte(p[5:])) {
			t.Fatalf("the public log holds a principal %q", p)
		}
	}
}
