# firestore

Firestore database in native mode (pay per use) and composite indexes. The index list belongs to the Firestore adapter (M1-03b); the deployment passes it in as `indexes`. Deletion protection is on by default.

Fields the adapter never queries (its `body` JSON) are exempt from single-field indexing through `exempt_fields`.

Inputs: `project`, `location`, `database_name`, `deletion_protection`, `indexes`, `exempt_fields`. Output: `database_name`.
