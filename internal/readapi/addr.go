package readapi

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

const (
	billIDParts  = 4
	maxLineValue = 1 << 30
	latestWord   = "latest"
	pathPrefix   = "/v1/bills/"
	specPath     = "/v1/openapi.yaml"
)

var (
	billIDPattern  = regexp.MustCompile(`^[a-z]{2}-[0-9]+-[a-z]+-[0-9]+$`)
	sectionPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._]*$`)
	rangePattern   = regexp.MustCompile(`^[0-9]+(-[0-9]+)?$`)
)

// parseBillID turns "us-119-hr-4127" into a BillRef. A malformed id names no
// bill, so it is ErrNotFound.
func parseBillID(id string) (data.BillRef, error) {
	if !billIDPattern.MatchString(id) {
		return data.BillRef{}, fmt.Errorf("bill id: %w", data.ErrNotFound)
	}
	parts := strings.Split(id, "-")
	if len(parts) != billIDParts {
		return data.BillRef{}, fmt.Errorf("bill id: %w", data.ErrNotFound)
	}
	num, err := strconv.Atoi(parts[3])
	if err != nil || num < 1 {
		return data.BillRef{}, fmt.Errorf("bill number: %w", data.ErrNotFound)
	}
	return data.BillRef{Jurisdiction: parts[0], Session: parts[1], Type: parts[2], Number: num}, nil
}

// billID is the inverse of parseBillID.
func billID(b data.BillRef) string {
	return fmt.Sprintf("%s-%s-%s-%d", b.Jurisdiction, b.Session, b.Type, b.Number)
}

// parseSection turns "s4.b" into the section path ["s4", "b"].
func parseSection(id string) ([]string, error) {
	if !sectionPattern.MatchString(id) {
		return nil, fmt.Errorf("section id: %w", data.ErrNotFound)
	}
	return strings.Split(id, "."), nil
}

// sectionID is the dotted form of a section path.
func sectionID(path []string) string { return strings.Join(path, ".") }

// parseRange turns "12" or "12-13" into an inclusive range; anything else
// is ErrInvalid.
func parseRange(s string) (from, to int, err error) {
	if !rangePattern.MatchString(s) {
		return 0, 0, fmt.Errorf("range %q: %w", s, data.ErrInvalid)
	}
	lo, hi, found := strings.Cut(s, "-")
	if from, err = strconv.Atoi(lo); err != nil {
		return 0, 0, fmt.Errorf("range %q: %w", s, data.ErrInvalid)
	}
	to = from
	if found {
		if to, err = strconv.Atoi(hi); err != nil {
			return 0, 0, fmt.Errorf("range %q: %w", s, data.ErrInvalid)
		}
	}
	if from < 1 || to < from || to > maxLineValue {
		return 0, 0, fmt.Errorf("range %q: %w", s, data.ErrInvalid)
	}
	return from, to, nil
}

// billPath is the API path of a bill.
func billPath(id string) string { return pathPrefix + id }

// versionPath is the API path of the version with the given code of a bill.
func versionPath(id, code string) string { return billPath(id) + "/v/" + code }

// anchorOf is the fragment form of a line, for example "s4-l12".
func anchorOf(path []string, n int) string { return sectionID(path) + "-l" + strconv.Itoa(n) }

// fragmentOf is the fragment of a line or range, for example "#s4-l12-13".
func fragmentOf(path []string, from, to int) string {
	f := "#" + anchorOf(path, from)
	if to != from {
		f += "-" + strconv.Itoa(to)
	}
	return f
}
