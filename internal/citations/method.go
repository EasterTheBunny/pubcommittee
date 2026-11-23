package citations

// RulesVersion is part of every stored citation set's method. Any change to
// extraction or resolution rules that changes output for the same input
// needs a new RulesVersion.
const RulesVersion = "citations/1"

// Method returns the method string stored in a CitationRef: the parser
// version, the rules version, and the US Code release point the citations
// were resolved against.
func Method(parserVersion, release string) string {
	return parserVersion + "+" + RulesVersion + "+usc@" + release
}
