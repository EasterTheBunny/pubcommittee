# firebase_hosting_edge

Firebase Hosting site as the API edge (CDN and TLS, no load balancer): rewrites to Cloud Run services and Cache-Control headers per path glob, both inputs. Route only public services; `admin-api` is never listed. The custom domain is optional (`null` runs on the provider domain; `<domain>` is not decided).

Uses the `google-beta` provider. Inputs: see `variables.tf`. Outputs: `site_id`, `default_url`.
