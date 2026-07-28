# `illumio-core_provisioning`: Terraform-Native Policy Provisioning — Design

Date: 2026-07-28
Status: Approved design, not yet implemented
Relates to: PR #1 (`fix/provision-hardening`), which hardens the existing binary but does not change the architecture

## 1. Problem

Policy objects created by Terraform land in the PCE's `draft` version. They only
take effect once provisioned into `active` via `POST /orgs/{org}/sec_policy`
with a `change_subset`.

Terraform has no post-apply hook, so today the provider records every mutated
policy object's href to a `hrefs.csv` file in its working directory, and a
separate `provision` binary is run afterwards:

```bash
terraform apply && provision
```

PR #1 fixed the correctness bugs in that binary — it no longer drops changes
silently, panics, or reports false success. The architecture, however, remains
a problem:

1. **Structurally incompatible with remote execution.** The provider writes the
   file into the *plugin process's* working directory; the binary reads it from
   the *user's*. That coincidence only holds for local CLI runs. Terraform
   Cloud/Enterprise, Atlantis, Spacelift and most CI containers break it.
2. **Provisioning is invisible to Terraform.** State has no notion of draft
   versus active, so `plan` cannot say "this will be provisioned", and nothing
   detects that policy was activated out-of-band.
3. **Cross-process coordination is unsound.** `provider.go`'s `fileMutex` is a
   `sync.Mutex`, which protects one process. Terraform spawns a separate plugin
   process per provider alias, so two aliased providers write the same file with
   no coordination, and `O_APPEND` is not atomic on NFS.
4. **The file accumulates.** It is append-only; if `provision` is never run,
   entries pile up across applies.
5. **Two credential paths.** The binary re-reads `ILLUMIO_*` environment
   variables and builds its own client, so a user who configures the provider
   via HCL must *also* export environment variables.

## 2. Goal

Make provisioning a first-class Terraform resource so that it is state-tracked,
visible in `plan`, ordered by the dependency graph, and works identically
locally, in CI, and in Terraform Cloud.

The `provision` binary and `hrefs.csv` remain functional and are deprecated, not
removed.

## 3. Resource design

```hcl
resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.app.href,
    illumio-core_ip_list.external.href,
    illumio-core_enforcement_boundary.quarantine.href,
  ]

  update_description = "Provisioned by Terraform"
}
```

Referencing resource attributes in `hrefs` gives Terraform real dependency
edges, so the objects are always created before they are provisioned. No
`depends_on` is required and no ordering is inferred from a flat file.

### 3.1 Arguments

| Argument | Type | Behaviour |
|---|---|---|
| `hrefs` | set of string | Optional. Policy object hrefs to provision. Each must be a `sec_policy` object href. |
| `provision_all_pending` | bool | Optional, default `false`. When true, provision everything in the pending set, including objects not managed by Terraform. |
| `update_description` | string | Optional, default `"Provisioned by Terraform"`. Becomes the PCE commit message. |

`hrefs` and `provision_all_pending` are **mutually exclusive**
(`ConflictsWith`), and at least one must be set. Neither set is a validation
error, because a provisioning resource that can never provision anything is
always a mistake; both set is a validation error rather than a silent
precedence rule, so the config always states plainly which mode is in use.

⚠️ `provision_all_pending` is dangerous on a shared PCE: it activates *any*
pending change, including a half-finished policy another team or a UI user left
in draft. The schema description and docs must say so explicitly. It exists for
single-owner PCEs where enumerating hrefs is impractical.

### 3.2 Attributes

| Attribute | Meaning |
|---|---|
| `version` | Integer version number of the security policy created by the last provisioning |
| `version_href` | e.g. `/orgs/4130175/sec_policy/3` |
| `provisioned_at` | Timestamp of the last successful provisioning |
| `object_counts` | Map of object type to count, as returned by the PCE |
| `workloads_affected` | Number of workloads affected, as returned by the PCE |
| `pending_hrefs` | Hrefs this resource is responsible for that currently have pending changes. Empty when everything is provisioned. |

### 3.3 Identity

Provisioning has no natural remote identity — a PCE security policy version is a
point-in-time event, not an object Terraform owns. The resource ID is therefore
a generated `resource.UniqueId()`, stable for the resource's lifetime, with
`version_href` tracking the most recent provisioning as an ordinary computed
attribute.

Consequently **the resource is not importable**. `ImportState` is not
implemented, and the docs say why.

## 4. The core problem: detecting when re-provisioning is needed

When a rule set's *contents* change, its href does **not**. A naive `hrefs`
list is therefore unchanged, Terraform sees no diff, and nothing re-provisions —
the resource would silently do nothing after the first apply. That is the same
class of silent failure PR #1 removed, so the design must not reintroduce it.

**Solution: `CustomizeDiff` queries the pending set at plan time.**

```
CustomizeDiff:
  pending  := client.PendingPolicyHrefs()          // GET /sec_policy/pending
  targets  := provision_all_pending ? pending : intersect(hrefs, pending)
  if len(targets) > 0:
      d.SetNew("pending_hrefs", sorted(targets))
      d.SetNewComputed("version")
      d.SetNewComputed("version_href")
      d.SetNewComputed("provisioned_at")
```

Setting a *new* value on `pending_hrefs` produces a real planned change, so
`terraform plan` shows exactly which objects will be provisioned — the first
time provisioning state has ever been visible in a plan.

⚠️ **Verify this first.** The entire design rests on the assumption that
`SetNew`/`SetNewComputed` inside `CustomizeDiff` produces a planned *update* on
a resource whose configured attributes are all unchanged. This is the standard
SDKv2 mechanism for remote-state-driven diffs, but it must be proven with a
throwaway resource before the rest of the resource is built. If it does not
hold, fall back to the `triggers` map approach — an explicit map of values the
user wires to the objects' `updated_at` attributes — and revise §4 accordingly.
Nothing else in this spec changes if that fallback is needed.

`GET /orgs/{org}/sec_policy/pending` returns `{}` (HTTP 200) when nothing is
pending, so the clean case is unambiguous and needs no special-casing.

**Accepted cost:** one API call per plan, and plan output depends on live PCE
state. This is standard for providers that reconcile against a remote system,
but it does mean `terraform plan` requires working PCE credentials. This must be
documented prominently.

## 5. Lifecycle

| Op | Behaviour |
|---|---|
| Create | Build the change subset from the target hrefs, `POST /orgs/{org}/sec_policy`, record version attributes |
| Read | `GET /sec_policy/pending`; refresh `pending_hrefs`. Never errors when the recorded version is gone — versions are historical events |
| Update | Identical to Create; provisioning is inherently idempotent because only pending objects can be provisioned |
| Delete | **No-op.** Remove from state only |

### 5.1 Why Delete is a no-op

Provisioning cannot be undone. `DELETE /orgs/{org}/sec_policy/pending` exists,
but it reverts *all* pending draft changes org-wide — including changes made by
other teams or in the UI — and `POST /sec_policy/{id}/restore` rolls the entire
policy back to an earlier version. Neither is an acceptable automatic
consequence of `terraform destroy` on a bookkeeping resource. The resource must
never call either. Removing it from state is the only safe behaviour, and the
documentation must state that destroying it does not revert policy.

### 5.2 Request shape

`POST /orgs/{org}/sec_policy`:

```json
{
  "update_description": "Provisioned by Terraform",
  "change_subset": {
    "rule_sets": [{"href": "/orgs/1/sec_policy/draft/rule_sets/3"}],
    "ip_lists":  [{"href": "/orgs/1/sec_policy/draft/ip_lists/7"}]
  }
}
```

This is exactly `models.SecurityPolicy` with the map-based
`SecurityPolicyChangeSubset` introduced in PR #1, so no new model is needed.

The response is expected to be a security policy version object, matching the
elements of `GET /orgs/{org}/sec_policy`:

```json
{
  "href": "/orgs/4130175/sec_policy/3",
  "version": 3,
  "commit_message": "...",
  "created_at": "2026-01-22T13:22:17.180Z",
  "created_by": {"href": "/users/..."},
  "object_counts": {...},
  "workloads_affected": 0
}
```

**To confirm during implementation:** the exact POST response body has not been
observed — verifying it would require provisioning real policy, which the
read-only design probe deliberately avoided. If the response does not carry the
version object, fall back to `GET /orgs/{org}/sec_policy?max_results=1` directly
after the POST. The implementation must handle both.

## 6. Shared code

Three pieces currently live in `cmd/provision` (package `main`) and are needed
by the resource. Duplicating them would recreate the drift PR #1 just removed.

| Move | From | To |
|---|---|---|
| `resourceTypeFromHref` | `cmd/provision` | `models.ResourceTypeFromHref` — pure string function, no new dependencies, and it belongs beside `SecurityPolicyChangeSubset.AppendHref` |
| pending-set collection | `cmd/provision.collectPendingHrefs` | `client.(*V2).PendingPolicyHrefs()` — the `client` package already imports gabs and owns HTTP, so it should own the GET as well as the parse |

`cmd/provision` is then rewritten to call the shared implementations, so the
binary and the resource can never disagree about what is provisionable. The
existing tests move with the code.

## 7. Coexistence with `hrefs.csv`

The provider writes `hrefs.csv` on every policy mutation via
`Config.StoreHref`. A user who adopts `illumio-core_provisioning` never runs the
binary, so the file grows forever and is never consumed.

Add a provider-level setting:

```hcl
provider "illumio-core" {
  write_href_file = false   # default true; deprecated
}
```

- Default `true` — existing workflows are unaffected.
- Documented as deprecated, to be removed together with the binary in the next
  major version.
- The `illumio-core_provisioning` docs tell users to set it to `false`.

## 8. Known limitations

These are stated plainly rather than designed around, because the resource model
genuinely cannot solve them.

### 8.1 Destroy does not provision deletions

`illumio-core_provisioning` depends on the objects it provisions, so Terraform
destroys it **first**, then the objects. Their deletions are therefore left
unprovisioned in draft, and the objects remain active in the PCE until something
provisions the deletion.

This is not a regression — the binary has the identical problem, which is why
the current docs instruct users to run `provision` after a destroy. It is
inherent to modelling a post-apply side effect as a resource.

Workarounds, in preference order:

1. Run `provision` (the deprecated binary) once after `terraform destroy`.
2. Run a second, separate Terraform configuration containing only an
   `illumio-core_provisioning` resource with `provision_all_pending = true`.

The real fix is a post-destroy hook, which Terraform 1.14's `action_trigger`
provides. That is Stage 3 and is gated behind migrating the provider to
terraform-plugin-framework.

### 8.2 `plan` requires PCE credentials

`CustomizeDiff` calls the PCE (§4). A plan with no credentials or no network
access to the PCE will fail for this resource. Users needing hermetic plans
should not adopt this resource until Stage 3.

### 8.3 Provisioning is org-wide, the resource is not

The PCE provisions a change subset atomically for the whole org. Two
`illumio-core_provisioning` resources applied concurrently create two policy
versions. This is safe but produces more versions than expected; the docs should
recommend one provisioning resource per configuration.

## 9. Testing

### 9.0 Spike (before anything else)

A throwaway resource proving that `CustomizeDiff` + `SetNew` on a computed
attribute yields a planned update with no configured-attribute change (§4). If
this fails, switch to the `triggers` fallback before writing the resource.

### 9.1 Unit

- `models.ResourceTypeFromHref` — existing tests, moved.
- Change-subset construction from a mixed href set, including nested objects
  (deny rules and sec rules resolve to their parent rule set).
- Target selection: intersection of `hrefs` with the pending set;
  `provision_all_pending` returning the whole set.
- Validation: neither `hrefs` nor `provision_all_pending` set is an error.

### 9.2 Mocked HTTP

- `POST /orgs/{org}/sec_policy` body matches §5.2 exactly.
- Version attributes populated from the response, and from the
  `GET /sec_policy` fallback when the POST response lacks them.
- `CustomizeDiff` forces a diff when a managed href is pending, and forces none
  when the pending set is empty or disjoint.
- Delete issues **no** HTTP request.

### 9.3 Acceptance (live PCE, draft policy)

1. Create a rule set and an ip list; do not provision. Confirm both appear in
   `sec_policy/pending`.
2. Add `illumio-core_provisioning`; apply. Confirm a new policy version exists
   and the pending set no longer contains those hrefs.
3. `terraform plan` is empty.
4. Modify the rule set's *contents* (href unchanged). Confirm `plan` now shows a
   provisioning change with the rule set in `pending_hrefs` — the regression
   test for §4.
5. Apply; confirm a new version and an empty plan.
6. `provision_all_pending = true` picks up an object created outside Terraform.
7. Destroy the provisioning resource alone; confirm no HTTP call and no policy
   change.

## 10. Definition of done

- `illumio-core_provisioning` creates a policy version from an explicit href set.
- `terraform plan` names the objects that will be provisioned.
- Changing a policy object's contents produces a provisioning diff on the next
  plan (§4).
- Destroying the resource makes no API call and reverts nothing.
- `provision_all_pending` is opt-in, defaults to false, and is documented as
  unsafe on shared PCEs.
- `resourceTypeFromHref` and pending collection have exactly one implementation,
  shared by the binary and the resource.
- `write_href_file = false` stops `hrefs.csv` from being written.
- The binary and `hrefs.csv` still work unchanged by default.
- §8.1 (destroy does not provision deletions) is documented in the resource
  docs, not only in this spec.
