#!/bin/sh
# M1-15 acceptance, public-module part (no cloud credentials, no apply):
# fmt, validate, and a refresh-free plan of infra/examples/ with fake values,
# plus static checks of the module rules. Criteria needing a real project
# (deploy identity, idle cost, environments, leak scans) are cloud phase and
# run from the private deployment repository.
# Environment: TOFU (default "tofu"; set to a wrapper that runs tofu in a container),
#              INFRA_DIR (default <repo>/infra).
set -eu
here=$(cd "$(dirname "$0")" && pwd)
infra=${INFRA_DIR:-$here/../../../infra}
tofu=${TOFU:-tofu}
ex=$infra/examples
mod=$infra/modules/gcp

fail() {
	echo "FAIL: $1" >&2
	exit 1
}

$tofu fmt -check -recursive "$infra" >/dev/null || fail "tofu fmt"
echo "ok: fmt"

(cd "$ex" && $tofu init -backend=false -input=false >/dev/null && $tofu validate >/dev/null) || fail "validate"
echo "ok: validate"

plan=$(cd "$ex" && $tofu plan -refresh=false -input=false -no-color -var-file=fake.tfvars) || fail "plan with fake values"
echo "$plan" | grep -q '^Plan: ' || fail "plan produced no summary"
echo "ok: plan with fake values and no credentials"

# Every module has the standard files.
for d in "$mod"/*/; do
	for f in main.tf variables.tf outputs.tf versions.tf README.md; do
		[ -f "$d$f" ] || fail "$d$f missing"
	done
done
echo "ok: module files"

# No fixed-cost resources.
if grep -Eq 'google_compute_(global_)?forwarding_rule|google_compute_router_nat|google_sql_database_instance|google_kms_' $(find "$mod" -name "*.tf") "$ex/main.tf"; then
	fail "fixed-cost or out-of-scope resource found"
fi
if grep -Eq 'min_instance_count *= *[1-9]' $(find "$mod" -name "*.tf"); then
	fail "minimum instances set"
fi
echo "ok: no fixed-cost resources"

# Variables that name a project, region, name, image, or domain have no default.
for f in "$mod"/*/variables.tf; do
	awk -v file="$f" '
		/^variable "/ { name = $2; gsub(/"/, "", name); isid = (name ~ /^(project|region|name|image|location|site_id|account_id|service_name|custom_domain)$/) }
		/^  default/ && isid && $0 !~ /null/ { print file ": default on " name; bad = 1 }
		END { exit bad }
	' "$f" || fail "real-value-shaped default"
done
echo "ok: no defaults on identifying inputs"

# admin-api is never routed through the edge, never public.
grep -q 'admin-api' "$ex/main.tf" || fail "example lacks admin-api"
if awk '/module "edge"/,0' "$ex/main.tf" | grep -q 'admin'; then
	fail "admin-api referenced by the edge"
fi
if awk '/module "admin_api" *[{]/,/^}/' "$ex/main.tf" | grep -q 'allow_unauthenticated *= *true'; then
	fail "admin-api allows unauthenticated calls"
fi
echo "ok: admin-api not on the edge and not public"

# Planted violations are rejected by module validation.
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp -r "$infra" "$tmp/infra"
rm -rf "$tmp/infra/examples/.terraform"
sed -i 's#"roles/datastore.viewer"\]$#"roles/datastore.viewer", "roles/cloudsql.client"]#' "$tmp/infra/examples/main.tf"
grep -q 'roles/cloudsql.client' "$tmp/infra/examples/main.tf" || fail "plant did not apply"
(cd "$tmp/infra/examples" && $tofu init -backend=false -input=false >/dev/null)
if out=$(cd "$tmp/infra/examples" && $tofu plan -refresh=false -input=false -no-color -var-file=fake.tfvars 2>&1); then
	fail "SQL role on a unit account was accepted"
fi
echo "$out" | grep -q 'SQL, KMS, owner, and editor roles are not allowed' || fail "plant failed for another reason"
echo "ok: SQL role on a unit account is rejected"

echo "M1-15 public-module checks passed"
