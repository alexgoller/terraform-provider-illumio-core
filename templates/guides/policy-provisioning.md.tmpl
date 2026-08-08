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

**A policy object's HREF does not change when its contents change.** Editing a
rule, an IP list's ranges or a service's ports updates the object in place, so
the `hrefs` set is byte-for-byte identical and the provisioning resource has no
reason to plan anything. At plan time the object has not been updated in the PCE
yet either, so it is not in the pending set.

The result is that a successful `terraform apply` silently leaves policy
inactive:

```
Apply complete! Resources: 0 added, 1 changed, 0 destroyed.
```

with the object still in draft, and nothing in the output saying so.

`lifecycle.replace_triggered_by` fixes this:

```hcl
lifecycle {
  replace_triggered_by = [
    illumio-core_rule_set.app,
    illumio-core_ip_list.external,
  ]
}
```

Replacement is destroy-then-create, and because delete is a no-op and create
provisions, replacement is exactly the desired behaviour.

### The rule: `hrefs` is not enough on its own

**Every object named in `hrefs` must also appear in `replace_triggered_by`,
along with the rules, which have no href of their own in `hrefs`.**

An object listed only in `hrefs` is provisioned when it is first created and
then never again. This is easy to miss because creation works perfectly — the
gap only opens on the first edit.

The two lists do different jobs:

| | Purpose |
|---|---|
| `hrefs` | *What* to provision — the change subset sent to the PCE |
| `replace_triggered_by` | *When* to provision — what makes the resource run again |

### It also fixes ordering, which is easy to overlook

Referencing a rule set's `href` orders provisioning after the **rule set**, but
the rules inside it are siblings of the provisioning resource, not ancestors:

```
illumio-core_rule_set.app
   ├── illumio-core_security_rule.web_to_db     (via rule_set_href)
   └── illumio-core_provisioning.policy         (via hrefs)
```

Nothing there orders the two branches, so provisioning could run before the rule
is written. Listing the rule in `replace_triggered_by` adds the missing edge.
Terraform's own documentation describes `replace_triggered_by` in terms of
planned actions and does not mention ordering, but the graph shows the edge:

```
$ terraform graph        # rule listed in replace_triggered_by
"illumio-core_provisioning.policy" -> "illumio-core_security_rule.ringfence";

$ terraform graph        # rule omitted
"illumio-core_provisioning.policy" -> "illumio-core_rule_set.rs";
"illumio-core_provisioning.policy" -> "illumio-core_service.https";
```

So `replace_triggered_by` supplies both the trigger and the ordering. No
`depends_on` is needed.

### Verified against a live PCE

All three behaviours were confirmed on a 25.2 PCE, not inferred:

| Configuration | Change made | Result |
|---|---|---|
| Rule in `replace_triggered_by` | create everything | one apply, nothing left pending, rule active |
| Rule **omitted** | edit the rule's description | `0 added, 1 changed`, provisioning never ran, **rule set left in draft** |
| Rule restored | edit again | provisioning destroyed and recreated in the same apply, pending cleared |
| IP list in `hrefs` only | edit its description | provisioning never ran, **IP list left in draft** |

The last row is the one that catches people: it is not specific to rules. Any
provisionable object edited in place behaves this way.

## Objects deleted outside Terraform

Deleting a policy object in the PCE UI does **not** remove it if the object has
already been provisioned. The PCE marks the draft copy `update_type: delete` and
leaves the active copy in place until someone provisions the deletion:

```
DELETE /orgs/1/sec_policy/draft/ip_lists/152   -> 204
GET    /orgs/1/sec_policy/draft/ip_lists/152   -> 200   update_type: delete
GET    /orgs/1/sec_policy/active/ip_lists/152  -> 200
```

Nothing returns 404, so the ordinary drift path — read, get 404, drop from state,
recreate on the next apply — never triggers.

### What the provider does

Every provisionable resource carries a computed `pending_deletion` attribute and
warns when it is set:

```
Warning: [illumio-core_ip_list] /orgs/1/sec_policy/draft/ip_lists/159 is marked
for deletion in the PCE
```

`pending_deletion` is readable from `terraform show -json`, which is what a
pipeline should assert on:

```bash
terraform show -json \
  | jq -e '[.values.root_module.resources[]
            | select(.values.pending_deletion == true)] | length == 0'
```

The warning appears on `terraform plan` and `terraform refresh`. It does **not**
appear on an `apply` that has no changes to make, because that exits before
reporting refresh diagnostics — so do not rely on apply output alone.

### Why Terraform cannot simply fix it

Both obvious repairs are unavailable:

- **Cancelling the deletion.** There is no per-object way to do it. A `PUT` of the
  original body returns `204` and leaves `update_type` at `delete`.
  `DELETE /sec_policy/pending` reverts every pending change in the organization,
  and `POST /sec_policy/{version}/restore` rolls back a whole policy version —
  both would discard other people's work on a shared PCE.
- **Recreating it.** Dropping the resource from state so the next apply recreates
  it does not work, because names must be unique and the original still exists:

  ```
  POST /orgs/1/sec_policy/draft/ip_lists  -> 406
  [{"token": "ip_list_name_not_unique", "message": "IP List name must be unique"}]
  ```

  That would turn a silent drift into a failing apply.

So the object's HREF cannot be preserved. Once the deletion is provisioned the
object is gone, and its replacement necessarily gets a new HREF. The provider
reports the situation and leaves the decision to a human.

### Recovering

1. Decide whether the deletion was intended. If it was, remove the resource from
   the configuration and run `terraform apply`.
2. If it was not, provision the pending deletion — scoped to that object, never
   `provision_all_pending` — and then `terraform apply` to recreate it.

The replacement has a **new HREF**, and anything referencing it is updated in the
same apply. Judge the blast radius before running it:

| Deleted object | What recreating it costs |
|---|---|
| IP list, service, label group | the object itself, plus a diff on every rule referencing it |
| Rule set | **its rules are recreated too**, since they are children of the rule set |
| Label | every workload and scope referencing it is re-pointed |

### Catching it reliably

`terraform plan` warns only about objects already in state, and only when it is
run. On a shared PCE the reliable detector is the pending set itself:

```bash
curl -s -u "$KEY:$SECRET" "$PCE/api/v2/orgs/1/sec_policy/pending" \
  | jq '[.. | objects | select(.href) | .href]'
```

Run that on a schedule, filter to the HREFs Terraform manages, and alert on
anything marked for deletion that your configuration still declares. This catches
deletions between Terraform runs, which a plan-time warning cannot.

### Preventing it

The durable fix is authorization rather than detection. Scoped PCE roles —
`limited_ruleset_manager` bound to an application's labels — stop people deleting
inside label scopes that Terraform owns. That does not constrain an
organization-wide admin, but it removes the common accident, which is someone
tidying up in a scope that is not theirs.

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

### `terraform destroy` needs two passes

This is a real limitation, not an oversight, and it is more disruptive than
simply leaving objects in draft.

`illumio-core_provisioning` depends on the objects it provisions, so Terraform
destroys it **first**, then the objects. Their deletions are therefore left
unprovisioned in draft — and anything the still-active policy references cannot
be deleted at all. `terraform destroy` **fails partway**:

```
Error: 406 Not Acceptable
[{"token":"label_still_has_associated_rule_set",
  "message":"Label still has an associated rule set"}]
```

Labels, IP lists and services used by a provisioned rule set are all affected:
the rule set is gone from draft but still live, so the PCE correctly refuses to
remove what it depends on. You are left with a partial teardown.

The refusal comes in more than one form, depending on what holds the reference:

```
label_still_has_associated_rule_set    a rule set scope holds the label
label_referenced_by_label_group        a label group holds the label
```

Both mean the same thing — Terraform's view and the **active** policy have
diverged, and the PCE is enforcing referential integrity against what is live.
It is not only rule sets that pin an object.

The working sequence is:

```bash
# 1. Removes what it can. Policy objects go to draft; anything the active
#    policy still references fails with a 406.
terraform destroy

# 2. Provision the pending deletions, so the rule set actually goes away.
#    Use explicit hrefs — never provision_all_pending on a shared PCE.
curl -u "$KEY:$SECRET" -X POST -H "Content-Type: application/json" \
  -d '{"update_description":"teardown","change_subset":{"rule_sets":[{"href":"..."}]}}' \
  "https://pce:8443/api/v2/orgs/1/sec_policy"

# 3. Now the dependants can go.
terraform destroy
```

Step 2 can also be the deprecated `provision` binary, or a separate minimal
configuration containing only an `illumio-core_provisioning` resource.

Capture the HREFs **before** step 1 — `terraform show -json` no longer has them
once the resources leave state.

Observed on a live PCE: a 17-resource configuration destroyed 13 objects, failed
on 4 labels, and completed after the pending deletions were provisioned.

The upstream binary has the identical problem, which is why its documentation
says to run `provision` after a destroy. It is inherent to modelling a
post-apply side effect as a resource: nothing is left to activate the deletions
once the provisioning resource itself is gone.

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
