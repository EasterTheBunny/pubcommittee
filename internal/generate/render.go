package generate

import (
	"fmt"
	"strings"

	"github.com/easterthebunny/pubcommittee/internal/data"
)

// PrevPrefix marks anchors of the earlier version in a version change summary.
const PrevPrefix = "prev:"

// renderSource writes the version's full text as delimited source material,
// one `[anchor] text` row per line. prefix is added to every anchor.
func renderSource(tag string, v data.OfficialVersion, prefix string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<%s bill=%q version=%q>\n", tag, v.Ref.Bill.String(), v.Ref.Version)
	for _, s := range v.Sections {
		fmt.Fprintf(&b, "== section %s %s ==\n", strings.Join(s.Path, "."), s.Heading)
		for _, l := range s.Lines {
			fmt.Fprintf(&b, "[%s%s] %s\n", prefix, data.AnchorID(s.Path, l.N, l.N), l.Text)
		}
	}
	fmt.Fprintf(&b, "</%s>\n", tag)
	return b.String()
}

// lineMap maps every single-line anchor of the version (with prefix) to its text.
func lineMap(v data.OfficialVersion, prefix string, into map[string]string) {
	for _, s := range v.Sections {
		for _, l := range s.Lines {
			into[prefix+data.AnchorID(s.Path, l.N, l.N)] = l.Text
		}
	}
}

// sectionAnchors lists the single-line anchors of one section, in order.
func sectionAnchors(s data.Section) []string {
	out := make([]string, 0, len(s.Lines))
	for _, l := range s.Lines {
		out = append(out, data.AnchorID(s.Path, l.N, l.N))
	}
	return out
}
