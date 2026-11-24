# cloud_run_service

One Cloud Run v2 service per unit. The minimum instance count is fixed at 0; `max_instances` is an input. The image is passed by digest; the registry is not created here. Optional managed Prometheus sidecar (image, args, and env, including the scrape interval, are inputs). Invokers are explicit; `allow_unauthenticated` is for public `read-api` only and cannot be combined with `invokers`.

Open: the sidecar scrape configuration mechanism, scrape interval, and histogram buckets are not decided; they are inputs here.

Inputs: see `variables.tf`. Outputs: `name`, `uri` (an environment value; do not publish).
