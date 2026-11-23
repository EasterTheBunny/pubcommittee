package billxml

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// ParserVersion is stamped on every output. Any change to what Parse emits
// for the same bytes needs a new value: re-parsing is a deliberate act that
// writes a new parsed version beside the old one.
const ParserVersion = "billxml/1"

// Line kinds.
const (
	KindText    = "text"
	KindGroup   = "group" // heading of a division, title, subtitle, chapter, ...
	KindTable   = "table"
	KindToc     = "toc"
	KindFormula = "formula"
	KindGraphic = "graphic"
)

// Document is the parsed version, serialised as text.json.
type Document struct {
	ParserVersion string    `json:"parser_version"`
	SourceSHA256  string    `json:"source_sha256"`
	Kind          string    `json:"kind"`            // "bill" or "resolution"
	Stage         string    `json:"stage,omitempty"` // bill-stage attribute, for example "Reported-in-House"
	LegisNum      string    `json:"legis_num,omitempty"`
	OfficialTitle string    `json:"official_title,omitempty"`
	SourceID      string    `json:"source_id,omitempty"` // dms-id of the bill element
	Sections      []Section `json:"sections"`
	Warnings      []string  `json:"warnings"`
}

// Section is one structural unit: a section element, the preamble (path
// ["s0"]), or a run of text outside any section.
type Section struct {
	Path     []string `json:"path"` // ["s4"]: "s" and the section's position, from 1
	Enum     string   `json:"enum"` // the section's enumerator as printed, for example "4."
	Label    string   `json:"label"`
	Heading  string   `json:"heading"`
	SourceID string   `json:"source_id,omitempty"`
	Lines    []Line   `json:"lines"`
}

// Line is the smallest addressable unit. N counts from 1 across the whole
// version in document order; Anchor is the UI fragment form "s4-l12".
type Line struct {
	N            int    `json:"n"`
	Anchor       string `json:"anchor"`
	Depth        int    `json:"depth"`
	Text         string `json:"text"`
	SourceID     string `json:"source_id,omitempty"`
	Kind         string `json:"kind"`
	Quoted       bool   `json:"quoted,omitempty"`       // inside a quoted-block: text inserted into existing law
	Continuation bool   `json:"continuation,omitempty"` // finishes the sentence of the line above it, at that line's depth
	Raw          string `json:"raw,omitempty"`          // original markup of a non-prose unit
}

// Marshal returns the canonical text.json bytes: two-space indent, no HTML
// escaping, trailing newline. Same document, same bytes.
func Marshal(d *Document) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(d); err != nil {
		return nil, fmt.Errorf("billxml: marshal: %w", err)
	}
	return buf.Bytes(), nil
}

// DataSections converts the sections to the domain type used by the ports.
func (d *Document) DataSections() []data.Section {
	out := make([]data.Section, len(d.Sections))
	for i, s := range d.Sections {
		ds := data.Section{Path: s.Path, Label: s.Label, Heading: s.Heading, SourceID: s.SourceID}
		ds.Lines = make([]data.Line, len(s.Lines))
		for j, l := range s.Lines {
			ds.Lines[j] = data.Line{N: l.N, Depth: l.Depth, Text: l.Text, SourceID: l.SourceID}
		}
		out[i] = ds
	}
	return out
}
