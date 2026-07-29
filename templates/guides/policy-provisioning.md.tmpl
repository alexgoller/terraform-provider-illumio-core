---
page_title: "Policy provisioning"
subcategory: ""
description: |-
  Provisioning draft Illumio policy from Terraform with illumio-core_provisioning, replacing the external provision binary.
---

# Policy provisioning

Policy objects created by Terraform land in the PCE's **draft** version. They
take effect only once provisioned into the active policy.

Upstream requires running a separate binary afterwards:

```bash
terraform apply && provision
```

That cannot work in Terraform Cloud or most CI, because the provider writes a
`hrefs.csv` file into the *plugin process's* working directory and the binary
reads it from the *user's*. It also leaves provisioning invisible to Terraform:
state has no notion of draft versus active, so `plan` can never say what will be
provisioned.

`illumio-core_provisioning` makes provisioning a resource.

## Basic usage

```hcl
provider "illumio-core" {
  # hrefs.csv is only consumed by the deprecated provision binary.
  write_href_file = false
}

resource "illumio-core_rule_set" "app" {
  name = "RS-APP"

  scopes {
    label {
      href = illumio-core_label.env.href
    }
  }
}

resource "illumio-core_provisioning" "policy" {
  hrefs = [illumio-core_rule_set.app.href]

  update_description = "Provisioned by Terraform"

  lifecycle {
    replace_triggered_by = [illumio-core_rule_set.app]
  }
}
```

`terraform apply` now provisions. Referencing `href` gives Terraform a real
dependency edge, so provisioning is always ordered after the objects are
created.

## `replace_triggered_by` is not optional

**A policy object's HREF does not change when its contents change.** At plan
time the object has not been updated in the PCE yet, so it is not yet in the
pending set, so no provisioning is planned. Without `replace_triggered_by`:

- apply #1 updates the rule set and leaves it **in draft**
- only the *next* plan notices and provisions it

That means a successful `terraform apply` can silently leave policy inactive.
`lifecycle.replace_triggered_by` makes Terraform plan the provisioning run in
the same apply as the change:

```hcl
lifecycle {
  replace_triggered_by = [
    illumio-core_rule_set.app,
    illumio-core_ip_list.external,
  ]
}
```

List every policy resource whose changes should trigger provisioning.
Replacement is destroy-then-create, and because delete is a no-op and create
provisions, replacement is exactly the desired behaviour.

## What `plan` shows

Provisioning state is visible in the plan for the first time. The resource
queries the pending policy set during planning and names the objects it will
provision:

```
  # illumio-core_provisioning.policy will be updated in-place
  ~ resource "illumio-core_provisioning" "policy" {
      ~ pending_hrefs = [
          + "/orgs/1/sec_policy/draft/rule_sets/3",
          + "/orgs/1/sec_policy/draft/ip_lists/7",
        ]
      ~ version       = 6 -> (known after apply)
    }
```

This also catches **drift**: if someone changes policy in the PCE UI or with
another tool, the next plan shows a provisioning change naming that object.

⚠️ Because of this, `terraform plan` requires working PCE credentials when the
resource is used.

## Provisioning everything pending

```hcl
resource "illumio-core_provisioning" "everything" {
  provision_all_pending = true
}
```

⚠️ **Unsafe on a shared PCE.** This activates *any* pending change, including a
half-finished policy another team or a UI user left in draft. Prefer an explicit
`hrefs` set. `hrefs` and `provision_all_pending` are mutually exclusive, and at
least one must be set.

## Nested objects

Security rules and deny rules provision through their **parent rule set**.
Reference the rule set:

```hcl
resource "illumio-core_provisioning" "policy" {
  hrefs = [illumio-core_rule_set.app.href]

  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.app,
      illumio-core_deny_rule.block_ssh,
      illumio-core_security_rule.allow_web,
    ]
  }
}
```

Passing a nested object's HREF in `hrefs` also works — it resolves to the parent
rule set automatically — but referencing the rule set is clearer.

## Destroying

Destroying `illumio-core_provisioning` makes **no API call**. It cannot: policy
provisioning is not reversible.

`DELETE /sec_policy/pending` reverts *every* pending draft change org-wide, and
`/sec_policy/{id}/restore` rolls the whole policy back to an earlier version.
Neither is an acceptable automatic consequence of destroying a bookkeeping
resource, so the resource never calls either. Removing it from state changes
nothing in the PCE.

### `terraform destroy` does not provision deletions

This is a real limitation, not an oversight.

`illumio-core_provisioning` depends on the objects it provisions, so Terraform
destroys it **first**, then the objects. Their deletions are therefore left
unprovisioned in draft, and the objects stay active in the PCE until something
provisions the deletion.

The upstream binary has the identical problem, which is why its documentation
says to run `provision` after a destroy. It is inherent to modelling a
post-apply side effect as a resource.

Workarounds, in order of preference:

1. Run the `provision` binary once after `terraform destroy`.
2. Apply a separate, minimal configuration containing only an
   `illumio-core_provisioning` resource with `provision_all_pending = true`.

The real fix is a post-destroy hook — Terraform 1.14's `action_trigger` — which
requires migrating the provider to terraform-plugin-framework.

## Attributes

| Attribute | Meaning |
|---|---|
| `version` | Version number of the policy created by the last provisioning |
| `version_href` | e.g. `/orgs/1/sec_policy/6` |
| `provisioned_at` | Timestamp of the last successful provisioning |
| `object_counts` | Object counts by type in the provisioned version |
| `workloads_affected` | Workloads affected by the last provisioning |
| `pending_hrefs` | Objects this resource owns that currently have pending changes |

## Migrating from the `provision` binary

1. Add an `illumio-core_provisioning` resource with `replace_triggered_by`.
2. Set `write_href_file = false` on the provider so `hrefs.csv` is no longer
   written and does not accumulate unconsumed entries.
3. Stop running `provision` after apply — except after `terraform destroy`,
   until the limitation above is resolved.

The binary still works and is unchanged in behaviour, apart from the
correctness fixes described in
[Differences from upstream](differences-from-upstream.html).

## Multiple provisioning resources

The PCE provisions a change subset atomically for the whole organization. Two
`illumio-core_provisioning` resources applied together create two policy
versions. That is safe, but produces more versions than expected — prefer one
provisioning resource per configuration.
