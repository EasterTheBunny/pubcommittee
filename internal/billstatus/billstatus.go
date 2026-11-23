// Package billstatus parses one govinfo BILLSTATUS XML document into a
// data.BillRecord. Only the fields the catalog keeps are decoded; the rest of
// the document, including the dc: Dublin Core block, is ignored.
package billstatus

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// jurisdiction is the Jurisdiction of every bill in a BILLSTATUS document.
const jurisdiction = "us"

// Values of <originChamberCode>.
const (
	originHouse  = "H"
	originSenate = "S"
)

// versionExt is the file suffix of the official XML text versions.
const versionExt = ".xml"

// document is the root of a BILLSTATUS file.
type document struct {
	XMLName xml.Name `xml:"billStatus"`
	Bill    bill     `xml:"bill"`
}

// bill holds the direct children of <bill> that the catalog keeps.
type bill struct {
	Number         string        `xml:"number"`
	Congress       string        `xml:"congress"`
	Type           string        `xml:"type"`
	OriginCode     string        `xml:"originChamberCode"`
	Title          string        `xml:"title"`
	IntroducedDate string        `xml:"introducedDate"`
	UpdateDate     string        `xml:"updateDate"`
	LegislationURL string        `xml:"legislationUrl"`
	Sponsors       []person      `xml:"sponsors>item"`
	LatestAction   action        `xml:"latestAction"`
	PolicyArea     string        `xml:"policyArea>name"`
	Subjects       []named       `xml:"subjects>legislativeSubjects>item"`
	Committees     []committee   `xml:"committees>item"`
	RelatedBills   []relatedItem `xml:"relatedBills>item"`
	TextVersions   []textVersion `xml:"textVersions>item"`
}

// person is a sponsor entry.
type person struct {
	BioguideID string `xml:"bioguideId"`
	FullName   string `xml:"fullName"`
	Party      string `xml:"party"`
}

// action is a date and text pair, as in <latestAction>.
type action struct {
	Date string `xml:"actionDate"`
	Text string `xml:"text"`
}

// named is an entry that carries only a name.
type named struct {
	Name string `xml:"name"`
}

// committee is one entry of <committees>, with its activities.
type committee struct {
	Name       string     `xml:"name"`
	Chamber    string     `xml:"chamber"`
	Activities []activity `xml:"activities>item"`
}

// activity is one step of a committee on the bill.
type activity struct {
	Name string `xml:"name"`
	Date string `xml:"date"`
}

// relatedItem is one entry of <relatedBills>.
type relatedItem struct {
	Title         string         `xml:"title"`
	Congress      string         `xml:"congress"`
	Number        string         `xml:"number"`
	Type          string         `xml:"type"`
	Relationships []relationship `xml:"relationshipDetails>item"`
}

// relationship is one entry of <relationshipDetails>.
type relationship struct {
	Type string `xml:"type"`
}

// textVersion is one entry of <textVersions>.
type textVersion struct {
	Type    string   `xml:"type"`
	Date    string   `xml:"date"`
	Formats []format `xml:"formats>item"`
}

// format is one file of a text version.
type format struct {
	URL string `xml:"url"`
}

// Parse maps one BILLSTATUS document to a BillRecord. src records where the
// bytes came from and is copied to BillRecord.Source. Malformed XML or a
// missing congress, type or number: error wrapping data.ErrInvalid.
func Parse(xmlBytes []byte, src data.SourceInfo) (data.BillRecord, error) {
	var doc document
	if err := xml.Unmarshal(xmlBytes, &doc); err != nil {
		return data.BillRecord{}, fmt.Errorf("%w: decode billstatus: %w", data.ErrInvalid, err)
	}
	b := doc.Bill
	number, err := strconv.Atoi(trim(b.Number))
	if err != nil {
		return data.BillRecord{}, fmt.Errorf("%w: bill number %q: %w", data.ErrInvalid, b.Number, err)
	}
	ref := billRef(b.Congress, b.Type, number)
	if err := ref.Validate(); err != nil {
		return data.BillRecord{}, fmt.Errorf("billstatus: %w", err)
	}
	rec := data.BillRecord{
		Bill:              ref,
		Title:             trim(b.Title),
		Chamber:           chamberOf(b.OriginCode),
		Introduced:        trim(b.IntroducedDate),
		UpdatedAt:         parseTime(b.UpdateDate),
		LatestAction:      data.Action{Date: trim(b.LatestAction.Date), Text: trim(b.LatestAction.Text)},
		PolicyArea:        trim(b.PolicyArea),
		Subjects:          subjects(b.Subjects),
		CommitteeActivity: committeeActivity(b.Committees),
		Related:           related(b.RelatedBills),
		Versions:          versions(ref, b.TextVersions),
		BillPageURL:       trim(b.LegislationURL),
		Source:            src,
	}
	if len(b.Sponsors) > 0 {
		rec.SponsorID = trim(b.Sponsors[0].BioguideID)
		rec.Sponsor = trim(b.Sponsors[0].FullName)
		rec.SponsorParty = trim(b.Sponsors[0].Party)
	}
	return rec, nil
}

// billRef builds a BillRef from the congress, type and number of a bill.
func billRef(congress, billType string, number int) data.BillRef {
	return data.BillRef{
		Jurisdiction: jurisdiction,
		Session:      trim(congress),
		Type:         strings.ToLower(trim(billType)),
		Number:       number,
	}
}

// chamberOf maps <originChamberCode> to a data.Chamber value, or "" when the
// code is neither H nor S.
func chamberOf(code string) string {
	switch trim(code) {
	case originHouse:
		return data.ChamberHouse
	case originSenate:
		return data.ChamberSenate
	default:
		return ""
	}
}

// parseTime reads an RFC3339 timestamp; an empty or unparsable value gives
// the zero time.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, trim(s))
	if err != nil {
		return time.Time{}
	}
	return t
}

// trim removes surrounding white space from a value read from the document.
func trim(s string) string {
	return strings.TrimSpace(s)
}

// subjects returns the legislative subject names in document order.
func subjects(items []named) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, trim(it.Name))
	}
	return out
}

// committeeActivity returns one entry for each activity of each committee,
// in document order.
func committeeActivity(committees []committee) []data.CommitteeActivity {
	var out []data.CommitteeActivity
	for _, c := range committees {
		for _, a := range c.Activities {
			out = append(out, data.CommitteeActivity{
				Committee: trim(c.Name),
				Chamber:   trim(c.Chamber),
				Activity:  trim(a.Name),
				At:        parseTime(a.Date),
			})
		}
	}
	return out
}

// related returns the related bills whose number is a positive integer. The
// relationship is the first relationship type of the item.
func related(items []relatedItem) []data.RelatedBill {
	var out []data.RelatedBill
	for _, it := range items {
		n, err := strconv.Atoi(trim(it.Number))
		if err != nil || n <= 0 {
			continue
		}
		rb := data.RelatedBill{Bill: billRef(it.Congress, it.Type, n), Title: trim(it.Title)}
		if len(it.Relationships) > 0 {
			rb.Relationship = trim(it.Relationships[0].Type)
		}
		out = append(out, rb)
	}
	return out
}

// versions returns the official XML text versions of bill, oldest first. A
// repeated version code keeps its first entry in document order.
func versions(bill data.BillRef, items []textVersion) []data.SourceVersion {
	var out []data.SourceVersion
	seen := make(map[string]bool)
	for _, it := range items {
		u := xmlURL(it.Formats)
		if u == "" {
			continue
		}
		code := versionCode(u, bill)
		if code == "" || strings.Contains(code, "/") || seen[code] {
			continue
		}
		seen[code] = true
		out = append(out, data.SourceVersion{
			Ref:         data.VersionRef{Bill: bill, Version: code},
			Type:        trim(it.Type),
			PublishedAt: parseTime(it.Date),
			URL:         u,
		})
	}
	slices.SortStableFunc(out, func(a, b data.SourceVersion) int {
		return cmp.Or(a.PublishedAt.Compare(b.PublishedAt), strings.Compare(a.Ref.Version, b.Ref.Version))
	})
	return out
}

// xmlURL returns the first format URL that ends in ".xml", or "" when none does.
func xmlURL(formats []format) string {
	for _, f := range formats {
		if u := trim(f.URL); strings.HasSuffix(u, versionExt) {
			return u
		}
	}
	return ""
}

// versionCode derives the version code from a text version URL: the file
// name without ".xml" and without the "BILLS-<congress><type><number>"
// prefix, lower-cased. For example BILLS-119hr4313rfs.xml gives "rfs".
func versionCode(u string, bill data.BillRef) string {
	name := strings.TrimSuffix(path.Base(u), versionExt)
	prefix := "BILLS-" + bill.Session + bill.Type + strconv.Itoa(bill.Number)
	return strings.ToLower(strings.TrimPrefix(name, prefix))
}
