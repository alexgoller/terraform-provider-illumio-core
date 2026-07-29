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

The fork is not published to the Terraform Registry, so
`source = "illumio/illumio-core"` resolves to the upstream provider unless
Terraform is told otherwise. `scripts/build-mirror.sh` builds a provider mirror
that serves the fork under the same source address.

See [Using this fork](using-this-fork.html).

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
