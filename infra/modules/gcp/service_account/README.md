# service_account

One Google service account per unit, plus optional project roles. Every value is an input. Roles for SQL, KMS, owner, and editor are rejected (M1 has none; `read-api` must never hold them).

Inputs: `project`, `account_id`, `display_name`, `project_roles`. Outputs: `email`, `member`.
