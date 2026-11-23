// Package versiondiff compares two official versions of a bill. It maps each
// old line to its counterpart in the newer version and classifies it as
// unchanged, moved, changed, or removed (design/02 § New official version
// handling). The result is a data.Comparison, stored write-once per
// (bill, from, to, method) and read by read-api and, from M2, by
// re-anchoring.
//
// Rules, in order:
//
//  1. Line text is normalised: runs of white space collapse to one space and
//     the ends are trimmed. Case, punctuation, and wording are kept.
//  2. Sections are paired by source id, then by label and heading, then by
//     heading alone.
//  3. Inside a pair of sections, a longest common subsequence of identical
//     lines is unchanged; if the two sections carry different labels
//     (renumbered) those lines are moved.
//  4. Remaining lines with identical text anywhere are moved. Lines equal
//     once a leading enumerator such as "(b)" or "SEC. 4." is removed are
//     also moved (renumbered).
//  5. Remaining lines pair by word similarity (Dice coefficient on words);
//     a pair scoring at least ChangedThreshold is changed. Below it, the old
//     line is removed and the new line is added.
//
// The result depends only on the two texts and AlgorithmVersion. Any change
// to these rules needs a new AlgorithmVersion.
package versiondiff
