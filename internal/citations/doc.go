// Package citations extracts references to existing law from a stored bill
// version, resolves them to US Code provisions with a confidence state,
// attaches provision excerpts and freshness flags, and stores the result
// write-once per version and method (design/01 § Domain concepts and
// § Existing law freshness).
package citations
