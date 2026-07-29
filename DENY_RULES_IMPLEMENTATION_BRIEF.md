# Deny and Override-Deny Rules: Findings and Terraform Provider Implementation Brief

> **Historical handoff document.** This brief was written before the Terraform
> implementation existed. It is kept as the record of how the API contract was
> established. Two statements in it were later disproved against the
> authoritative schema and a live PCE — see
> `docs/superpowers/specs/2026-07-28-deny-rules-design.md` §2 for the audit:
>
> - §2.6 lists `virtual_service` and `virtual_server` as deny-rule actors. They
>   are **not** valid. The permitted set is `actors: "ams"`, `label`,
>   `label_group`, `ip_list` and `workload`.
> - §2.4 leaves `description` and `network_type` writability open. Both are
>   writable, confirmed on PCE 26.30.2.
>
> Everything else in this brief was confirmed correct, including the endpoint,
> the single `deny_rules` array, and the absence of `override_deny_rules`.

This document is a handoff for implementing deny-rule support in
`terraform-provider-illumio-core`. It reconstructs the work performed in the
`illumio-py` fork, including the incorrect early models, the schema audit, the
live-PCE validation, and the final API contract.

The short version:

- Illumio calls these **deny rules**, not deny lists.
- Deny rules are separate policy objects from allow/security rules.
- They live only below a rule set:
  `/orgs/{org}/sec_policy/{pversion}/rule_sets/{rule_set_id}/deny_rules`.
- There is one deny-rule endpoint and one deny-rule representation.
- `override: false` is an ordinary deny rule.
- `override: true` is an override-deny rule.
- A rule set contains one `deny_rules` array holding both kinds.
- There is no `override_deny_rules` endpoint or rule-set array.
- Do not add an `action` field to the existing security-rule resource.
- Do not add `priority`, `overrides`, or `resolve_labels_as` to a deny rule.
- Consumers initiate the connection; providers are the destinations.
- Final precedence is `override-deny > allow > deny`.

## 1. Why the historical code must not be ported blindly

The Python work went through several iterations. Some early commits and notes
capture useful experiments but do not describe the final API.

The original `illumio-py` change, commit `07cd32d` ("Add deny lists"), invented
several concepts:

- `Rule.action` with `allow`, `deny`, and `override_deny`
- a top-level `/sec_deny_rules` collection
- a separate `/sec_override_deny_rules` collection
- numeric deny-rule `priority`
- an override rule's `overrides` list of deny-rule HREFs
- allow-rule `resolve_labels_as` on deny rules

Those concepts were later proven wrong. They must not appear in the Terraform
provider.

The February 2026 follow-up corrected `/sec_deny_rules` to `/deny_rules` and
added `override`, but it still temporarily exposed separate
`deny_rules`/`override_deny_rules` concepts on `RuleSet`. The July 2026
spec-conformance audit deliberately treated that model as untrusted, rebuilt it
from the authoritative schema, and then validated the result against a live
PCE.

Historical notes say a February test ruleset was exercised on a PCE. The July
audit nevertheless found that the model contained invented fields. For provider
work, the July schema- and live-validated model is the authoritative source.

## 2. Final, validated API contract

### 2.1 Endpoint and containment

Collection:

```text
/orgs/{org}/sec_policy/{pversion}/rule_sets/{rule_set_id}/deny_rules
```

Example draft collection:

```text
/orgs/1/sec_policy/draft/rule_sets/3/deny_rules
```

Example object:

```text
/orgs/1/sec_policy/draft/rule_sets/3/deny_rules/10
```

Create therefore needs the containing rule-set HREF:

```text
POST {rule_set_href}/deny_rules
```

Read, update, and delete operate on the deny-rule HREF returned by the PCE.

The following probe returned 404 during the final live validation and must not
be implemented:

```text
GET {rule_set_href}/override_deny_rules
```

### 2.2 Ordinary versus override-deny

Both kinds use the same endpoint and shape:

```json
{
  "enabled": true,
  "override": false,
  "providers": [],
  "consumers": [],
  "ingress_services": []
}
```

- `override=false` (or the server default when omitted) means ordinary deny.
- `override=true` means override-deny.
- An override-deny rule still **blocks** traffic. It does not "allow traffic
  that would otherwise be denied."
- Evaluation order is `override-deny > allow > deny`.

`illumio-py` retains `OverrideDenyRule` only as a convenience builder that
defaults `override=True`. It is not a second API object or collection. For
Terraform, one resource with an `override` boolean is the clean mapping.

### 2.3 Traffic direction

The final adversarial review corrected earlier reversed documentation:

- `consumers` are the sources that initiate connections.
- `providers` are the destinations providing the service.

For "block external clients from reaching an internal SSH service":

- consumer: external IP list/label
- provider: internal workload/label
- ingress service: TCP/22

This orientation should be explicit in schema descriptions and examples.

### 2.4 Confirmed deny-rule fields

The authoritative `deny_rules_get` schema and live response established these
policy fields:

- `enabled`
- `override`
- `network_type`
- `unscoped_consumers`
- `all_ips_except_for_in_consumers`
- `all_ips_except_for_in_providers`
- `providers`
- `consumers`
- `ingress_services`
- `egress_services`

The common mutable-object response also carries metadata such as:

- `href`
- `description`
- `created_at`, `updated_at`, and possibly deletion metadata
- `created_by`, `updated_by`, and possibly deletion metadata
- capability/update metadata returned by the PCE

Before making inherited common fields configurable, check the create/update
schema rather than assuming every response field is writable. In particular,
do not infer `name` or `external_data_*` write support merely because another
policy object has those fields.

### 2.5 Fields explicitly disproved

Do not add:

- `action`
- `priority`
- `overrides`
- `resolve_labels_as`
- a separate `override_deny_rules` field or collection

Also do not reuse allow-only controls without schema proof:

- `sec_connect`
- `stateless`
- `machine_auth`
- `consuming_security_principals`
- `use_workload_subnets`

### 2.6 Actor and service representations

The Python model uses the existing rule actor representation:

```json
{"actors": "ams"}
{"label": {"href": "/orgs/1/labels/2"}}
{"label_group": {"href": "..."}}
{"workload": {"href": "..."}}
{"virtual_service": {"href": "..."}}
{"virtual_server": {"href": "..."}}
{"ip_list": {"href": "..."}}
```

Only one actor selector belongs in each provider/consumer entry. The Go
provider already validates this through `models.HasOneActor`; reuse that
behavior where the deny-rule schema permits the same actor types. Do not assume
every allow-rule actor is valid for deny rules without checking the schema or a
live response.

Ingress services can be inline protocol/port definitions or service HREFs:

```json
{"port": 22, "proto": 6}
{"port": 3389, "to_port": 3395, "proto": 6}
{"href": "/orgs/1/sec_policy/draft/services/3"}
```

The February live work found that inline `{"proto": 1}` did not work for ICMP
in `ingress_services`. The working approach was to find or create an ICMP
service and reference its HREF. Do not silently create duplicate services.

The validated fixture represents `egress_services` as service references:

```json
"egress_services": [
  {"href": "/orgs/1/sec_policy/draft/services/3"}
]
```

## 3. What changed in `illumio-py`

Relevant repository:

```text
~/git/illumio-py
```

### Initial additions and February corrections

- `07cd32d` — original "Add deny lists" implementation; useful only as a record
  of the incorrect starting assumptions.
- `fb3f9ce` — added deny fields and fixtures to `RuleSet`.
- `033b4b9` — corrected the endpoint from `/sec_deny_rules` to `/deny_rules`.
- `212c655` — added `override` to `DenyRule`.
- `6fa9341` — documented `DenyRule` and `OverrideDenyRule`.

### July schema audit and final rework

- `5b5152e` — rewrote deny/override-deny modeling and removed the invented
  action/priority/overrides concepts.
- `0c788e2` — aligned the model with the authoritative `deny_rules_get` schema:
  restored the real `override` field, removed `resolve_labels_as`, and added
  `egress_services` and the `all_ips_except_*` fields.
- `17b7a4a` — after live-PCE validation, reduced `RuleSet` to one `deny_rules`
  array holding both ordinary and override-deny rules.
- `6d7351d` — added a live create/read/delete test script.
- `24eb7be` — added user documentation.
- `b091c90` — adversarial review fixes, including the source/destination
  direction correction and removal of the bogus public
  `pce.override_deny_rules` collection.

The released final behavior is documented in the `illumio-py` 1.2.0 and 1.3.0
changelog entries.

### Current Python reference files

- `illumio/rules/rule.py`
  - `_DenyRuleBase`
  - `DenyRule`
  - `OverrideDenyRule`
- `illumio/rules/ruleset.py`
  - single `deny_rules: List[DenyRule]`
- `tests/unit/test_unit_deny_rules.py`
- `tests/unit/test_unit_rule_sets.py`
- `tests/data/deny_rules.json`
- `tests/data/override_deny_rules.json`
- `tests/data/rule_sets.json`
- `tools/create_test_deny_rules.py`
- `examples/06_deny_rules.py`
- `docs/source/guides/security-policy.rst`
- `docs/source/project/migrating-from-illumio.rst`
- `tasks/todo.md`

At handoff time, the focused Python deny/ruleset tests pass.

## 4. Current Terraform provider gap

The Go provider currently has:

- `illumio-core_security_rule`
- singular and plural security-rule data sources
- `illumio-core_rule_set`
- generic client CRUD functions

It has no deny-rule model, resource, or data source.

Important existing files:

- `models/security_rule.go`
- `models/rule_set.go`
- `illumio-core/resource_illumio_security_rule.go`
- `illumio-core/data_source_illumio_security_rule.go`
- `illumio-core/data_source_illumio_security_rules.go`
- `illumio-core/resource_illumio_rule_set.go`
- `illumio-core/provider.go`
- `client/client_crud.go`

The existing security-rule implementation is a useful mechanical reference,
but its schema cannot be copied wholesale. It contains allow-only fields such
as `resolve_labels_as`, `sec_connect`, `stateless`, `machine_auth`, and
`use_workload_subnets`.

`models.RuleSet` currently models neither embedded allow rules nor embedded
deny rules. Adding a deny-rule resource does not require making embedded rule
arrays configurable on the rule-set resource. If data sources expose embedded
rules later, they must expose one `deny_rules` array with an `override` flag.

## 5. Recommended Terraform design

### 5.1 One dedicated resource

Add:

```text
illumio-core_deny_rule
```

Do not add `action` to `illumio-core_security_rule`. The objects have different
endpoints and different schemas, so a polymorphic resource would create
fragile state migrations and endpoint switching.

Suggested resource shape:

```hcl
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false
  description   = "Block external clients from internal SSH"

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

Override-deny is the same resource:

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
    proto = "6"
    port  = "3389"
  }
}
```

### 5.2 Go model

Create a separate model, for example `models/deny_rule.go`.

Do not embed or alias `models.SecurityRule` because that makes it too easy to
serialize allow-only fields. Reuse the small actor and service types where
their JSON is identical.

Conceptual shape:

```go
type DenyRule struct {
    Enabled                     *bool                       `json:"enabled,omitempty"`
    Override                    *bool                       `json:"override,omitempty"`
    Description                 *string                     `json:"description,omitempty"`
    NetworkType                 string                      `json:"network_type,omitempty"`
    UnscopedConsumers           *bool                       `json:"unscoped_consumers,omitempty"`
    AllIPsExceptForInConsumers  *bool                       `json:"all_ips_except_for_in_consumers,omitempty"`
    AllIPsExceptForInProviders  *bool                       `json:"all_ips_except_for_in_providers,omitempty"`
    Providers                   []*SecurityRuleProvider     `json:"providers"`
    Consumers                   []*SecurityRuleConsumer     `json:"consumers"`
    IngressServices             []IngressService            `json:"ingress_services"`
    EgressServices              []Href                       `json:"egress_services,omitempty"`
}
```

Treat this as a sketch, not a field-type mandate:

- Confirm whether `enabled`/`override` should be pointers so omitted, false,
  and server defaults remain distinguishable.
- Confirm the existing `Href` type serializes to `{"href":"..."}` in a slice.
- Confirm whether `network_type` is writable or response-only.
- Confirm allowed actor variants against the deny-rule create schema.
- Add only writable common fields to the request model.

The model should implement the provider's existing `models.Model`/`ToMap`
contract.

### 5.3 Resource lifecycle

Create:

```go
endpoint := fmt.Sprintf("%s/deny_rules", ruleSetHref)
_, data, err := illumioClient.Create(endpoint, denyRule)
d.SetId(data.S("href").Data().(string))
```

Read:

- GET `d.Id()`
- derive `rule_set_href` from the deny-rule HREF
- flatten all supported fields
- preserve `override=false` in Terraform state
- handle an absent remote object consistently with other provider resources

Update:

- PUT the deny-rule HREF
- changing `override` should update the same object; it must not switch endpoint
- mark the parent rule set for provisioning with the existing `StoreHref`
  mechanism

Delete:

- DELETE the deny-rule HREF
- mark the containing rule set for provisioning
- clear the Terraform ID

Import:

- use HREF passthrough, consistent with the security-rule resource
- populate `rule_set_href` from the imported HREF during read

### 5.4 Schema behavior

Recommended first-class arguments:

- `rule_set_href` — required, `ForceNew`
- `enabled` — required or optional with an explicit default
- `override` — optional, default `false`
- `description` — optional if confirmed writable
- `providers` — required set
- `consumers` — required set
- `ingress_services` — required or schema-conditional as the API specifies
- `egress_services` — optional set of service HREFs
- `network_type` — only if confirmed writable
- `unscoped_consumers` — optional
- `all_ips_except_for_in_consumers` — optional
- `all_ips_except_for_in_providers` — optional
- response metadata — computed only

Boolean defaults deserve tests. Terraform Plugin SDK v2 can otherwise blur
"not configured" and `false`, which matters if the PCE applies a server
default. The final Python builder deliberately allows an ordinary deny rule's
`override` to be omitted; a Terraform default of `false` is reasonable only if
the provider intentionally owns that value.

For actor blocks, refactor shared schema/expand/flatten helpers rather than
copying hundreds of lines. Keep deny-specific validation separate from
allow-specific validation.

For ingress services, the existing resource exposes protocol and ports as
strings and converts them to integers. Reuse that state representation if it
avoids drift, but do not automatically inherit the current TCP/UDP-only
validator without confirming the deny-rule schema. ICMP should be documented
through a service HREF unless live testing proves an inline representation.

### 5.5 Optional data sources

Minimum useful support is the managed resource plus import. If adding data
sources in the same change, follow existing naming:

- `data "illumio-core_deny_rule"` by HREF
- `data "illumio-core_deny_rules"` with an explicit `rule_set_href`

The plural endpoint must be:

```text
{rule_set_href}/deny_rules
```

Do not query a policy-version-wide top-level `/deny_rules` collection unless
the API documentation independently proves one exists.

## 6. Test plan

### 6.1 Model/unit tests

Assert JSON request bodies:

- ordinary deny sends `override=false` or intentionally omits it
- override-deny sends `override=true`
- both use the same model
- no `action`
- no `priority`
- no `overrides`
- no `resolve_labels_as`
- no allow-only fields
- `egress_services` serialize as service HREF objects
- false booleans survive expand/flatten round trips

Assert actor validation:

- exactly one actor selector per provider/consumer block
- providers are destinations and consumers are sources in descriptions/examples
- exclusions and supported actor types follow the deny schema

Assert service validation:

- HREF and inline proto/port are mutually exclusive
- `to_port` requires `port`
- `to_port` is greater than `port`
- numeric JSON remains stable after Terraform string conversion

### 6.2 Resource lifecycle tests with mocked HTTP

Verify exact methods and URLs:

```text
POST   {rule_set_href}/deny_rules
GET    {deny_rule_href}
PUT    {deny_rule_href}
DELETE {deny_rule_href}
```

Include:

- create/read/update/delete ordinary deny
- create/read/update/delete override-deny
- toggle `override` without changing endpoint
- import by full HREF
- remote deletion/state removal behavior
- parent rule-set `StoreHref` calls
- no perpetual diff after read

### 6.3 Acceptance/live tests

Run only against a disposable draft ruleset:

1. Create an empty draft rule set.
2. Create an ordinary deny rule (`override=false`).
3. Create an override-deny rule (`override=true`) through the same Terraform
   resource type.
4. Read both back directly from the PCE.
5. Confirm both HREFs contain `/rule_sets/{id}/deny_rules/`.
6. Confirm a fetched rule set has one `deny_rules` array and no
   `override_deny_rules` array.
7. Update `enabled`, description, and `override` where supported.
8. Import one object into fresh Terraform state and verify a clean plan.
9. Delete both rules and the temporary rule set.

Never provision the test policy unless the acceptance environment explicitly
authorizes it. Draft-object CRUD is enough to prove this provider mapping.

For ICMP:

1. Search for an existing ICMP service.
2. Reference its HREF from the deny rule.
3. Create a temporary service only if none exists and cleanup is guaranteed.

### 6.4 Baseline commands

Before implementation:

```bash
go test ./...
```

Baseline note from this handoff environment (2026-07-28): the command does not
currently pass on untouched `main`. The existing vendored HashiCorp packages
fail to compile with missing symbols including `simpleVersionRe` and
`tfexec.Terraform`, and the client tests also stop when `ILLUMIO_PCE_HOST` is
unset. These failures predate this documentation-only handoff. Establish a
reproducible baseline (including the repository's intended vendor/module mode
and test environment) before attributing failures to deny-rule work.

During implementation:

```bash
gofmt -w models/deny_rule.go illumio-core/resource_illumio_deny_rule.go
go test ./models ./illumio-core
go test ./...
```

Also run the repository's existing formatting, lint, documentation generation,
and acceptance-test conventions from the Makefile and contributor docs.

## 7. Suggested implementation sequence for Claude Code

1. Read this file and the Python reference files listed above.
2. Read the current security-rule resource/data sources and identify helpers
   that can be safely generalized.
3. Locate the authoritative deny-rule GET and create/update schemas available
   to the project; record any difference from this brief before coding.
4. Add failing model serialization tests.
5. Add failing expand/flatten and URL lifecycle tests.
6. Implement the dedicated `models.DenyRule`.
7. Implement `illumio-core_deny_rule`.
8. Register the resource in `illumio-core/provider.go`.
9. Add import support.
10. Add generated/manual resource documentation and examples.
11. Optionally add singular/plural data sources.
12. Run the complete Go test suite.
13. Run a disposable draft-ruleset acceptance test.
14. Record the exact PCE version and observed response keys in the PR.

## 8. Definition of done

The work is complete when:

- Terraform can create, read, update, import, and delete a deny rule nested
  under a rule set.
- `override=false` and `override=true` work through the same resource and
  endpoint.
- A post-import `terraform plan` is clean.
- Terraform state retains all supported false booleans without drift.
- Request bodies contain no disproved/invented fields.
- No `/override_deny_rules`, `/sec_deny_rules`, or top-level `/deny_rules`
  endpoint is used.
- Docs describe consumers as sources and providers as destinations.
- Unit and full provider tests pass.
- A live draft-policy test confirms the endpoint and returned field shape.

## 9. Local evidence used for this handoff

The summary was reconstructed from:

- current `illumio-py` source, fixtures, tests, docs, and git history
- the February 2026 work log
- the July 2026 spec-conformance task record
- the current Terraform provider source

No credentials or environment-specific PCE details are included here.
