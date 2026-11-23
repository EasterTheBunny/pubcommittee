---
name: tool-opentofu
description: Writing PubCommittee's public OpenTofu modules under infra/modules/gcp/ and the placeholder example root, and validating them in a container with fake values and no cloud credentials. Cloud phase only (M1-15). Load for any change under infra/.
---

# OpenTofu modules (public)

Read [architecture § Public modules, private deployment](../../../architecture/README.md#public-modules-private-deployment). Environment roots, values, state, and the operator roster live in the private deployment repository; never here. Task-orchestrators and workers never read `deploy/`; only the top-level agent may, and it runs only read-only `tofu` commands there ([AGENTS.md](../../../AGENTS.md)).

## Module rules

- One module per concern under `infra/modules/gcp/<name>/`: `main.tf`, `variables.tf`, `outputs.tf`, `versions.tf`, `README.md`.
- **Every** project, region, name, domain, and principal is an input. No defaults that look like real values.
- No network specifics or defense tuning (rate limits, detection thresholds) hard-coded.
- No sensitive outputs; no secret values in variables (create Secret Manager secrets without values).
- Scale to zero: no load balancers, NAT gateways, minimum instances, or always-on databases.
- Least privilege: one service account per unit; `read-api` gets no SQL or key roles.

## Validate (inside a container, no credentials)

```sh
docker exec -w /src/infra/examples <container> tofu fmt -check -recursive /src/infra
docker exec -w /src/infra/examples <container> tofu init -backend=false
docker exec -w /src/infra/examples <container> tofu validate
docker exec -w /src/infra/examples <container> tofu plan -refresh=false -var-file=fake.tfvars   # only if the providers allow a credential-free plan
```

- Never `tofu apply`, never `gcloud`, never authenticate to a cloud, never use real values. Those are operator steps in the deployment repository: print the command for the operator instead.
- `infra/examples/fake.tfvars` uses obviously fake values (`example-project`, `example.invalid`).
