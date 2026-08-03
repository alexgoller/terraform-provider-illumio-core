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

This fork is **not published to the Terraform Registry**, so
`source = "illumio/illumio-core"` resolves to the upstream provider unless you
tell Terraform otherwise. Build a provider mirror:

```bash
git clone https://github.com/alexgoller/terraform-provider-illumio-core.git
cd terraform-provider-illumio-core
scripts/build-mirror.sh
```

Then point Terraform at it in `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/absolute/path/to/mirror"
    include = ["registry.terraform.io/illumio/illumio-core"]
  }
  direct {
    exclude = ["registry.terraform.io/illumio/illumio-core"]
  }
}
```

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "illumio/illumio-core"
      version = "2.0.3"
    }
  }
}
```

`terraform init` now installs the fork. The full instructions, including sharing
a mirror with a team over HTTPS and migrating to and from upstream, are in the
[Using this fork](guides/using-this-fork.html) guide.

## Documentation

| Guide | Covers |
|---|---|
| [Using this fork](guides/using-this-fork.html) | Installing it instead of the upstream provider, distributing it without a registry, migrating both ways |
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
