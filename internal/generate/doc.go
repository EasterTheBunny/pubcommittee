// Package generate builds the generation tasks of M1-09 from stored official
// text, citations and comparisons, checks the model outputs with the
// deterministic accuracy validators, and stores them. It depends on ports
// only; adapters (claude, fakegen) implement data.Generator.
package generate
