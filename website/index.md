---
layout: default
title: "Overview"
description: "Terraform provider for Illumio Core, with deny rules, Terraform-native policy provisioning, and reconcile fixes."
---

# Illumio Core Terraform Provider

A fork of [`illumio/terraform-provider-illumio-core`](https://github.com/illumio/terraform-provider-illumio-core)
that manages Illumio PCE security policy from Terraform.

It is a **superset** of the upstream provider: nothing was removed and no schema
changed incompatibly, so existing configurations and state work unchanged.

## What this fork adds

### Deny rules

Deny rules and override-deny rules are not supported upstream at all. This fork
adds `illumio-core_deny_rule` plus singular and plural data sources.

```hcl
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false

  providers {
    label {
      href = illumio-core_label.internal.href
    }
  }

  consumers {
    ip_list {
      href = illumio-core_ip_list.external.href
    }
  }

  ingress_services {
    proto = "6"
    port  = "22"
  }
}
```

Consumers are the sources that initiate the connection; providers are the
destinations. Set `override = true` for an override-deny rule — the same object
at the same endpoint, distinguished only by that flag. Evaluation order is
`override-deny > allow > deny`, and an override-deny rule still *blocks*
traffic.

### Terraform-native provisioning

Policy objects created by Terraform land in the PCE's `draft` version and only
take effect once provisioned. Upstream requires running a separate binary
afterwards — `terraform apply && provision` — which cannot work in Terraform
Cloud or most CI, because the provider writes a `hrefs.csv` file that the binary
then reads from disk.

`illumio-core_provisioning` makes provisioning a resource: state-tracked,
visible in `terraform plan`, and ordered by the dependency graph.

```hcl
resource "illumio-core_provisioning" "policy" {
  hrefs = [illumio-core_rule_set.app.href]

  lifecycle {
    replace_triggered_by = [illumio-core_rule_set.app]
  }
}
```

The `replace_triggered_by` block is **not optional in practice**. A policy
object's HREF does not change when its contents change, so without it a
contents-only edit is not provisioned until the *following* apply.

### Reconcile fix

Upstream's `terraform plan` fails outright when an object is deleted outside
Terraform:

```
Error: not-found: /orgs/1/sec_policy/draft/ip_lists/7
```

Plan, apply and destroy all then fail until you run `terraform state rm` by
hand. This fork removes the object from state instead, so the next plan
recreates it — Terraform's normal behaviour.

### Correctness fixes

- The upstream `provision` binary could drop a policy change and still exit 0.
- Upstream `main` does not compile from a fresh clone: a `.gitignore` pattern
  excluded two vendored files.

## Getting started

This fork is published to the Terraform Registry as
[`alexgoller/illumio-core`](https://registry.terraform.io/providers/alexgoller/illumio-core/latest)
— a separate provider from the upstream `illumio/illumio-core`, with its own
version stream and signing key.

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "alexgoller/illumio-core"
      version = "2.0.11"
    }
  }
}
```

```bash
terraform init
```

Releases are GPG-signed and published for 17 platforms — linux, darwin, windows,
freebsd and openbsd across `386`, `amd64`, `arm` and `arm64`.

**Already using the upstream provider?** Change `source`, then rewrite the
address recorded in state:

```bash
terraform state replace-provider \
  registry.terraform.io/illumio/illumio-core \
  registry.terraform.io/alexgoller/illumio-core
```

`terraform plan` should then report no changes. Airgapped installs and the
mirror route are covered in the
[Using this fork](guides/using-this-fork.html) guide.

## Documentation

| Guide | Covers |
|---|---|
| [Organizing policy](guides/policy-architecture.html) | Structuring policy as three pillars across many teams: precedence, scope contracts, state boundaries, ownership |
| [Using this fork](guides/using-this-fork.html) | Installing from the registry, migrating to and from upstream, airgapped installs |
| [Deny rules](guides/deny-rules.html) | `illumio-core_deny_rule`, override-deny, the narrower actor set, ICMP |
| [Policy provisioning](guides/policy-provisioning.html) | `illumio-core_provisioning` in place of the `provision` binary |
| [Adopting existing objects](guides/adopting-existing-objects.html) | Declaring labels, IP lists, label groups and rule sets that already exist |
| [Managed workloads](guides/managed-workloads.html) | Adopting VEN-paired workloads, import by hostname, destroy semantics |
| [Differences from upstream](guides/differences-from-upstream.html) | Every behavioural difference, and why |

Per-resource reference documentation is in the sidebar.

## Known limitations

- **Deny rules depend on an undocumented PCE feature.** The
  `{rule_set_href}/deny_rules` endpoint does not appear in the 26.3 OpenAPI
  specification and its schema is flagged as a private-permission feature. It
  was verified against a live PCE at 26.30.2, but will not work where the
  feature is not enabled.
- **`terraform plan` needs PCE credentials** when using
  `illumio-core_provisioning`, because it queries the pending policy set.
- **`terraform destroy` does not provision deletions.** The provisioning
  resource is destroyed before the objects it provisions, so their deletions
  stay in draft.

## Source

[github.com/alexgoller/terraform-provider-illumio-core](https://github.com/alexgoller/terraform-provider-illumio-core)
