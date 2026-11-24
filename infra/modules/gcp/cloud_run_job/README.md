# cloud_run_job

One Cloud Run v2 job per unit, with the OTLP endpoint as an input and an optional Cloud Scheduler trigger. `schedule = null` (the default) means manual runs only. When set, a separate scheduler service account gets `roles/run.invoker` on this job only. Which jobs are scheduled, and when, is an open question: no cadence is hard-coded.

Inputs: see `variables.tf`. Output: `name`.
