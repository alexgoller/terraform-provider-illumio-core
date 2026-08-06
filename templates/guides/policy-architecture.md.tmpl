---
page_title: "Organizing policy"
subcategory: ""
description: |-
  Structuring Illumio policy as three pillars - infrastructure services, ransomware containment and application policy - so that many teams can contribute to one PCE safely.
---

# Organizing policy

A single PCE serving a whole organization has one hard problem: many teams must
be able to change policy without being able to break each other. This guide sets
out a structure that solves it, built on three pillars:

| Pillar | Protects | Typically owned by |
|---|---|---|
| **Infrastructure services** | the flows every workload needs — DNS, NTP, directory, backup, monitoring, patching | platform team |
| **Ransomware containment** | the whole estate, by closing the paths lateral movement uses | security team |
| **Application policy** | one application in one environment | the application's team |

The pillars are not just a filing convention. Each maps onto a different level of
the PCE's rule precedence, so the hierarchy between them is enforced by the
policy engine rather than by agreement.

## Precedence is the org chart

Illumio evaluates policy in this order:

```
override-deny  >  allow  >  deny
```

Map the pillars onto that ladder:

```
override-deny   ←  Ransomware containment    security; nobody can punch through
      ▲
    allow       ←  Infrastructure services   platform
                ←  Application policy        application teams
      ▲
     deny       ←  soft guardrails           security; a team may override with an allow
```

Three consequences follow, and they are what let the structure hold together
without constant coordination:

1. **Containment cannot be undone from below.** An application team cannot write
   an allow rule that defeats an override-deny, however it is scoped. The
   containment layer is a floor, not a default.
2. **Infrastructure and application rules never conflict.** Both are allow rules,
   and allow rules are additive — the union is the result. Two teams adding rules
   on the same day cannot produce a contradiction.
3. **Security gets two strengths of guardrail.** `override = true` is absolute.
   `override = false` is a discouraged-but-possible default: a team that
   genuinely needs the flow writes an allow rule, and that allow appears in a
   pull request diff. The exception process documents itself.

## The scope contract

Where an application is identified by the tuple `(app, env)`, that tuple is the
contract between every pillar. It appears in a rule set scope as two labels in
**one** `scopes` block:

```hcl
resource "illumio-core_rule_set" "sap_prod" {
  name = "sap | prod"

  scopes {
    label { href = local.app_href }
    label { href = local.env_href }
  }
}
```

Labels combine as **OR within a label type, AND across types**. Two labels of
different types in one block therefore select the *intersection*: workloads that
are both `app=sap` and `env=prod`.

~> **A scope missing its `env` label is the dangerous mistake.** It does not
fail; it silently widens the rule set to `app=sap` in *every* environment, so a
change meant for development reaches production. Because it is a widening rather
than an error, neither `plan` nor the PCE will object. Do not rely on review to
catch it — make it structurally impossible by giving application teams a module
whose `app` and `env` variables are both required, so a scope cannot be written
without both.

## Looking labels up: use exact matching

Cross-pillar references go through data sources, and this is where the second
silent-widening trap lives.

The singular data sources — `illumio-core_label`, `illumio-core_service`,
`illumio-core_label_group`, `illumio-core_ip_list` — take **`href` only**. Their
`key`, `value` and `name` attributes are computed outputs, not lookup filters. To
find an object by name you need the plural data source.

!> **`illumio-core_labels` defaults to `match_type = "partial"`.** A filter for
`value = "prod"` also matches `non-prod`, `preprod` and `production`. Since the
result is a list you then index into, the wrong environment can end up in a
production scope with nothing reporting an error. **Always set
`match_type = "exact"`.**

```hcl
data "illumio-core_labels" "env" {
  key        = "env"
  value      = var.env
  match_type = "exact"
}

locals {
  env_href = one(data.illumio-core_labels.env.items[*].href)
}
```

`one()` is doing real work: it returns the single element, and **errors if the
filter matched more than one**. That converts an ambiguous lookup into a failed
plan instead of an arbitrary `items[0]`.

`illumio-core_services`, `illumio-core_label_groups` and `illumio-core_ip_lists`
have **no `match_type` argument** — their `name` filter is always a partial
match. Pin those with an explicit comparison:

```hcl
data "illumio-core_services" "smb" {
  name = "SMB"
}

locals {
  smb_href = one([
    for s in data.illumio-core_services.smb.items : s.href if s.name == "SMB"
  ])
}
```

Make this the house pattern. Every cross-pillar lookup should end in `one()`, so
a renamed or duplicated object fails the plan rather than resolving to something
plausible.

## Repository layout

One repository, one state per pillar, and one state per `(app, env)` pair. The
numeric prefixes carry both the dependency order and the precedence order:

```
illumio-policy/
├── CODEOWNERS
├── modules/
│   ├── application/          rule set with an (app, env) scope + ring-fence
│   ├── app-flow/             one application to another, on a named service
│   └── containment-rule/     override-deny with its exemption group
├── 00-foundation/            labels, label groups, services, IP lists   @platform
├── 10-containment/           override-denies, enforcement boundaries    @security
├── 20-infrastructure/        DNS, NTP, directory, backup, monitoring    @platform
└── 30-applications/
    ├── sap/
    │   ├── prod/                                                        @team-erp
    │   └── dev/                                                         @team-erp
    └── payments/
        ├── prod/                                                        @team-payments
        └── dev/                                                         @team-payments
```

`CODEOWNERS` turns each directory into a review gate:

```
/00-foundation/       @platform
/10-containment/      @security
/20-infrastructure/   @platform
/30-applications/sap/ @team-erp
```

Each `(app, env)` pair gets its own directory and its own state rather than
sharing one per application. The blast radius of a mistaken apply is then a
single environment of a single application.

## Why separate states are safe

Each root module contains exactly one `illumio-core_provisioning` resource
listing **only the hrefs it owns**:

```hcl
resource "illumio-core_provisioning" "sap_prod" {
  hrefs = [module.sap_prod.rule_set_href]

  update_description = "sap prod"

  lifecycle {
    replace_triggered_by = [module.sap_prod.rule_set]
  }
}
```

Provisioning sends a `change_subset` naming those hrefs, so activating one team's
policy leaves every other team's pending draft untouched. That is what makes
concurrent applies by independent teams safe, and it is why the pillars can be
separate states at all.

!> **Never set `provision_all_pending = true` on a shared PCE.** It activates
every pending change, including another team's half-finished draft that was never
reviewed. In a multi-team setup this is the one setting that turns a routine
apply into an incident. Reserve it for a PCE with a single owner.

See the [policy provisioning](policy-provisioning.html) guide for the resource
itself.

## Labels are the published interface

The foundation layer owns the label taxonomy. Every other layer reads it through
data sources rather than remote state outputs. Data sources keep the coupling
loose: teams apply on their own schedule, no state is shared, and no team can
take a lock that blocks another.

The cost is that a label's key and value become a published contract — renaming
one breaks every consumer. Treat the taxonomy as an API, and change it by adding
rather than renaming.

**Application labels need not bottleneck on the platform team.** Because
resources adopt on create, an application module can declare its own `app` label
and adopt the existing one if the PCE already has it, so a new application
onboards without waiting on a foundation change. The `env` and `loc` dimensions
stay platform-owned, because their values are finite and shared. See
[adopting existing objects](adopting-existing-objects.html).

## The application module

Application teams should not write scopes by hand. Give them a module that takes
the tuple and produces a correctly scoped rule set with a ring-fence:

```hcl
# modules/application/main.tf
variable "app" { type = string } # required — no default
variable "env" { type = string } # required — no default

data "illumio-core_labels" "app" {
  key        = "app"
  value      = var.app
  match_type = "exact"
}

data "illumio-core_labels" "env" {
  key        = "env"
  value      = var.env
  match_type = "exact"
}

locals {
  app_href = one(data.illumio-core_labels.app.items[*].href)
  env_href = one(data.illumio-core_labels.env.items[*].href)
}

resource "illumio-core_rule_set" "this" {
  name    = "${var.app} | ${var.env}"
  enabled = true

  scopes {
    label { href = local.app_href }
    label { href = local.env_href }
  }
}

# Ring-fence: everything inside the scope may talk to everything else inside it.
resource "illumio-core_security_rule" "ringfence" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "ring-fence ${var.app} ${var.env}"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = var.intra_app_service_href }
}
```

Neither variable has a default, so a scope cannot be written without both halves
of the tuple. The team's own root module then reads:

```hcl
module "sap_prod" {
  source = "../../../modules/application"
  app    = "sap"
  env    = "prod"
}
```

That is the whole surface an application team needs for a ring-fenced
application. Cross-application flows go through a second module, so every
inter-application dependency is an explicit, reviewable module call rather than a
hand-written rule.

## Containment owns its own exceptions

Because an override-deny cannot be superseded from below, any flow that must
cross the containment layer has to be granted **by** the containment layer. Deny
rules support `exclusion` on label and label group actors, so containment carries
its own exemption list.

An estate-wide rule set uses a `scopes` block with no labels in it, which is the
PCE's "All | All | All" global scope. The block itself is required — omitting it
is a validation error:

```hcl
# 10-containment/ — @security
resource "illumio-core_rule_set" "containment" {
  name        = "containment | fleet"
  description = "estate-wide lateral movement controls"
  enabled     = true

  scopes {} # no labels: applies across the estate
}

resource "illumio-core_deny_rule" "no_smb" {
  rule_set_href = illumio-core_rule_set.containment.href
  enabled       = true
  override      = true
  description   = "SMB is not a peer-to-peer protocol"

  providers {
    label_group {
      href      = illumio-core_label_group.smb_servers.href
      exclusion = true
    }
  }

  consumers { actors = "ams" }

  ingress_services { href = local.smb_href }
}
```

Every workload is denied SMB except members of the `smb_servers` label group,
which security owns. A backup team that needs SMB opens a pull request against
`10-containment/` adding its label to that group, and `CODEOWNERS` routes the
review to security automatically.

This is the structural payoff: the request path for a containment exception is
enforced by the precedence engine, not by a wiki page telling people to ask.

Note that `exclusion` is valid only for `label` and `label_group` actors — not
for workloads or IP lists. Build exemption lists out of label groups.

## Who provisions

Two workable models. Splitting by pillar is the better default:

| | Applications | Infrastructure and containment |
|---|---|---|
| **Recommended** | provision themselves | security-owned provisioning step |

An application's policy is confined to its own `(app, env)` scope, so the blast
radius of a bad apply is bounded and immediate feedback is worth more than a
second gate. Containment and infrastructure reach the whole fleet, so their root
modules create draft policy only and a security-owned pipeline runs the
provisioning — turning the PCE's draft/active split into a real approval gate.

If you would rather have one model everywhere, let every pillar provision itself
and rely on pull request review. What you must not do is mix the two silently: a
team that believes its apply took effect, when in fact it produced a draft
awaiting someone else, will debug the wrong thing.

## Aligning PCE permissions with the repository

Directory ownership and PCE authorization should agree, so that bypassing the
repository does not bypass the boundary:

| Directory | CODEOWNERS | PCE role |
|---|---|---|
| `30-applications/sap/` | `@team-erp` | `limited_ruleset_manager`, scoped to `app=sap` |
| `20-infrastructure/` | `@platform` | `ruleset_manager` |
| `10-containment/` | `@security` | `ruleset_manager` + `ruleset_provisioner` |
| `00-foundation/` | `@platform` | `ruleset_manager` |

Scoped PCE roles mean an application team cannot write policy outside its own
labels even by using the API directly. Git review and PCE authorization then
reinforce each other rather than duplicating each other.

## Rolling it out

Order matters, because containment can break things infrastructure has not yet
permitted.

1. **Foundation first.** Declare the label taxonomy, services and IP lists,
   adopting what already exists on the PCE.
2. **Infrastructure allows next.** Get DNS, NTP, directory, backup, monitoring
   and patching expressed before anything starts blocking.
3. **Containment in visibility first.** Write the containment rules, but keep
   workloads in `visibility_only` while you read the traffic they *would* have
   blocked. The enforcement mode is Terraform-managed:

   ```hcl
   resource "illumio-core_managed_workload" "example" {
     enforcement_mode = "visibility_only" # then "selective", then "full"
   }
   ```

4. **Move to enforcement one group at a time**, by moving a label's worth of
   workloads from `visibility_only` to `selective` and then `full`.
5. **Applications last**, one `(app, env)` at a time, each starting from the
   ring-fence module and adding cross-application flows as they are discovered.

Steps 3 and 4 are where an estate-wide programme usually succeeds or fails.
Containment written directly into `full` enforcement will block something nobody
predicted.

## Anti-patterns

| Don't | Why |
|---|---|
| `provision_all_pending = true` on a shared PCE | Activates other teams' unreviewed drafts |
| One state for all policy | Every team's apply blocks every other; blast radius is the estate |
| A rule set scope with `app` but no `env` | Silently widens across every environment, and nothing reports it |
| A label lookup without `match_type = "exact"` | `prod` also matches `non-prod` and `preprod` |
| Indexing `items[0]` instead of using `one()` | An ambiguous lookup resolves silently instead of failing the plan |
| Application teams writing scopes by hand | The tuple contract holds only if a module enforces it |
| Granting containment exceptions with an allow rule | Cannot work against `override = true`; the exception belongs in the containment layer |
| Passing hrefs between pillars via remote state | Couples team schedules and creates cross-team lock contention; use data sources |
| Renaming a label value | The taxonomy is a published interface; add, don't rename |

## See also

- [Deny rules](deny-rules.html) — `override`, the actor set, and exclusions
- [Policy provisioning](policy-provisioning.html) — `illumio-core_provisioning`
- [Adopting existing objects](adopting-existing-objects.html) — brownfield onboarding
- [Managed workloads](managed-workloads.html) — enforcement mode and destroy semantics
