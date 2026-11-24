# infra

Public, reusable OpenTofu modules and a placeholder example root. Environment roots, values, state, and the operator roster live in the private deployment repository, which pins modules by tag (`?ref=modules/vX.Y.Z`; `v0.1.0` predates the prefix). Module tags never start a deploy; `vX.Y.Z` tags are application releases. See architecture, "Public modules, private deployment".

- `modules/gcp/`: one module per concern. Every project, region, name, domain, and principal is an input; no real-value defaults; no sensitive outputs; scale to zero.
- `examples/`: wires every module with fake values in `fake.tfvars`. Validate and plan without credentials:

```sh
tofu init -backend=false && tofu validate
tofu plan -refresh=false -var-file=fake.tfvars
```

Never run `tofu apply` from this repository. The shared Artifact Registry and the per-environment deploy identities are created by the private bootstrap, not by these modules; modules take image references as input.
