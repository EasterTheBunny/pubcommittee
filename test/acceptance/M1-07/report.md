# M1-07 resolution report

Measured on 2026-11-22 with `go run -tags memory ./scripts/measure-citations -law .local-data/uscode/119-142 -labels test/acceptance/M1-07/labels.json` against release point 119-142 and the 8 fixture bills (all their versions).

How the sample was labelled, and what that means:
- Labels (`labels.json`) cover every citation of the 8 fixture bills: the three exit bills and the five parser-corpus bills. They were written by the task agent, not a steward: the expected outcome of each distinct citation was derived with an independent script (a Python check of the cited section and subdivision against the title files' own identifiers; the text patterns re-implemented in the script), then read through. A steward should review them; until then the figures are agent-labelled.
- Because the script and the resolver apply the same rule (does the cited provision exist in the release point), precision of `resolved` and state accuracy here show that the code implements the stated rules, not that the rules are right for every bill. The informative numbers are the resolution rate and the breakdown of what stays unresolved: public laws, chapters, and Acts without a mapping source.
- Act-section labels record the outcome expected given the stored sources (no mapping table), not the ideal outcome. Their "unresolved" state counts as correct.
- Titles loaded: 3, 5, 10, 15, 16, 18, 20, 23, 25, 31, 34, 36, 38, 42, 49 (every title the sample cites).

## Results

Release point `119-142` (current through 2026-11-10). Counts are occurrences in the labelled bill versions.

| Measure | Value |
| --- | --- |
| Labelled citations | 200 |
| Resolution rate (labelled citations resolved correctly / all labelled) | 64.0% (128/200) |
| Detection (labelled citations found) | 100.0% (200/200) |
| State correct | 100.0% (200/200) |
| State and provision correct | 100.0% (200/200) |
| Precision of `resolved` (detected resolved that are right) | 100.0% (128/128) |
| Coverage of `resolved` (labelled resolved that were resolved right) | 100.0% (128/128) |
| Linked coverage (labelled resolved or ambiguous that were linked) | 100.0% (138/138) |
| Detected but not labelled | 0 |

| Kind | Labelled | Found | State correct | Precision of resolved | Coverage of resolved |
| --- | --- | --- | --- | --- | --- |
| act-section | 20 | 100.0% (20/20) | 100.0% (20/20) | 100.0% (4/4) | 100.0% (4/4) |
| public-law | 26 | 100.0% (26/26) | 100.0% (26/26) | n/a | n/a |
| usc | 137 | 100.0% (137/137) | 100.0% (137/137) | 100.0% (124/124) | 100.0% (124/124) |
| usc-chapter | 17 | 100.0% (17/17) | 100.0% (17/17) | n/a | n/a |

## Mismatches with the labels (0)

## Detected but not in the labels (0)
