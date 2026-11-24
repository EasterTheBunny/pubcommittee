# Obviously fake values. Never put real values here.
project            = "example-project"
region             = "example-region"
firestore_location = "example-region"

images = {
  "read-api"      = "example.invalid/registry/read-api@sha256:0000000000000000000000000000000000000000000000000000000000000000"
  "admin-api"     = "example.invalid/registry/admin-api@sha256:0000000000000000000000000000000000000000000000000000000000000000"
  "job-ingest"    = "example.invalid/registry/job-ingest@sha256:0000000000000000000000000000000000000000000000000000000000000000"
  "job-summarize" = "example.invalid/registry/job-summarize@sha256:0000000000000000000000000000000000000000000000000000000000000000"
  "job-exports"   = "example.invalid/registry/job-exports@sha256:0000000000000000000000000000000000000000000000000000000000000000"
}

bucket_names = {
  documents = "example-documents"
  exports   = "example-exports"
}

operator_principals           = ["user:operator@example.invalid"]
deploy_service_account_member = "serviceAccount:deploy@example.invalid"

metrics_sidecar_image           = "example.invalid/registry/metrics-sidecar:fake"
metrics_sidecar_scrape_interval = "60s"
otlp_endpoint                   = "https://otlp.example.invalid"

ingest_schedule = {
  cron      = "0 0 * * *"
  time_zone = "Etc/UTC"
}

edge_cache_control = "public, max-age=60"
