package readapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"time"

	apiv1 "github.com/easterthebunny/pubcommittee/api/v1"
	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	defaultLimit = 50
	maxLimit     = 200
)

// Server implements apiv1.ServerInterface over the public read ports.
type Server struct {
	ports   data.PublicReadPorts
	started time.Time
	// noticeHref is where discovery points for the machine-access notice;
	// Handler sets it from Options.UIURL.
	noticeHref string
}

var _ apiv1.ServerInterface = (*Server)(nil)

// New returns a Server reading from ports.
func New(ports data.PublicReadPorts) *Server {
	return &Server{ports: ports, started: time.Now().UTC().Truncate(time.Second), noticeHref: NoticePath}
}

// resolved is a bill version named by a request.
type resolved struct {
	id  string
	ref data.BillRef
	// n and latest are 1-based positions in the version list, for indexing
	// vs only; they are never served or used in an address.
	n          int
	latest     int
	code       string // version code of the answer
	latestCode string // version code of the latest version
	vs         []data.VersionSummary
	fromLatest bool
	version    data.OfficialVersion
}

// redirectError says a requested code is a referral that collapsed into
// another version; the answer is a redirect to the same route on that code.
type redirectError struct{ from, to string }

func (e redirectError) Error() string { return "version " + e.from + " collapsed into " + e.to }

// listVersions returns the versions of a bill; a bill with none is ErrNotFound.
func (s *Server) listVersions(ctx context.Context, ref data.BillRef) ([]data.VersionSummary, error) {
	vs, err := s.ports.BillText.ListVersions(ctx, ref)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	if len(vs) == 0 {
		return nil, fmt.Errorf("bill %s has no versions: %w", ref, data.ErrNotFound)
	}
	return vs, nil
}

// resolve finds the version named by code (a version code or "latest") of a
// bill and loads it. A code that is a collapsed referral gives redirectError.
// loadText false skips reading the text and leaves version zero.
func (s *Server) resolve(ctx context.Context, bill, n string, loadText bool) (resolved, error) {
	ref, err := parseBillID(bill)
	if err != nil {
		return resolved{}, err
	}
	vs, err := s.listVersions(ctx, ref)
	if err != nil {
		return resolved{}, err
	}
	out := resolved{id: billID(ref), ref: ref, latest: len(vs), latestCode: vs[len(vs)-1].Ref.Version, vs: vs}
	if n == latestWord {
		out.n, out.fromLatest = len(vs), true
	} else {
		pos := slices.IndexFunc(vs, func(v data.VersionSummary) bool { return v.Ref.Version == n })
		if pos < 0 {
			for _, v := range vs {
				if slices.ContainsFunc(v.Referrals, func(r data.Referral) bool { return r.Code == n }) {
					return resolved{}, redirectError{from: n, to: v.Ref.Version}
				}
			}
			return resolved{}, fmt.Errorf("version %q of %s: %w", n, out.id, data.ErrNotFound)
		}
		out.n = pos + 1
	}
	out.code = vs[out.n-1].Ref.Version
	if loadText {
		if out.version, err = s.ports.BillText.GetVersion(ctx, vs[out.n-1].Ref); err != nil {
			return resolved{}, fmt.Errorf("get version: %w", err)
		}
	}
	return out, nil
}

// class is the cache class of an answer: pinned text is immutable, an
// answer resolved from "latest" can change when a new version arrives.
func (r resolved) class() cacheClass {
	if r.fromLatest {
		return cacheShort
	}
	return cacheImmutable
}

// address builds the AddressInfo of an answer at path (canonical) for a
// request that named version or not.
func (r resolved) address(canonical, fragment string) apiv1.AddressInfo {
	a := apiv1.AddressInfo{
		Canonical:          canonical,
		Page:               new(canonical[len("/v1"):]),
		PinnedVersion:      r.code,
		LatestVersion:      r.latestCode,
		IsLatest:           r.n == r.latest,
		ResolvedFromLatest: r.fromLatest,
	}
	if fragment != "" {
		a.Fragment = new(fragment)
	}
	if r.n != r.latest {
		a.Compare = new(versionPath(r.id, r.code) + "/compare/" + r.latestCode)
	}
	return a
}

// location is the Content-Location of an answer: set only when the request
// used a floating address.
func (r resolved) location(canonical string) string {
	if r.fromLatest {
		return canonical
	}
	return ""
}

func meta(m data.Meta) (v int, asOf time.Time, p apiv1.Provenance, c apiv1.TrustedContentClass) {
	return m.Version, m.AsOf.UTC(), apiv1.Provenance(m.Provenance), apiv1.TrustedContentClass(m.ContentClass)
}

// pageCursor is the cursor of an in-memory list: the offset of the next item.
type pageCursor struct {
	Offset int `json:"o"`
}

// limitOf validates a limit parameter.
func limitOf(p *int) (int, error) {
	if p == nil {
		return defaultLimit, nil
	}
	if *p < 1 || *p > maxLimit {
		return 0, fmt.Errorf("limit %d: %w", *p, data.ErrInvalid)
	}
	return *p, nil
}

func encodeCursor(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", fmt.Errorf("encode cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func decodeCursor(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return fmt.Errorf("cursor: %w", data.ErrInvalid)
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("cursor: %w", data.ErrInvalid)
	}
	return nil
}

// slicePage returns one page of items from a stable in-memory list.
func slicePage[T any](items []T, cursor *string, limit *int) (page []T, next *string, err error) {
	lim, err := limitOf(limit)
	if err != nil {
		return nil, nil, err
	}
	var c pageCursor
	if cursor != nil && *cursor != "" {
		if err := decodeCursor(*cursor, &c); err != nil {
			return nil, nil, err
		}
	}
	if c.Offset < 0 || c.Offset > len(items) {
		return nil, nil, fmt.Errorf("cursor: %w", data.ErrInvalid)
	}
	end := min(c.Offset+lim, len(items))
	page = items[c.Offset:end]
	if end < len(items) {
		tok, err := encodeCursor(pageCursor{Offset: end})
		if err != nil {
			return nil, nil, err
		}
		next = &tok
	}
	return page, next, nil
}

// bindError answers a parameter the generated binder rejected.
func bindError(w http.ResponseWriter, _ *http.Request, _ error) {
	invalid(w, "malformed parameter")
}
