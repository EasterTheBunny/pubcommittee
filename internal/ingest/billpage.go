package ingest

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// BillPageLabelDerived labels a bill-page link built from the bill id rather
// than taken from the source record, so readers and machines can tell the two
// apart without a contract change.
const BillPageLabelDerived = "Congress.gov (derived from the bill number)"

// billPageSlug maps a US bill type to the bill-page path segment of
// congress.gov; "" for a type the catalog does not hold.
func billPageSlug(billType string) string {
	switch billType {
	case "hr":
		return "house-bill"
	case "s":
		return "senate-bill"
	case "hjres":
		return "house-joint-resolution"
	case "sjres":
		return "senate-joint-resolution"
	case "hconres":
		return "house-concurrent-resolution"
	case "sconres":
		return "senate-concurrent-resolution"
	case "hres":
		return "house-resolution"
	case "sres":
		return "senate-resolution"
	}
	return ""
}

// DerivedBillPageURL builds the legislature's bill page from the bill id. Only
// the "us" jurisdiction has a rule; other jurisdictions, unknown bill types and
// malformed sessions give "".
func DerivedBillPageURL(ref data.BillRef) string {
	if ref.Jurisdiction != "us" {
		return ""
	}
	slug := billPageSlug(ref.Type)
	n, err := strconv.Atoi(ref.Session)
	if slug == "" || err != nil || n < 1 || ref.Number < 1 {
		return ""
	}
	return fmt.Sprintf("https://www.congress.gov/bill/%s-congress/%s/%d", ordinal(n), slug, ref.Number)
}

// ordinal writes n with its English suffix: 1st, 2nd, 3rd, 11th, 21st, 112th.
func ordinal(n int) string {
	const (
		hundred   = 100
		ten       = 10
		firstTeen = 11
		lastTeen  = 13
	)
	suffixes := [ten]string{"th", "st", "nd", "rd", "th", "th", "th", "th", "th", "th"}
	if r := n % hundred; r >= firstTeen && r <= lastTeen {
		return strconv.Itoa(n) + "th"
	}
	return strconv.Itoa(n) + suffixes[n%ten]
}

// withBillPage returns links with a derived bill-page link added when none is
// present and the bill id has a rule. The derived link comes first, as an
// official one does. links is never modified.
func withBillPage(ref data.BillRef, links []data.Link) []data.Link {
	if slices.ContainsFunc(links, func(l data.Link) bool { return l.Kind == data.LinkBillPage }) {
		return links
	}
	u := DerivedBillPageURL(ref)
	if u == "" {
		return links
	}
	out := make([]data.Link, 0, len(links)+1)
	out = append(out, data.Link{Label: BillPageLabelDerived, URL: u, Kind: data.LinkBillPage})
	return append(out, links...)
}
