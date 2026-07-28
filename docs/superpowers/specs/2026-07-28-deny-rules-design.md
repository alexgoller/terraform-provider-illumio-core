# Deny Rules in `terraform-provider-illumio-core` — Design

Date: 2026-07-28
Status: Approved, pending live-PCE probe
Supersedes scope discussion in: `DENY_RULES_IMPLEMENTATION_BRIEF.md` (still authoritative for history and rationale)

## 1. Goal

Add deny-rule support to the existing provider as a purely additive change, so
existing users upgrade by bumping the provider version and rewriting nothing.

Deliverables:

- `illumio-core_deny_rule` managed resource, with import
- `illumio-core_deny_rule` and `illumio-core_deny_rules` data sources
- `models.DenyRule` with a deny-specific actor type
- Shared actor/ingress-service helpers extracted from the security-rule resource
- Docs and examples
- Unit, mocked-HTTP, and live draft-policy acceptance tests

Explicitly out of scope: the full-API v2 rearchitecture, any change to
`illumio-core_security_rule`, and any `action` field on existing resources.

## 2. Schema audit — differences from the brief

The brief instructed: *"Locate the authoritative deny-rule GET and create/update
schemas available to the project; record any difference from this brief before
coding."* This section is that record. Source of truth is
`../illumio-py/webservices-v2-experimental-26.3.0/` (Illumio OpenAPI 26.3.0-3066).

### 2.1 The deny-rule endpoint is not in the 26.3 spec (material)

Evidence:

- No path containing `deny` exists in `illumio-core-26.3-openapi.json` (195 paths).
- `v2/illumio.api.json` defines 126 classes including `sec_rules`, `rule_sets`,
  `sec_rule`. There is **no** `deny_rules` class. The only textual match in that
  file is the query parameter `include_deny_rules`.
- `sec_policy_rule_sets_get/post/put.schema.json` expose `rules` and
  `ip_tables_rules` only. There is **no** `deny_rules` array on the rule set.
- The only deny artifacts are `common/deny_rules_get.schema.json` and
  `common/deny_rule_actor.schema.json`.
- `common/deny_rules_get.schema.json` carries `"expose_to":
  ["end_user_private_perm"]`.

Conclusion: deny rules are a private or unreleased PCE feature in 26.3. The
brief's July live-PCE validation is the only evidence the CRUD endpoint exists.

Consequence for this plan: a live probe is **step 1 of implementation**, before
any Go code. See §7.

### 2.2 `virtual_service` and `virtual_server` are not valid deny-rule actors

Brief §2.6 lists seven actor selectors, including `virtual_service` and
`virtual_server`. The authoritative `common/deny_rule_actor.schema.json` permits
exactly:

| Selector | Notes |
|---|---|
| `actors` | string, `enum: ["ams"]` |
| `exclusion` | boolean, default `false`, `end_user_experimental` |
| `label` | `href_object` |
| `label_group` | `href_object` |
| `ip_list` | `href_object` |
| `workload` | `href_object`, `end_user_private_perm` |

`additionalProperties: false`, `minItems: 1`.

This is exactly the trap the brief warned about ("Do not assume every allow-rule
actor is valid for deny rules"). The deny resource therefore gets its own actor
schema and its own Go type. It must not reuse
`models.SecurityRuleProvider`/`SecurityRuleConsumer`, which carry
`VirtualService`/`VirtualServer`.

### 2.3 `network_type` is a defined enum

`common/rule_network_type.schema.json`: `type: string`,
`enum: ["brn", "non_brn", "all"]`, `default: "brn"`.

The `illumio-py` `_DenyRuleBase` carries `network_type` as a field, and the
validated fixture `tests/data/deny_rules.json` contains `"network_type": "brn"`.
Treated as writable-optional, pending live confirmation (§2.6).

### 2.4 The TCP/UDP-only ingress validator is correct for deny rules

`common/sec_rule_ingress_services.schema.json` is a `oneOf`:

1. `href_object` — a service reference
2. inline: `required: ["proto"]`, `proto` is `enum: [6, 17]`, plus optional
   `port` / `to_port` (integers, 0–65535)
3. an `external_data_*` variant (private permission, not modelled)

`proto` being restricted to 6 (TCP) and 17 (UDP) independently confirms the
February live finding that inline `{"proto": 1}` fails for ICMP. ICMP must be
expressed as a service HREF. The existing security-rule validator's TCP/UDP
restriction is therefore correct to inherit — now proven, not assumed.

`common/sec_rule_egress_services.schema.json` is an array of `href_object` only,
confirming brief §2.6.

### 2.5 Minor confirmations

- `deny_rules_get` `required`: `["href", "providers", "consumers",
  "ingress_services"]`. `enabled` is **not** required.
- `description` is present as `type: ["string", "null"]`.
- `override_deny_rules` appears as a second array **only** in read-only analysis
  outputs (`sec_policy_policy_check_get`, `sec_policy_rule_search_post_response`,
  `sec_policy_rule_coverage_post_response`, traffic-flow schemas). It never
  appears in a storage or request schema. The brief's "one `deny_rules` array"
  rule holds for CRUD; the split in analysis output is presentational.

### 2.6 Known limitation

There is **no POST or PUT deny-rule schema anywhere in 26.3**. Writability of
`description` and `network_type` cannot be established from schema. The GET
shape and the `illumio-py` model both imply writable. This is provisional and
must be settled by the live probe.

## 3. Prerequisite: repair the build baseline

`go build ./...` fails on untouched `main`:

```
vendor/github.com/hashicorp/hc-install/product/consul.go:18:60: undefined: simpleVersionRe
vendor/github.com/hashicorp/terraform-exec/tfexec/apply.go:109:11: undefined: Terraform
```

Root cause: `.gitignore` line 12 is `**/terraform.*`. That glob matches
`vendor/**/terraform.go`, so every vendored file with that name was silently
never committed — including
`hc-install/product/terraform.go` (defines `simpleVersionRe`) and
`terraform-exec/tfexec/terraform.go` (defines `type Terraform struct`).

Fix, as its own commit before any deny-rule work:

1. Replace `**/terraform.*` with `**/.terraform.*` and `terraform.tfstate*`.
2. Re-run `go mod vendor`.
3. Verify `go build ./...` and `go vet ./...` are clean.
4. Commit the restored vendor files separately.

This is a pre-existing repository defect, unrelated to deny rules, but it blocks
all verification and so must land first.

## 4. Resource design

### 4.1 Shape

```hcl
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false
  description   = "Block external clients from internal SSH"

  providers {
    label { href = illumio-core_label.internal.href }
  }

  consumers {
    ip_list { href = illumio-core_ip_list.external.href }
  }

  ingress_services {
    proto = "6"
    port  = "22"
  }
}
```

Override-deny is the same resource with `override = true`. There is no second
resource type and no endpoint switching.

### 4.2 Arguments

| Argument | Type | Behaviour |
|---|---|---|
| `rule_set_href` | string | Required, `ForceNew`, validated as a rule-set HREF |
| `enabled` | bool | Optional, default `true` |
| `override` | bool | Optional, default `false` |
| `description` | string | Optional (provisional, §2.6) |
| `network_type` | string | Optional, `ValidateFunc` over `brn`/`non_brn`/`all` (provisional, §2.6) |
| `providers` | set | Required, `MinItems: 1` |
| `consumers` | set | Required, `MinItems: 1` |
| `ingress_services` | set | Required, `MinItems: 1` |
| `egress_services` | set of `{href}` | Optional |
| `unscoped_consumers` | bool | Optional |
| `all_ips_except_for_in_consumers` | bool | Optional |
| `all_ips_except_for_in_providers` | bool | Optional |
| `href`, `created_at`, `updated_at`, `created_by`, `updated_by`, `update_type`, `caps` | | Computed |

`enabled` and `override` are given explicit defaults so the provider
deliberately **owns** those values. This is the brief's stated precondition for
defaulting (§5.4) and prevents SDKv2 from blurring "not configured" with
`false`, which would otherwise cause drift against a PCE-applied default.

Actor blocks accept exactly the five selectors from §2.2, plus `exclusion`.
Exactly one selector per block, enforced by `models.HasOneActor`.

### 4.3 Go model

`models/deny_rule.go`, standalone — it must not embed or alias
`models.SecurityRule`, because that makes it trivially easy to serialise
allow-only fields.

```go
type DenyRuleActor struct {
    Actors     string                 `json:"actors,omitempty"`
    Exclusion  *bool                  `json:"exclusion,omitempty"`
    Label      *LabelOptionalKeyValue `json:"label,omitempty"`
    LabelGroup *Href                  `json:"label_group,omitempty"`
    Workload   *Href                  `json:"workload,omitempty"`
    IPList     *Href                  `json:"ip_list,omitempty"`
}

type DenyRule struct {
    Enabled                    bool             `json:"enabled"`
    Override                   bool             `json:"override"`
    Description                *string          `json:"description,omitempty"`
    NetworkType                string           `json:"network_type,omitempty"`
    UnscopedConsumers          *bool            `json:"unscoped_consumers,omitempty"`
    AllIPsExceptForInConsumers *bool            `json:"all_ips_except_for_in_consumers,omitempty"`
    AllIPsExceptForInProviders *bool            `json:"all_ips_except_for_in_providers,omitempty"`
    Providers                  []*DenyRuleActor `json:"providers"`
    Consumers                  []*DenyRuleActor `json:"consumers"`
    IngressServices            []IngressService `json:"ingress_services"`
    EgressServices             []Href           `json:"egress_services,omitempty"`
}
```

Pointer semantics, resolving the brief's §5.2 open question: `Enabled` and
`Override` are **plain `bool` with no `omitempty`**, so they are always present
in the request body. This follows directly from §4.2 — the provider owns those
two values, so "omitted" is never a state the provider needs to represent, and
a non-pointer field makes it impossible to accidentally send `null` or drop a
`false`. The remaining optional booleans (`UnscopedConsumers`,
`AllIPsExceptForIn*`) stay `*bool` with `omitempty`, because the provider does
**not** own their defaults and must be able to leave them unset for the PCE to
decide.

Reuses existing `Href`, `LabelOptionalKeyValue`, and `IngressService`.
`models.HasOneActor` is reflection-based and works on `DenyRuleActor` unchanged
(it already skips `*bool` fields, so `Exclusion` is correctly ignored).
Implements the existing `ToMap()` contract via `toMap`.

Note `DenyRuleActor` is a single type for both sides; unlike the security rule,
providers and consumers have identical permitted selectors per §2.2.

### 4.4 Shared helper extraction

`illumio-core/resource_illumio_security_rule.go` is 832 lines, much of it actor
and ingress-service schema/expand/flatten. Rather than copy it, extract into a
new `illumio-core/rule_actors.go`:

- `actorSchema(permitted []string) *schema.Resource`
- `expandActors(...)` / `flattenActors(...)`
- `ingressServiceSchema()`, `expandIngressServices(...)`, `flattenIngressServices(...)`
- `egressServiceSchema()` and its expand/flatten

The security-rule resource is refactored to call these with its own permitted
set. Deny-specific validation stays separate from allow-specific validation.
This refactor must be behaviour-preserving: the existing security-rule tests
must pass unchanged before and after.

### 4.5 Lifecycle

| Op | Call |
|---|---|
| Create | `POST {rule_set_href}/deny_rules`, `d.SetId(href)` |
| Read | `GET d.Id()`, derive `rule_set_href` from the HREF |
| Update | `PUT d.Id()` — toggling `override` never changes endpoint |
| Delete | `DELETE d.Id()` |
| Import | HREF passthrough; `rule_set_href` populated during read |

Create, update, and delete each mark the **parent rule set** for provisioning:

```go
pConfig.StoreHref("rule_sets", ruleSetHREF)
```

This is identical to how `resource_illumio_security_rule.go:811` already handles
its parent, so deny rules inherit the provider's existing provisioning
behaviour with no new mechanism.

Absent remote objects are handled consistently with other resources
(`d.SetId("")` and return, not an error).

### 4.6 Data sources

- `data "illumio-core_deny_rule"` — by HREF
- `data "illumio-core_deny_rules"` — requires `rule_set_href`, queries
  `{rule_set_href}/deny_rules`

No top-level or policy-version-wide `/deny_rules` collection is queried; the
audit in §2.1 found no evidence one exists.

The rule-set resource is **not** changed to expose embedded rule arrays. If a
future change exposes them on data sources, it must expose one `deny_rules`
array with an `override` flag, per brief §4.

## 5. Error handling

- Endpoint absent (404 on the probe or on create): fail with an explicit
  diagnostic naming the PCE version and stating that deny rules require a PCE
  where the feature is enabled — not a generic HTTP error.
- Rejected actor type: caught client-side by schema, so the user gets a
  Terraform-level error naming the permitted selectors rather than a PCE 406.
- Inline ICMP: rejected client-side with a message directing the user to
  reference an ICMP service HREF (§2.4).
- Conflicting `href` and inline `proto`/`port` in a service block: mutually
  exclusive, enforced in schema.

## 6. Test plan

### 6.1 Unit — model serialisation

- Ordinary deny sends `override: false`; override-deny sends `override: true`;
  both use the same type.
- Request bodies contain no `action`, `priority`, `overrides`,
  `resolve_labels_as`, `sec_connect`, `stateless`, `machine_auth`,
  `consuming_security_principals`, `use_workload_subnets`.
- `DenyRuleActor` has no `virtual_service`/`virtual_server` field (compile-time,
  reinforced by a marshal test).
- `egress_services` serialise as `{"href": "..."}` objects.
- False booleans survive expand/flatten round trips.

### 6.2 Unit — validation

- Exactly one actor selector per block; zero or two is an error.
- `to_port` requires `port`; `to_port` > `port`.
- `proto` outside {6, 17} is rejected inline.
- `network_type` outside {brn, non_brn, all} is rejected.
- String-to-int conversion for port/proto is stable across round trips.

### 6.3 Mocked HTTP — lifecycle

Assert exact method and URL:

```
POST   {rule_set_href}/deny_rules
GET    {deny_rule_href}
PUT    {deny_rule_href}
DELETE {deny_rule_href}
```

Plus: override toggle without endpoint change, import by full HREF, remote
deletion handling, `StoreHref("rule_sets", ...)` invoked on each mutation, and
no perpetual diff after read.

### 6.4 Live acceptance

Against a disposable **draft** rule set only. Never provisioned unless the
environment explicitly authorises it.

1. Create an empty draft rule set.
2. Create an ordinary deny rule (`override=false`).
3. Create an override-deny rule (`override=true`) via the same resource type.
4. Read both back from the PCE.
5. Confirm both HREFs contain `/rule_sets/{id}/deny_rules/`.
6. Confirm a fetched rule set has one `deny_rules` array and no
   `override_deny_rules` array.
7. Update `enabled`, `description`, `override`.
8. Import one object into fresh state; verify a clean plan.
9. Negative test: a `virtual_service` actor is rejected.
10. Delete both rules and the rule set.

ICMP: search for an existing ICMP service and reference its HREF. Create a
temporary service only if none exists and cleanup is guaranteed. Never silently
create duplicate services.

### 6.5 Commands

```bash
go build ./... && go vet ./...      # must be clean after §3
go test ./models/... ./illumio-core/...
TF_ACC=1 go test ./illumio-core/... -run TestAccIllumioDenyRule -v
```

Acceptance tests require `ILLUMIO_PCE_HOST`, `ILLUMIO_PCE_ORG_ID`,
`ILLUMIO_API_KEY_USERNAME`, `ILLUMIO_API_KEY_SECRET`.

## 7. Implementation sequence

0. **Repair the baseline** (§3). Separate commit. `go build ./...` clean.
1. **Live probe** (§2.1). Against the PCE, confirm:
   - `GET {rule_set_href}/deny_rules` returns 200, not 404
   - `GET {rule_set_href}/override_deny_rules` returns 404
   - whether `description` and `network_type` are accepted on create/update
   - which actor selectors are accepted
   - the exact response key set
   Record findings; amend this spec if reality differs before writing Go code.
2. Extract shared actor/service helpers (§4.4); existing security-rule tests
   must still pass.
3. Failing model serialisation tests, then `models.DenyRule`.
4. Failing expand/flatten and URL lifecycle tests, then the resource.
5. Register in `illumio-core/provider.go`; add import support.
6. Data sources.
7. Docs, templates, and examples.
8. Full test suite, then live acceptance.
9. Record the exact PCE version and observed response keys in the PR.

## 8. Definition of done

- Terraform can create, read, update, import, and delete a deny rule nested
  under a rule set.
- `override=false` and `override=true` work through the same resource and
  endpoint.
- A post-import `terraform plan` is clean.
- False booleans survive without drift.
- Request bodies contain no disproved or invented fields.
- No `/override_deny_rules`, `/sec_deny_rules`, or top-level `/deny_rules`
  endpoint is used.
- `virtual_service` and `virtual_server` are not accepted as deny-rule actors.
- Docs describe consumers as sources and providers as destinations.
- `go build ./...`, `go vet ./...`, and the unit suite pass.
- Live draft-policy test confirms the endpoint and returned field shape.
- The change is additive: no existing resource's schema or behaviour changes.
