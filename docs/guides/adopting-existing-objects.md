---
page_title: "Adopting existing objects"
subcategory: ""
description: |-
  Managing labels, IP lists, label groups and rule sets that already exist on the PCE, without importing them.
---

# Adopting existing objects

Most PCE estates already have objects Terraform did not create — labels shared
across teams, IP lists created in the UI, rule sets from an earlier project.

Upstream, declaring one of those in Terraform **fails the apply**:

```
Error: 406 Not Acceptable: Client configuration error.
Message: [{"token":"label_key_and_value_must_be_unique","message":"Label key and value must be unique"}]
```

The PCE rejects the duplicate rather than returning the existing object, so the
only recourse was `terraform import` with a numeric HREF looked up in the UI —
for every object.

In this fork, Terraform **adopts** it instead.

## How it works

```hcl
resource "illumio-core_label" "role_web" {
  key   = "role"
  value = "web"
}
```

`terraform apply` tries to create the label. If the PCE says it already exists,
Terraform looks it up by the fields that make it unique, starts managing it, and
applies the rest of your configuration to it. No import, no HREFs.

State records that it was adopted:

```
adopted = true
href    = "/orgs/1/labels/17732923533262812"
```

## Which objects, and what identifies them

| Resource | Matched on | PCE token when duplicated |
|---|---|---|
| `illumio-core_label` | `key` + `value` | `label_key_and_value_must_be_unique` |
| `illumio-core_ip_list` | `name` | `ip_list_name_not_unique` |
| `illumio-core_label_group` | `name` | `name_must_be_unique` |
| `illumio-core_rule_set` | `name` | `rule_set_name_in_use` |

`illumio-core_service` is **not** in this list. The PCE permits duplicate
service names — creating the same service twice returns 201 both times — so
there is no unique key to adopt by, and matching on name could silently manage
the wrong object.

Matches must be **exact and unique**. The PCE matches `name` partially, so a
substring hit is filtered out. If more than one object matches, the apply fails
and names the candidates:

```
Error: 2 objects match name="web", so it is not clear which to adopt.
Import the intended one by HREF: /orgs/1/..., /orgs/1/...
```

## Destroying an adopted object

**Adopted objects are not deleted.** Destroying the resource removes it from
Terraform state and leaves it in the PCE, with a warning:

```
Warning: Adopted label removed from Terraform state, not deleted
/orgs/1/labels/17732923533262812 already existed when Terraform first managed
it, so it was adopted rather than created. It has been removed from state and
still exists in the PCE.
```

This matters because a label is very likely shared. Terraform deleting an object
it never created — possibly one another team depends on — is not a reasonable
consequence of removing a resource from a configuration.

Objects Terraform **did** create are deleted normally on destroy. The `adopted`
attribute is what distinguishes the two, so you can always see which you have:

```bash
terraform state show illumio-core_label.role_web | grep adopted
```

### Deliberately deleting an adopted object

Delete it in the PCE, or remove the `adopted` marker by recreating the resource
under Terraform's ownership. There is no flag to force deletion of an adopted
object: if you want it gone, deleting it where it lives is clearer than teaching
Terraform to do it.

## Adoption versus import

Both work; they answer different questions.

| | Adoption | Import |
|---|---|---|
| Trigger | `terraform apply` | `terraform import` or an `import` block |
| Identified by | The resource's own unique fields | Any supported identifier, or the HREF |
| Destroy | Leaves the object in place | Deletes it — import claims ownership |
| Best for | Declaring what should exist, regardless of what already does | Taking full ownership of a specific object |

If you want Terraform to **own** a pre-existing object — including deleting it on
destroy — import it rather than letting it be adopted. Import sets `adopted` to
its default of `false`.

## Interaction with provisioning

Adopted policy objects — IP lists, label groups, rule sets — are provisioned
exactly like created ones. Adoption changes ownership, not policy state. If the
object had pending changes before Terraform adopted it, those are still pending
and will be included the next time you provision.

See [Policy provisioning](policy-provisioning.html).

## Managed workloads

Managed workloads adopt too, but by a different route: they cannot be created at
all, so there is no create to collide. Set `hostname` and the workload is
adopted directly.

See [Managed workloads](managed-workloads.html).
