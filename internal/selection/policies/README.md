# Selection policy versions

`v1.json` is policy version 1: `mode: pilot`, `phase: early_pilot`. `job-ingest --publish-policy v1` stores it in a local data directory. Publishing a version publicly is an operator action (`pubctl`).

## Topic terms are drafts

Confirmation of the terms, and who maintains them, is an open question for the operator ([01 § Open questions](../../../design/01-bill-ingestion-and-display.md)). Sources:

- **Policy areas and legislative subjects** are the legislature's own names, copied from the BILLSTATUS records in `fixtures/` (the 200-entry catalog sample and the exit bills). Each listed subject appears in at least one of those records. A subject that is in the legislature's vocabulary but absent from the sample is not listed; the operator or a later run over a larger catalog may add it.
- **Title terms** are plain words and phrases, matched as whole words in the bill's title, so a bill can match before subject indexing arrives. A match on title terms alone is recorded as `topic:<name>:title-only`.

Candidates the evidence did not settle (not in v1):

| Topic | Question |
| --- | --- |
| deficit reduction | The sample has one subject, "Budget process". The policy area "Economics and Public Finance" is broader than deficits; adding it tracks every bill indexed there. |
| affordability | "Wages and earnings", "Housing and community development funding", and the policy area "Housing and Community Development" are in the sample; whether they count as affordability is a policy call. |
| healthcare | The policy area "Health" is listed, so every bill the legislature files under Health matches. "Veterans' medical care" and "Animal and plant health" are not listed. |

## Reading a version

`GET /v1/selection-policy` and `/v1/selection-policy/{version}` return the structured fields and a `scope` sentence generated from them.
