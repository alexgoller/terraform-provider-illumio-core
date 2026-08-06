---
page_title: "Organizing policy"
subcategory: ""
description: |-
  A best-practice layout for Illumio policy across many teams - scope-per-directory ownership, provider-centric cross-scope rules, and three pillars of policy - expressed with this provider.
---

# Organizing policy

A single PCE serving a whole organization has one hard problem: many teams must
be able to change policy without being able to break each other.

The conventions below are not invented here. They follow the model proven in
[illumio-policy-gitops](https://github.com/alexgoller/illumio-policy-gitops),
which manages Illumio policy as reviewed YAML in Git, and restates that model in
Terraform. If you are already using that repository, the directory names and
ownership boundaries here match it deliberately, so that policy compiled from
YAML lands in the same shape as policy written by hand.

## The scope is the unit of ownership

An application is identified by the tuple `(app, env)`. That tuple is the unit of
everything: one directory, one state, one team, one rule set, one blast radius.

```
scopes/app-payments_env-prod/
```

Naming the directory after the labels rather than after the team means ownership
can change without the policy moving.

Labels combine as **OR within a label type, AND across types**, so the two labels
in one `scopes` block select the intersection — workloads that are both
`app=payments` and `env=prod`.

~> **A scope missing its `env` label is the dangerous mistake.** It does not
fail; it silently widens the rule set to `app=payments` in *every* environment,
so a change meant for development reaches production. Neither `plan` nor the PCE
will object, because it is a widening rather than an error. Do not rely on review
to catch it — give teams a module whose `app` and `env` variables are both
required, so a scope cannot be written without both.

## Three pillars

Policy divides into three kinds with different owners and different review
weight. They are not three mechanisms — they are three kinds of file:

| Pillar | Where | Owner | Rule kind |
|---|---|---|---|
| **Ransomware containment** | `scopes/_global/containment.tf` | security | override-deny, estate-wide |
| **Infrastructure services** | `scopes/_global/baseline.tf`, and `inbound.tf` in each service's own scope | platform / service owner | allow |
| **Application** | `scopes/app-X_env-Y/` | application team | allow |

These compose because the PCE ranks them. Policy is evaluated as:

```
override-deny  >  allow  >  deny
```

Three consequences follow, and they are what let the pillars coexist without
constant coordination:

1. **Containment cannot be undone from below.** No application team can write an
   allow rule that defeats an override-deny, however it is scoped. Containment is
   a floor, not a default.
2. **Infrastructure and application rules never conflict.** Both are allow rules,
   and allow rules are additive — the union is the result. Two teams adding rules
   on the same day cannot produce a contradiction.
3. **Security gets two strengths of guardrail.** `override = true` is absolute.
   `override = false` is a discouraged-but-possible default: a team that needs
   the flow writes an allow rule, and that allow appears in a pull request diff.
   The exception process documents itself.

## Layout

```
illumio-policy/
├── CODEOWNERS
├── modules/
│   ├── scope/                          rule set with an (app, env) scope + ring-fence
│   └── containment-rule/               override-deny with its exemption group
├── foundation/                         labels, label groups          @platform
├── services/                           reusable service definitions  @security
├── ip-lists/                           reusable IP lists             @security
└── scopes/
    ├── _global/
    │   ├── baseline.tf                 DNS, NTP — universal allows   @platform
    │   └── containment.tf              estate-wide override-denies   @security
    ├── app-ad_env-prod/
    │   ├── main.tf                     scope + intra-scope rules     @ad-team
    │   └── inbound.tf                  who may consume AD            @ad-team + @security
    └── app-payments_env-prod/
        └── main.tf                     scope + intra-scope rules     @payments-team
```

One state per directory under `scopes/`. `CODEOWNERS` turns each into a review
gate:

```
/foundation/                      @org/platform-team
/services/                        @org/security-team
/ip-lists/                        @org/security-team
/scopes/_global/                  @org/security-team
/scopes/app-ad_env-prod/          @org/ad-team
/scopes/app-payments_env-prod/    @org/payments-team

# Every dependency on another team's scope is reviewed by security as well.
/scopes/*/inbound.tf              @org/security-team
```

## The scope module

Application teams should not write scopes by hand:

```hcl
# modules/scope/main.tf
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
```

Neither variable has a default, so a scope cannot exist without both halves of
the tuple. A team's own root module is then a single call:

```hcl
module "scope" {
  source = "../../modules/scope"
  app    = "payments"
  env    = "prod"
}
```

Intra-scope rules — the ones wholly inside the team's own blast radius — live
beside it in `main.tf` and need no outside review. A ring-fence is the usual
starting point:

```hcl
resource "illumio-core_security_rule" "ringfence" {
  rule_set_href = module.scope.rule_set_href
  enabled       = true
  description   = "ring-fence: intra-scope"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = local.https_href }
}
```

## Cross-scope rules are provider-centric, and live in exactly one file

This is the most important convention here, and the one most often got wrong.

A rule protects the **providers** — the side being reached. So a dependency of
payments on Active Directory is not payments' policy; it is a change to what AD
accepts. It therefore belongs in **AD's** directory, in `inbound.tf`, owned by
the AD team, with security as an additional reviewer.

```hcl
# scopes/app-ad_env-prod/inbound.tf
# Every dependency ON this scope is declared here. Owned by @ad-team.
#
# payments → AD: Kerberos and LDAP
# Requested by alice@example.com on 2026-04-15.
# Payment processing authenticates service accounts against AD and validates
# group membership for PCI-DSS role enforcement. LDAPS (636) is required;
# plaintext LDAP (389) is for migration discovery only and will be removed.
resource "illumio-core_security_rule" "from_payments_kerberos" {
  rule_set_href      = module.scope.rule_set_href
  enabled            = true
  unscoped_consumers = true # extra-scope: consumers lie outside this scope
  description        = "payments → AD: Kerberos"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers {
    label { href = local.role_dc_href }
  }

  consumers {
    label { href = local.app_payments_href }
  }

  consumers {
    label { href = local.role_processing_href }
  }

  ingress_services { href = local.kerberos_href }
}
```

`unscoped_consumers = true` is what makes it an extra-scope rule: the consumers
are matched outside the rule set's own scope. Without it the rule silently does
nothing, because no workload inside `app=ad, env=prod` is also `app=payments`.

The two `consumers` blocks are deliberate. Each block holds exactly one actor,
and because the two labels are of different types they combine with AND — the
consumer is a workload that is both `app=payments` and `role=processing`.

**Declare the dependency once.** It is tempting to mirror it in the requesting
team's directory so that payments can see its own outbound dependencies. Resist
it: two files describing one rule is two things to keep in sync, and the copy
inevitably drifts or gets edited in place — at which point it is unclear which
one the PCE reflects. The requester is the pull request author, not the file
owner; that is enough involvement.

Outbound dependencies stay discoverable without a second copy:

```bash
# What does payments depend on?
grep -rl "app_payments_href" scopes/*/inbound.tf
```

Because the justification, requester and date live in the rule's comment and
`description`, the audit trail travels with the rule rather than with a mirror
of it.

## Global scope: two files, not one

An estate-wide rule set uses a `scopes` block with no labels in it, which is the
PCE's "All | All | All" global scope. The block itself is required — omitting it
is a validation error:

```hcl
resource "illumio-core_rule_set" "global" {
  name    = "global | containment"
  enabled = true

  scopes {} # no labels: applies across the estate
}
```

Keep the **baseline** (universal allows: DNS, NTP) and the **containment**
(estate-wide denies) in separate files even though both are unscoped. They have
opposite intent and different review weight, and mixing them makes it hard to
answer "what does this organization block?" at a glance.

### Containment owns its own exceptions

Because an override-deny cannot be superseded from below, any flow that must
cross the containment layer has to be granted **by** the containment layer. Deny
rules support `exclusion` on label and label group actors, so containment carries
its own exemption list:

```hcl
# scopes/_global/containment.tf — @security
resource "illumio-core_deny_rule" "no_smb" {
  rule_set_href = illumio-core_rule_set.global.href
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
`scopes/_global/` adding its label to that group, and `CODEOWNERS` routes the
review to security automatically.

This is the structural payoff: the request path for a containment exception is
enforced by the precedence engine, not by a wiki page telling people to ask.

Note that `exclusion` is valid only for `label` and `label_group` actors — not
for workloads or IP lists. Build exemption lists out of label groups.

### Where infrastructure services go

Split them by whether access needs a decision:

- **Universal and uncontroversial** — DNS, NTP: one rule in
  `scopes/_global/baseline.tf`. Writing an `inbound.tf` entry per consuming
  application would mean dozens of files all saying the same thing.
- **Controlled** — Active Directory, backup, monitoring, jump hosts: give the
  service its own scope and let it own `inbound.tf`, so its team approves each
  consumer individually.

The dividing question is whether the service's owner would ever say no. If they
would, it belongs in a scope with an `inbound.tf`.

## Looking things up: exact matching

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

`one()` is doing real work: it returns the single element and **errors if the
filter matched more than one**, converting an ambiguous lookup into a failed plan
rather than an arbitrary `items[0]`.

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

Make this the house pattern: every cross-directory lookup ends in `one()`, so a
renamed or duplicated object fails the plan instead of resolving to something
plausible.

## Why separate states are safe

Each directory contains exactly one `illumio-core_provisioning` resource listing
**only the hrefs it owns**:

```hcl
resource "illumio-core_provisioning" "scope" {
  hrefs = [module.scope.rule_set_href]

  update_description = "payments prod"

  lifecycle {
    replace_triggered_by = [module.scope.rule_set]
  }
}
```

Provisioning sends a `change_subset` naming those hrefs, so activating one team's
policy leaves every other team's pending draft untouched. That is what makes
concurrent applies by independent teams safe, and it is why scopes can be
separate states at all.

!> **Never set `provision_all_pending = true` on a shared PCE.** It activates
every pending change, including another team's half-finished draft that was never
reviewed. In a multi-team setup this is the one setting that turns a routine
apply into an incident. Reserve it for a PCE with a single owner.

See the [policy provisioning](policy-provisioning.html) guide for the resource
itself.

## Who provisions

An application's policy is confined to its own `(app, env)` scope, so its
directory can provision itself: the blast radius is bounded and fast feedback is
worth more than a second gate.

`scopes/_global/`, `services/` and `ip-lists/` reach the whole fleet. Have those
directories create draft policy only and let a security-owned pipeline provision
them, which turns the PCE's draft/active split into a real approval gate.

What you must not do is mix the two silently. A team that believes its apply took
effect, when in fact it produced a draft awaiting someone else, will debug the
wrong thing.

## Aligning PCE permissions with the repository

Directory ownership and PCE authorization should agree, so that bypassing the
repository does not bypass the boundary:

| Directory | CODEOWNERS | PCE role |
|---|---|---|
| `scopes/app-payments_env-prod/` | `@payments-team` | `limited_ruleset_manager`, scoped to `app=payments` |
| `scopes/app-ad_env-prod/` | `@ad-team` | `limited_ruleset_manager`, scoped to `app=ad` |
| `scopes/_global/` | `@security-team` | `ruleset_manager` + `ruleset_provisioner` |
| `foundation/` | `@platform-team` | `ruleset_manager` |

Scoped PCE roles mean an application team cannot write policy outside its own
labels even by using the API directly. Git review and PCE authorization then
reinforce each other rather than duplicating each other.

## Rolling it out

Order matters, because containment can break things infrastructure has not yet
permitted.

1. **Foundation first.** Declare the label taxonomy, services and IP lists,
   adopting what already exists on the PCE. See
   [adopting existing objects](adopting-existing-objects.html).
2. **Baseline allows next.** Get DNS, NTP, directory, backup, monitoring and
   patching expressed before anything starts blocking.
3. **Containment in visibility first.** Write the containment rules, but keep
   workloads in `visibility_only` while you read the traffic they *would* have
   blocked:

   ```hcl
   resource "illumio-core_managed_workload" "example" {
     enforcement_mode = "visibility_only" # then "selective", then "full"
   }
   ```

4. **Move to enforcement one group at a time**, promoting a label's worth of
   workloads from `visibility_only` to `selective` and then `full`.
5. **Scopes last**, one `(app, env)` at a time, each starting from the ring-fence
   and adding `inbound.tf` entries as dependencies are discovered.

Steps 3 and 4 are where an estate-wide programme usually succeeds or fails.
Containment written directly into `full` enforcement will block something nobody
predicted.

## Anti-patterns

| Don't | Why |
|---|---|
| Mirror a cross-scope rule in the requester's directory | Two files for one rule drift, and it becomes unclear which the PCE reflects |
| Author a cross-scope rule in the requesting team's scope | The rule protects the provider; the provider's owner must be able to refuse it |
| Forget `unscoped_consumers = true` on an extra-scope rule | The rule matches nothing and fails silently |
| `provision_all_pending = true` on a shared PCE | Activates other teams' unreviewed drafts |
| One state for all policy | Every team's apply blocks every other; blast radius is the estate |
| A scope with `app` but no `env` | Silently widens across every environment, and nothing reports it |
| A label lookup without `match_type = "exact"` | `prod` also matches `non-prod` and `preprod` |
| Indexing `items[0]` instead of using `one()` | An ambiguous lookup resolves silently instead of failing the plan |
| Granting a containment exception with an allow rule | Cannot work against `override = true`; the exception belongs in the containment layer |
| Renaming a label value | The taxonomy is a published interface; add, don't rename |

## See also

- [Deny rules](deny-rules.html) — `override`, the actor set, and exclusions
- [Policy provisioning](policy-provisioning.html) — `illumio-core_provisioning`
- [Adopting existing objects](adopting-existing-objects.html) — brownfield onboarding
- [Managed workloads](managed-workloads.html) — enforcement mode and destroy semantics
- [illumio-policy-gitops](https://github.com/alexgoller/illumio-policy-gitops) — the
  same conventions as reviewed YAML, with security checks and traffic evidence
