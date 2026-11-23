// Package billxml turns an official Congress bill XML document (govinfo
// BILLS, bill.dtd) into the parsed version every other feature reads:
// ordered sections, lines with depth, kind, anchor and source id, and the
// raw markup of non-prose units.
//
// Rules (design/01 § Domain concepts, architecture § Bill source):
//   - Parsing runs in ingest only, never on a reader's request. The same
//     bytes and the same ParserVersion give byte-identical output.
//   - Depth comes from the XML nesting. A line is emitted for each element
//     that carries its own text (enum, header, text); an element's own line
//     is its head line, and its child elements nest one level deeper.
//   - Quoted blocks (text inserted into existing law) are kept distinct:
//     their lines are flagged Quoted and nest under the line that
//     introduces them.
//   - Tables, formulas, graphics and tables of contents are single lines
//     with their raw markup kept.
//   - Nothing is dropped: unknown elements keep their text and add a
//     warning.
package billxml
