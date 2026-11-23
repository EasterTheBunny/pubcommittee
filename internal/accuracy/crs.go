package accuracy

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"regexp"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
	"github.com/easterthebunny/pubcommittee/internal/generate"
)

// Coverage verdicts of the CRS check (schemas/crs.json).
const (
	crsCovers      = "covers"
	crsMisses      = "misses"
	crsContradicts = "contradicts"
)

// CRSSummary is the plain text of the CRS summary in a BILLSTATUS file.
type CRSSummary struct {
	VersionCode string
	Text        string
}

// crsStatusXML is the part of a BILLSTATUS file the CRS check reads.
type crsStatusXML struct {
	Summaries []struct {
		Summary []struct {
			VersionCode string `xml:"versionCode"`
			Text        string `xml:"text"`
		} `xml:"summary"`
	} `xml:"bill>summaries"`
}

// ParseCRSSummaries reads the CRS summaries of a BILLSTATUS file in document
// order. A file with no summary returns nil and no error. Malformed XML
// returns an error wrapping data.ErrInvalid.
func ParseCRSSummaries(statusXML []byte) ([]CRSSummary, error) {
	var doc crsStatusXML
	if err := xml.Unmarshal(statusXML, &doc); err != nil {
		return nil, fmt.Errorf("bill status XML: %w: %w", data.ErrInvalid, err)
	}
	var out []CRSSummary
	for _, group := range doc.Summaries {
		for _, s := range group.Summary {
			out = append(out, CRSSummary{
				VersionCode: strings.TrimSpace(s.VersionCode),
				Text:        plainText(s.Text),
			})
		}
	}
	return out, nil
}

// plainText reduces the HTML of a CRS summary to plain text. Paragraph ends
// become newlines before the tags are dropped; entities are decoded last;
// whitespace inside each line is collapsed and empty lines are dropped.
func plainText(markup string) string {
	paragraphEnd := regexp.MustCompile(`(?i)</p>`)
	tag := regexp.MustCompile(`<[^>]+>`)
	s := paragraphEnd.ReplaceAllString(markup, "\n")
	s = tag.ReplaceAllString(s, "")
	return collapseLines(html.UnescapeString(s))
}

// collapseLines joins the non-empty lines of s with newlines, each line with
// its whitespace runs reduced to single spaces.
func collapseLines(s string) string {
	var lines []string
	for line := range strings.SplitSeq(s, "\n") {
		if fields := strings.Fields(line); len(fields) > 0 {
			lines = append(lines, strings.Join(fields, " "))
		}
	}
	return strings.Join(lines, "\n")
}

// crsRequestDoc is the JSON document inside the <request> tags of a CRS request.
type crsRequestDoc struct {
	Task      string `json:"task"`
	Reference string `json:"reference"`
	Generated string `json:"generated"`
}

// CRSRequest builds the coverage request comparing a generated summary
// (headline plus statement texts joined by newlines) with the CRS reference
// text. The grader model is model. The reference is never sent to the
// generating model; this request is for the grader only.
func CRSRequest(id, generated, reference, model string) (data.GenRequest, error) {
	if model == "" {
		return data.GenRequest{}, fmt.Errorf("CRS request %q: no grader model: %w", id, data.ErrInvalid)
	}
	pr, err := generate.LoadPrompt(generate.TaskCRS)
	if err != nil {
		return data.GenRequest{}, fmt.Errorf("CRS request: %w", err)
	}
	body, err := json.Marshal(crsRequestDoc{Task: generate.TaskCRS, Reference: reference, Generated: generated})
	if err != nil {
		return data.GenRequest{}, fmt.Errorf("CRS request: encode: %w: %w", data.ErrInvalid, err)
	}
	return data.GenRequest{
		ID:            id,
		Task:          generate.TaskCRS,
		Instructions:  pr.Instructions,
		Input:         "<request>" + string(body) + "</request>",
		Schema:        pr.Schema,
		Effort:        "medium",
		PromptVersion: pr.Version,
		Model:         model,
	}, nil
}

// CRSPoint is one point of the CRS reference and its verdict on the generated text.
type CRSPoint struct {
	Point   string `json:"point"`
	Verdict string `json:"verdict"`
}

// ParseCRS reads the coverage answer strictly: unknown fields, trailing data,
// a missing points list and verdicts other than covers, misses or contradicts
// are errors wrapping data.ErrInvalid.
func ParseCRS(text string) ([]CRSPoint, error) {
	var doc struct {
		Points []CRSPoint `json:"points"`
	}
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("CRS answer: %w: %w", data.ErrInvalid, err)
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("CRS answer: trailing data after the JSON object: %w", data.ErrInvalid)
	}
	if doc.Points == nil {
		return nil, fmt.Errorf("CRS answer: no points list: %w", data.ErrInvalid)
	}
	for _, p := range doc.Points {
		if !knownVerdict(p.Verdict) {
			return nil, fmt.Errorf("CRS answer: verdict %q for point %q: %w", p.Verdict, p.Point, data.ErrInvalid)
		}
	}
	return doc.Points, nil
}

// knownVerdict reports whether v is one of the CRS verdicts.
func knownVerdict(v string) bool {
	switch v {
	case crsCovers, crsMisses, crsContradicts:
		return true
	}
	return false
}

// Coverage counts the points: total, covered and contradicts.
func Coverage(points []CRSPoint) (total, covered, contradicts int) {
	for _, p := range points {
		total++
		switch p.Verdict {
		case crsCovers:
			covered++
		case crsContradicts:
			contradicts++
		}
	}
	return total, covered, contradicts
}
