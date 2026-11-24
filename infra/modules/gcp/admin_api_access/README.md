# admin_api_access

Operator-only `roles/run.invoker` bindings on `admin-api`, from the operator roster passed in by the private deployment repository, plus an optional deploy service account binding. Public principals are rejected. Cloud Run IAM cannot restrict a caller to one route, so `admin-api` itself accepts the deploy account only on the roster route.

Inputs: `project`, `region`, `service_name`, `operator_principals`, `deploy_service_account_member`. Output: `operator_count`.
