package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Chamber is where a bill starts: its versions run through that chamber's
// stages first, then the other chamber's.
type Chamber int

// The two chambers of Congress.
const (
	House Chamber = iota
	Senate
)

// Origin is the chamber a bill starts in, from its type: hr, hres, hjres and
// hconres start in the House; s, sres, sjres and sconres in the Senate.
func Origin(b BillRef) Chamber {
	if strings.HasPrefix(b.Type, "s") {
		return Senate
	}
	return House
}

// stage is one legislative stage: the House codes and the Senate codes that
// mark it, each list in its own order.
type stage struct{ house, senate []string }

func (st stage) of(c Chamber) []string {
	if c == Senate {
		return st.senate
	}
	return st.house
}

// chamberStages are the stages a bill passes through in one chamber, in
// order; a bill goes through them in its origin chamber, then (from the
// second) in the other.
func chamberStages() []stage {
	return []stage{
		{[]string{"ih"}, []string{"is"}},                 // introduced
		{[]string{"ash"}, []string{"sas", "sc"}},         // sponsors added, sponsor change
		{[]string{"oph", "hdh"}, []string{"ops", "hds"}}, // ordered printed, held at desk
		// referral instructions, reference change, referred
		{[]string{"rih", "rch", "rth"}, []string{"ris", "rcs", "rts"}},
		{[]string{"rh"}, []string{"rs"}},                                      // reported
		{[]string{"pch"}, []string{"pcs"}},                                    // placed on calendar
		{[]string{"cdh"}, []string{"cds"}},                                    // committee discharged
		{[]string{"lth", "fph", "iph", "fah"}, []string{"lts", "fps", "ips"}}, // laid on table, failed, postponed
		{[]string{"cph"}, []string{"cps"}},                                    // considered and passed
		{[]string{"eph", "eh"}, []string{"es"}},                               // engrossed
	}
}

// versionOrder is every GPO bill version code (govinfo "Bill versions" help
// page) in legislative stage order, earliest first, for a bill that starts in
// origin; a code's position is its rank. The bill goes through the origin
// chamber's stages, is received and referred in the other chamber and goes
// through its stages, then the chambers exchange amendments (the other
// chamber's engrossed amendment comes first) until it is enrolled. The table
// is the only ordering of versions: post dates are day-only and are never
// used to order.
//
// A code that is not in the table is stored but not listed (PublicVersions),
// and ingest names it in the pull's notes until the table covers it.
func versionOrder(origin Chamber) []string {
	other := House
	if origin == House {
		other = Senate
	}
	pick := func(st stage, c Chamber) []string { return st.of(c) }
	arrive := stage{[]string{"rdh", codeRFH}, []string{"rds", codeRFS}} // received, referred in the other chamber
	amend := stage{[]string{"eah"}, []string{"eas"}}                    // engrossed amendment
	resolve := stage{[]string{"reah"}, []string{"res"}}                 // re-engrossed amendment, resolving differences
	agree := stage{[]string{"ath"}, []string{"ats"}}                    // agreed to
	refAmend := stage{[]string{"rah"}, []string{"ras"}}                 // referred with amendments
	var out []string
	stages := chamberStages()
	for _, st := range stages {
		out = append(out, pick(st, origin)...)
	}
	out = append(out, "pav") // previous action vitiated
	out = append(out, pick(arrive, other)...)
	for _, st := range stages {
		out = append(out, pick(st, other)...)
	}
	for _, st := range []stage{amend, resolve, agree, refAmend} {
		out = append(out, pick(st, other)...)
		out = append(out, pick(st, origin)...)
	}
	out = append(out, pick(arrive, origin)...) // a received origin code, if it ever appears
	return append(out, "pwah", "rhuc", "as", "pap", "pp", "enr", "renr")
}

// VersionRank returns the stage rank of a version code of bill and whether
// the code is in the table. Lower ranks are earlier stages; no two codes
// share a rank. The order depends on the bill's origin chamber.
func VersionRank(bill BillRef, code string) (int, bool) {
	r := slices.Index(versionOrder(Origin(bill)), code)
	return r, r >= 0
}

// IsVersionCode reports whether code is in the stage rank table.
func IsVersionCode(code string) bool { return slices.Contains(versionOrder(House), code) }

// VersionCodes returns every code of the stage rank table for a bill that
// starts in origin, earliest first.
func VersionCodes(origin Chamber) []string { return versionOrder(origin) }

const (
	codeRFH = "rfh" // referred in House
	codeRFS = "rfs" // referred in Senate
)

// IsReferralCode reports whether a version code is a referral version, the
// only kind that can collapse into the version before it.
func IsReferralCode(code string) bool { return code == codeRFH || code == codeRFS }

// ReferralLabel is the name of a referral version as people read it, by code.
// The stored version type is a slug; this is the label of the referral note.
func ReferralLabel(code string) string {
	switch code {
	case codeRFS:
		return "Referred in Senate"
	case codeRFH:
		return "Referred in House"
	default:
		return code
	}
}

// Referral is one referral version whose text is the same as the version it
// collapsed into. Its original XML stays stored; it is not listed.
type Referral struct {
	Code        string    `json:"code"`         // version code of the referral, for example "rfs"
	Into        string    `json:"into"`         // version code it collapsed into, for example "eh"
	Type        string    `json:"type"`         // for example "Referred in Senate"
	PublishedAt time.Time `json:"published_at"` // the referral's own date
}

// ReferralSet is the stored collapse decision of one bill. It is derived
// from the stored comparison of each referral with the version before it, and
// is replaced whenever ingest runs the comparison step.
type ReferralSet struct {
	Bill      BillRef    `json:"bill"`
	Collapsed []Referral `json:"collapsed"`
}

// ReferralsPath is the bucket-relative path of a bill's ReferralSet.
func ReferralsPath(b BillRef) string { return "bills/" + b.String() + "/referrals.json" }

// ParseReferrals decodes a stored ReferralSet.
func ParseReferrals(b []byte) (ReferralSet, error) {
	var s ReferralSet
	if err := json.Unmarshal(b, &s); err != nil {
		return ReferralSet{}, fmt.Errorf("referrals: decode: %w: %w", ErrUnavailable, err)
	}
	return s, nil
}

// MarshalReferrals encodes a ReferralSet with collapsed referrals in code order.
func MarshalReferrals(s ReferralSet) ([]byte, error) {
	s.Collapsed = slices.Clone(s.Collapsed)
	if s.Collapsed == nil {
		s.Collapsed = []Referral{}
	}
	slices.SortFunc(s.Collapsed, func(a, b Referral) int { return strings.Compare(a.Code, b.Code) })
	out, err := json.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("referrals: encode: %w", err)
	}
	return out, nil
}

// PublicVersions turns every stored version of a bill into the version list
// that readers, exports, the change feed, comparisons, and generation use:
// ordered by stage rank only, without codes missing from the table and
// without referrals that collapsed. A collapsed referral whose target is not
// in the list stays a version of its own: unknown is better than wrong. Each
// version a referral collapsed into carries it in Referrals. The unknown
// codes come back in order, for the pull's notes.
func PublicVersions(all []VersionSummary, set ReferralSet) (list []VersionSummary, unknown []string) {
	into, gone := collapseTargets(all, set)
	list = []VersionSummary{}
	if len(all) == 0 {
		return list, nil
	}
	cmp := VersionCodeOrder(all[0].Ref.Bill)
	for _, v := range all {
		if !IsVersionCode(v.Ref.Version) {
			unknown = append(unknown, v.Ref.Version)
			continue
		}
		if gone[v.Ref.Version] {
			continue
		}
		v.Referrals = slices.Clone(into[v.Ref.Version])
		slices.SortFunc(v.Referrals, func(a, b Referral) int { return cmp(a.Code, b.Code) })
		list = append(list, v)
	}
	slices.SortFunc(list, func(a, b VersionSummary) int { return cmp(a.Ref.Version, b.Ref.Version) })
	slices.Sort(unknown)
	return list, unknown
}

// VersionCodeOrder returns the comparison that orders two version codes of
// bill by stage rank, for sorting its versions. Codes missing from the table
// sort last, by code.
func VersionCodeOrder(bill BillRef) func(a, b string) int {
	order := versionOrder(Origin(bill))
	return func(a, b string) int { return compareRanks(order, a, b) }
}

func compareRanks(order []string, a, b string) int {
	ra, rb := slices.Index(order, a), slices.Index(order, b)
	oka, okb := ra >= 0, rb >= 0
	switch {
	case oka && okb:
		return ra - rb
	case oka:
		return -1
	case okb:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// collapseTargets validates a ReferralSet against the stored versions: the
// referrals to hide, and the referrals to attach to the version each
// collapsed into. A collapse naming a missing or unknown code is ignored.
func collapseTargets(all []VersionSummary, set ReferralSet) (into map[string][]Referral, gone map[string]bool) {
	known := map[string]bool{}
	for _, v := range all {
		if IsVersionCode(v.Ref.Version) {
			known[v.Ref.Version] = true
		}
	}
	into, gone = map[string][]Referral{}, map[string]bool{}
	for _, r := range set.Collapsed {
		if known[r.Code] && known[r.Into] && r.Code != r.Into && !gone[r.Into] {
			gone[r.Code] = true
			into[r.Into] = append(into[r.Into], r)
		}
	}
	return into, gone
}

// LoadReferrals reads a bill's ReferralSet through get, the adapter's file
// read; a bill with none gives the empty set.
func LoadReferrals(
	ctx context.Context, get func(context.Context, string) ([]byte, error), bill BillRef,
) (ReferralSet, error) {
	b, err := get(ctx, ReferralsPath(bill))
	if errors.Is(err, ErrNotFound) {
		return ReferralSet{}, nil
	}
	if err != nil {
		return ReferralSet{}, fmt.Errorf("referrals: %w", err)
	}
	return ParseReferrals(b)
}
