// Package uslm parses one US Code title file (USLM XML from uscode.house.gov)
// into the section records that the law store keeps (data.LawSection).
//
// Rules:
//   - Top-level sections are the section elements whose identifier is
//     /us/usc/t<N>/s<id>. Sections nested in notes without such an identifier
//     are ignored, and subdivisions are never sections.
//   - Provisions are the section itself, then each subdivision (subsection,
//     paragraph, subparagraph, clause, subclause, item, subitem, subsubitem)
//     in document order. Notes, note, sourceCredit and toc never count and
//     their text is never part of a provision's text.
//   - Dashes in identifiers (en dash, em dash, non-breaking hyphen) are
//     normalised to a hyphen-minus: "1395cc–7" becomes "1395cc-7".
//   - The source is streamed with encoding/xml; only the current section is
//     held as a small tree, and it is discarded after the callback returns.
//   - Same bytes and release give identical output.
package uslm
