---
page_title: "Deny rules"
subcategory: ""
description: |-
  Managing Illumio deny rules and override-deny rules with Terraform, including actor and service constraints.
---

# Deny rules

A deny rule blocks the **consumers** (the sources that initiate the connection)
from reaching the **providers** (the destinations) on the given services.

Deny rules exist only below a rule set, at `{rule_set_href}/deny_rules`. They are
separate policy objects from security (allow) rules, with a different endpoint
and a different schema.

⚠️ **Availability.** The deny-rule endpoint does **not** appear in the 26.3
OpenAPI specification, and `deny_rules_get.schema.json` is flagged
`end_user_private_perm` — it is a private or unreleased PCE feature. It was
verified against a live PCE at 26.30.2, but will not work on a PCE where the
feature is not enabled. Expect a 404 from the endpoint if it is unavailable.

## Ordinary and override-deny rules

Both are the **same object at the same endpoint**, distinguished only by the
`override` flag. There is no separate `override_deny_rules` collection, and a
rule set exposes a single `deny_rules` array containing both kinds.

The policy evaluation order is:

```
override-deny  >  allow  >  deny
```

- `override = false` — an ordinary deny rule. An allow rule can supersede it.
- `override = true` — an override-deny rule. Allow rules cannot supersede it.

An override-deny rule **still blocks traffic**. It does not permit traffic that
would otherwise be denied.

## Basic usage

```hcl
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false
  description   = "Block external clients from internal SSH"

  # Destinations providing the service.
  providers {
    label {
      href = illumio-core_label.internal.href
    }
  }

  # Sources initiating the connection.
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

An override-deny rule is the same resource:

```hcl
resource "illumio-core_deny_rule" "block_rdp_unconditionally" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = true

  providers {
    actors = "ams"
  }

  consumers {
    label {
      href = illumio-core_label.contractors.href
    }
  }

  ingress_services {
    proto   = "6"
    port    = "3389"
    to_port = "3395"
  }
}
```

Toggling `override` updates the same object; it never moves the rule to a
different endpoint.

## Actors

Deny rules permit a **narrower** actor set than allow rules:

| Selector | Notes |
|---|---|
| `actors = "ams"` | All workloads |
| `label` | |
| `label_group` | |
| `ip_list` | |
| `workload` | |
| `exclusion` | A modifier, valid only on `label` and `label_group` |

`virtual_service` and `virtual_server` are **not valid** deny-rule actors, even
though they are valid on security rules. They are rejected at plan time:

```
Error: Unsupported block type
Blocks of type "virtual_service" are not expected here.
```

Exactly one selector per `providers` or `consumers` block. Two selectors, or
none, is an error.

### One actor per block

Each `providers` or `consumers` block holds exactly **one** actor. The PCE
stores actors as an array of single-actor entries, so several actors means
repeating the whole block:

```hcl
# Correct - two providers
providers {
  label {
    href = illumio-core_label.role_db.href
  }
}

providers {
  label {
    href = illumio-core_label.env_prod.href
  }
}
```

```hcl
# NOT valid - fails with "Too many label blocks"
providers {
  label {
    href = illumio-core_label.role_db.href
  }
  label {
    href = illumio-core_label.env_prod.href
  }
}
```

That produces:

```json
"providers": [
  {"label": {"href": "/orgs/1/labels/1"}},
  {"label": {"href": "/orgs/1/labels/2"}}
]
```

`illumio-core_security_rule` behaves the same way, so the two resources are
written identically.

## Services

### Ingress services

Either a service reference, or an inline protocol and port — never both in the
same block:

```hcl
ingress_services {
  href = illumio-core_service.ssh.href
}

ingress_services {
  proto = "6"      # TCP
  port  = "22"
}

ingress_services {
  proto   = "17"   # UDP
  port    = "3389"
  to_port = "3395"
}
```

`proto` accepts only `6` (TCP) and `17` (UDP).

### ICMP

**ICMP has no inline representation.** `proto = "1"` is rejected:

```
Error: expected proto to be one of ["6" "17"], got 1
```

Reference an ICMP service by HREF instead:

```hcl
ingress_services {
  href = data.illumio-core_service.icmp.href
}
```

### Egress services

Service references only — there is no inline port/protocol form:

```hcl
egress_services {
  href = illumio-core_service.dns.href
}
```

## Provisioning

Deny rules are provisioned as part of their **containing rule set**, not
individually. Reference the rule set, not the deny rule:

```hcl
resource "illumio-core_provisioning" "policy" {
  hrefs = [illumio-core_rule_set.app.href]

  lifecycle {
    replace_triggered_by = [illumio-core_deny_rule.block_ssh]
  }
}
```

See [Policy provisioning](policy-provisioning.html).

## Importing

Import by HREF:

```bash
terraform import illumio-core_deny_rule.block_ssh \
  "/orgs/1/sec_policy/draft/rule_sets/3/deny_rules/10"
```

`rule_set_href` is recovered from the HREF during import, so it does not need to
be supplied.

## Reading deny rules

```hcl
data "illumio-core_deny_rule" "one" {
  href = "/orgs/1/sec_policy/draft/rule_sets/3/deny_rules/10"
}

data "illumio-core_deny_rules" "all" {
  rule_set_href = illumio-core_rule_set.app.href
}

output "override_deny_rules" {
  value = [for r in data.illumio-core_deny_rules.all.items : r.href if r.override]
}
```

The plural data source requires `rule_set_href`: deny rules exist only below a
rule set, and there is no policy-version-wide collection to query.

## PCE version compatibility

`illumio-core_deny_rule` only sends the fields your configuration sets, so a
deny rule that uses no 26.2-specific feature works against an older PCE.

These attributes do not exist before PCE 26.2 and are omitted unless configured:

- `all_ips_except_for_in_consumers`
- `all_ips_except_for_in_providers`
- `unscoped_consumers`
- `network_type`

Setting any of them against a PCE older than 26.2 produces an error from the
PCE, because the field genuinely is not there. Leave them unset and the deny
rule is compatible.

⚠️ Provider versions **before 2.0.1** sent `all_ips_except_for_in_consumers`,
`all_ips_except_for_in_providers` and `unscoped_consumers` on every *update*
even when unset, so updating a deny rule failed on a 25.2 PCE.

## Fields that do not exist

Earlier iterations of this work invented several concepts that live PCE
validation disproved. They are not accepted by this resource, and should not be
added:

- `action` — deny rules are a separate object, not an allow rule with a mode
- `priority`
- `overrides`
- `resolve_labels_as`
- a separate `override_deny_rules` collection

Allow-only controls are likewise absent, because no schema supports them on a
deny rule: `sec_connect`, `stateless`, `machine_auth`,
`consuming_security_principals`, `use_workload_subnets`.

`name`, `external_data_set` and `external_data_reference` are not present on a
deny rule returned by the PCE, so they are not configurable either.
