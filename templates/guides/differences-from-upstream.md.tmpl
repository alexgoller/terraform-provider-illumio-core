---
page_title: "Differences from the upstream provider"
subcategory: ""
description: |-
  Every behavioural difference between this fork and illumio/terraform-provider-illumio-core, and why each one exists.
---

# Differences from the upstream provider

A complete list of how this fork differs from
[`illumio/terraform-provider-illumio-core`](https://github.com/illumio/terraform-provider-illumio-core).

The fork is a **superset**: no resource or data source was removed, and no
schema changed in a way that breaks an existing configuration. One *runtime*
behaviour changed deliberately — destroying a managed workload — and it is
called out below.

## Summary

| Area | Upstream | This fork |
|---|---|---|
| Deny rules | Not supported | `illumio-core_deny_rule` + 2 data sources |
| Provisioning | External `provision` binary + `hrefs.csv` | `illumio-core_provisioning` resource |
| Object deleted outside Terraform | `terraform plan` hard-errors | Removed from state, recreated |
| Destroying a managed workload | Unpairs the VEN, uninstalling the agent | Relinquishes management; opt in with `unpair_on_destroy` |
| Adopting a managed workload | `terraform import` by HREF | Also by hostname/name, or `hostname` on the resource |
| Declaring an object that already exists | Apply fails with a 406 | Adopted; destroy leaves it in place |
| Collection data sources | Silently returned a capped subset | Use the async API; complete results |
| A field missing from an API response | Panics the provider | Reads as empty |
| `provision` binary | Could drop a change and exit 0 | Fails loudly, non-zero exit |
| `go build ./...` | Fails on a fresh clone | Builds |
| Distribution | Terraform Registry | Provider mirror (`scripts/build-mirror.sh`) |

## New functionality

### Deny rules

`illumio-core_deny_rule`, plus `illumio-core_deny_rule` and
`illumio-core_deny_rules` data sources. Ordinary and override-deny rules are the
same object distinguished by an `override` flag.

See [Deny rules](deny-rules.html).

⚠️ The `{rule_set_href}/deny_rules` endpoint is **not in the 26.3 OpenAPI
specification** and its schema is flagged as a private-permission feature. It
was verified against a live PCE at 26.30.2, but will not work where the feature
is not enabled.

### Policy provisioning as a resource

`illumio-core_provisioning` replaces `terraform apply && provision`. Provisioning
becomes state-tracked, visible in `terraform plan`, and usable in Terraform
Cloud and CI, where the binary cannot run.

See [Policy provisioning](policy-provisioning.html).

New provider setting `write_href_file` (default `true`, deprecated) stops
`hrefs.csv` being written for adopters of the resource.

## Changed behaviour

### Destroying a managed workload no longer uninstalls the agent

**This is the one deliberate breaking change.**

Upstream, `terraform destroy` on an `illumio-core_managed_workload` called
`PUT /orgs/{org}/vens/unpair` with `firewall_restore: "default"`, uninstalling
the VEN from the host and restoring its firewall.

Managed workloads cannot be created by Terraform — they exist because a VEN
paired with the PCE, and the create function refused outright. The provider was
therefore uninstalling an agent it never installed, triggered by a destroy, a
removal from configuration, or a resource replacement.

Destroying now relinquishes management: the resource leaves state, the VEN stays
paired, and a warning says so. To restore the old behaviour:

```hcl
resource "illumio-core_managed_workload" "web" {
  unpair_on_destroy = true
}
```

### Objects deleted outside Terraform no longer wedge the configuration

Upstream, if a managed object was deleted in the PCE UI or by another tool,
`terraform plan` failed outright:

```
Error: not-found: /orgs/1/sec_policy/draft/ip_lists/7
```

Plan, apply and destroy then all failed until `terraform state rm` was run by
hand. This fork removes the object from state so the next plan recreates it,
which is Terraform's normal contract.

This looked intermittent upstream: labels, label types and unmanaged workloads
recovered, because Illumio *soft*-deletes those and bespoke customizers handled
it. Everything hard-deleted — ip lists, rule sets, services, virtual services,
enforcement boundaries — hit the error.

Any error other than a 404 is still surfaced, so a transient outage cannot
silently drop real infrastructure from state.

### Declaring an object that already exists

Upstream, declaring a label, IP list, label group or rule set that already
exists on the PCE failed the apply — the PCE rejects the duplicate with a 406
rather than returning the existing object:

```
[{"token":"label_key_and_value_must_be_unique","message":"Label key and value must be unique"}]
```

Adopting it meant `terraform import` with a numeric HREF, per object.

This fork adopts it instead: Terraform looks the object up by the fields that
make it unique, manages it, and records `adopted = true`. Destroying an adopted
object removes it from state but **leaves it in the PCE**, because Terraform did
not create it. Objects Terraform did create are still deleted normally.

`illumio-core_service` is deliberately excluded: the PCE permits duplicate
service names, so there is no unique key to adopt by.

See [Adopting existing objects](adopting-existing-objects.html).

### Adopting managed workloads

Upstream, the only route was `terraform import` with a full HREF containing a
PCE-generated UUID, looked up in the UI per host.

This fork adds two routes, both still supported alongside HREF import:

```bash
terraform import illumio-core_managed_workload.web "web-01.example.com"
terraform import illumio-core_managed_workload.web "hostname=web-01.example.com"
```

```hcl
resource "illumio-core_managed_workload" "fleet" {
  for_each = toset(var.managed_hostnames)
  hostname = each.value
}
```

See [Managed workloads](managed-workloads.html).

## Correctness fixes

### The `provision` binary could report success while dropping a change

Four silent-failure paths stacked upstream:

- the pending-policy scan used a hardcoded list of six resource types, with
  `firewall_settings` special-cased
- an href missing from that set was logged as a warning and skipped
- `SecurityPolicyChangeSubset.AppendHref` dispatched through a `switch` with
  **no `default` case**, discarding unrecognised types
- no failure path set a non-zero exit status

So a new provisionable object type was recorded, warned about once, dropped, and
reported as success while the policy stayed in draft. `virtual_servers` had no
field on the change subset at all, making it undeliverable rather than merely
dropped.

In this fork the binary derives the resource type from the href, so unknown
object types work without changing it; reports one error per unusable line;
refuses to provision a partial change subset; leaves `hrefs.csv` in place on
failure so the run can be retried; and exits non-zero on any failure. It also no
longer panics on a line without a comma, and no longer treats a missing
`hrefs.csv` as fatal — so `terraform apply && provision` stops failing on
applies that touched no policy objects.

The binary and the provider now share one implementation of
`ResourceTypeFromHref` and pending-policy collection, so they cannot disagree
about what is provisionable.

### Collection data sources silently returned a subset

Plural data sources called the plain `Get`, which is capped by the PCE when
`max_results` is not set. The PCE reports the real total in `X-Total-Count`, but
the provider never read that header — there were no references to it anywhere —
so a data source could hand Terraform a partial list with no warning, and the
configuration would then act on incomplete data.

The provider already contained `AsyncGet`, which switches to the PCE's
`Prefer: respond-async` job API above 500 results, but **no data source used
it**. All 22 collection data sources now do.

The PCE has no `offset` or `page` parameter, so the async job API is the only
way to retrieve a large collection.

Measured against a live PCE holding 550 workloads, with the same configuration
and the same PCE:

| | `illumio-core_workloads` returned |
|---|---|
| Upstream (plain `Get`) | **500** |
| This fork (`AsyncGet`) | **550** |

The PCE caps an uncapped `GET /workloads` at 500 while reporting
`X-Total-Count: 550`. Upstream dropped 50 workloads with no warning, no error
and a successful apply.

### A missing field crashed the provider

67 places read API values as `container.S("field").Data().(string)`, which
panics when the field is absent:

```
interface conversion: interface {} is nil, not string
```

A PCE version that omits an optional field, or a permission that hides one,
therefore crashed Terraform mid-apply — which risks leaving state diverged from
reality. All of them now read through a helper that returns an empty string
instead. One site also indexed `Children()[0]` without a length check, panicking
when the PCE returned an empty array.

### Recording an href for provisioning leaked a file handle and could panic

`Config.StoreHref` opened `hrefs.csv` and never closed it, leaking a descriptor
on every policy mutation, and called `panic()` when the file could not be opened
or written. A full disk or a read-only working directory therefore crashed the
provider. It now closes the handle and logs an error.

### A failed VEN unpair reported success

The diagnostics returned by the unpair call were discarded. They are now
returned.

### `go build ./...` failed on a fresh clone

`.gitignore` contained `**/terraform.*`, which matches `vendor/**/terraform.go`.
The vendored files defining `hc-install`'s `simpleVersionRe` and
`terraform-exec`'s `Terraform` type were never committed:

```
hc-install/product/consul.go:18:60: undefined: simpleVersionRe
terraform-exec/tfexec/apply.go:109:11: undefined: Terraform
```

The pattern is narrowed and the files restored. This affects anyone building
from source.

### Client unit tests required PCE credentials

The credential check was a `log.Fatal` in `init()`, which killed the whole test
binary, so no unit test in the `client` package could run without a PCE. Tests
needing a PCE now skip instead.

## Distribution

The fork is published to the Terraform Registry as `alexgoller/illumio-core`,
a separate provider from the upstream `illumio/illumio-core` with its own
version stream and signing key. `scripts/build-mirror.sh` still builds a
provider mirror, which is needed only for airgapped networks or to test an
unreleased build.

See [Using this fork](using-this-fork.html).

### PCE version-compatibility audit

All 28 schemas that differ between the 25.2.20 and 26.3.0 bundles were checked
against what the provider sends. **Deny rules were the only defect.**

Method: a field missing from a release only breaks something if the provider
*writes* it, so only `*_post` and `*_put` schemas matter. Those 26.3-only write
fields, and whether any resource covers them:

| Field | Write schema | Covered by a resource? |
|---|---|---|
| `all_ips_except_for_in_consumers` / `_providers` | `sec_rules_post/put`, `deny_rules` | **Yes — deny rule. Fixed in 2.0.1** |
| `ip_list_attribute_id` | `sec_policy_ip_lists_post/put` | No |
| `overwrite` | `label_mapping_rules_post/put` | No |
| `members` | `security_principals_put` | No |
| `uuid` | `service_accounts_post` | No |
| `total_internet_address_space`, `total_lateral_address_space` | `settings_put` | No |
| `golden_image` | `vens_put` | No |
| `counts` | `sec_policy_rule_search_post` | No |

`sec_rules_post/put` gained the same two `all_ips_except_*` fields as deny
rules, but `illumio-core_security_rule` does not reference them, so it was never
affected.

The reverse direction was checked too — a field 26.3 *removed* would break a
newer PCE. Only one exists on a write path, `use_census_permissions` on
`settings_put`, and no resource sends it.

The 16 schemas that exist only in 26.3.0 — `inactive_ven_cleanup_schedules`,
`ip_list_attributes`, `security_principal_users`, `vens_bulk_transplant` and
others — have no resource in the provider, so they cannot be a source of
version breakage.

Reads are safe in the other direction as well: a 25.2 response simply omits
26.3 fields, and since 2.0.0 every API value is read through a helper that
treats a missing field as empty rather than panicking.

Re-run this audit after archiving a new release:

```bash
scripts/diff-api-schema.py 25.2.20 26.3.0
```

## Not changed

- No resource or data source was removed.
- No existing schema changed incompatibly; existing configurations and state
  work unchanged.
- `illumio-core_security_rule` is untouched. The deny-rule resource is separate
  rather than an `action` field on the security rule, because the two have
  different endpoints and different schemas.
- Upstream's perpetual-diff behaviour was audited and found sound. Ten resources
  plan clean immediately after apply, including CIDR with host bits set, IP
  ranges, upper-case FQDNs, port ranges, and empty optional strings; and ip
  lists, services, enforcement boundaries, rule sets and labels all plan clean
  after `state rm` plus re-import. The work in upstream 1.1.4/1.1.5 appears to
  have covered this.
