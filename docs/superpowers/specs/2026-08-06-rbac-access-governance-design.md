# RBAC and Access Governance as Code — Design

Date: 2026-08-06
Status: Approved design, not yet implemented
Grounded on: archived schemas (`api-schemas/25.2.20`, `api-schemas/26.3.0`) and a
live read-only probe of a 25.2.10 PCE

## 1. Problem

Who can administer the PCE, and over which part of the environment, is currently
configured by hand in the UI. That makes it the least reviewable part of an
otherwise codified deployment: a grant of `owner` leaves no diff, no reviewer, no
history, and no drift detection. Access is exactly the kind of configuration that
benefits most from living in version control.

The provider today covers policy objects — labels, rule sets, rules, IP lists,
services, boundaries, workloads — but has no representation of *access* at all.

A second, related gap: security rules can be scoped to Active Directory user
groups (Illumio's Adaptive User Segmentation), through a top-level
`consuming_security_principals` field on the rule. The provider does not
implement that field, so user-based rules cannot be expressed in Terraform.

## 2. Goal

Make PCE access governance a first-class part of the Terraform configuration:
principals, their scoped role grants, service accounts, and API key lifecycle.

Every resource must work on both 25.2 and 26.x PCEs. Where a schema genuinely
differs between releases the resource selects its payload by version rather than
exposing the difference to the user; §5 identifies the single object where that
applies.

## 3. What the API actually offers

The endpoint list below is from the published OpenAPI indexes; the object shapes
were confirmed against a live 25.2.10 PCE, which is the authority on runtime
behaviour (see §8).

| Path | Methods | Present in |
|---|---|---|
| `/orgs/{org}/roles` | GET | 25.2, 26.3 |
| `/orgs/{org}/roles/{role_name}` | GET | 25.2, 26.3 |
| `/orgs/{org}/permissions` | GET POST | 25.2, 26.3 |
| `/orgs/{org}/permissions/{id}` | GET PUT DELETE | 25.2, 26.3 |
| `/orgs/{org}/auth_security_principals` | GET POST | 25.2, 26.3 |
| `/orgs/{org}/auth_security_principals/{id}` | GET PUT DELETE | 25.2, 26.3 |
| `/orgs/{org}/security_principals` | GET POST | 25.2, 26.3 |
| `/orgs/{org}/security_principals/{sid}` | GET PUT DELETE | 25.2, 26.3 |
| `/orgs/{org}/service_accounts` | GET POST | 25.2, 26.3 |
| `/orgs/{org}/service_accounts/{id}` | GET PUT DELETE | 25.2, 26.3 |
| `/orgs/{org}/service_accounts/{id}/api_keys` | POST | 25.2, 26.3 |
| `/orgs/{org}/service_accounts/{id}/api_keys/{key_id}` | DELETE | 25.2, 26.3 |

### 3.1 Roles are built-in and read-only

`roles` exposes **GET only** in every archived release. Roles cannot be created,
modified, or deleted. The live 25.2 PCE returns nine, and their hrefs are
*names*, not UUIDs:

```
/orgs/1/roles/owner
/orgs/1/roles/admin
/orgs/1/roles/read_only
/orgs/1/roles/ruleset_manager
/orgs/1/roles/ruleset_provisioner
/orgs/1/roles/ruleset_viewer
/orgs/1/roles/limited_ruleset_manager
/orgs/1/roles/global_object_provisioner
/orgs/1/roles/workload_manager
```

The role object has exactly one field, `href`. There is nothing to configure.

**Consequence for this design:** "role definition as code" in Illumio means the
**permission** — the binding of a principal to a built-in role over a label
scope. That is the grant, and it is the artefact worth reviewing in a pull
request. Because role hrefs are stable names, a permission can take
`role = "ruleset_manager"` as a validated string and construct the href itself.
No data source lookup is required to write a permission.

### 3.2 Two principal collections, with different jobs

The probe found both collections populated and, importantly, **unrelated**: no
`auth_security_principal` of type `group` corresponded to any `security_principal`
by name or by SID.

**`security_principals`** are Active Directory groups, keyed by SID:

```
href            /orgs/1/security_principals/S-1-5-21-…-1108
sid             S-1-5-21-…-1108
name            Engineering
description
deleted         false      (read-only)
used_by_ruleset true       (read-only)
```

The `used_by_ruleset` field is the tell: these exist to be referenced by
**security rules**, not to grant administrative access. This is Adaptive User
Segmentation.

**`auth_security_principals`** are the identities that administrative permissions
bind to, keyed by UUID, with a `type` of `user`, `group`, or `service_account`.

Both are needed, for different features.

### 3.3 Service accounts and auth principals share a UUID

Confirmed on both service accounts on the live PCE: the service account's UUID is
identical to its `auth_security_principal` UUID.

```
/orgs/1/service_accounts/4a971d49-4139-4791-b153-131f23d5d5c9
/orgs/1/auth_security_principals/4a971d49-4139-4791-b153-131f23d5d5c9   type=service_account
```

So granting a service account a permission needs no lookup — the auth principal
href is derivable from the service account href. The provider should do that
derivation rather than making users discover it.

### 3.4 Permission scope is the familiar label array

```json
{
  "auth_security_principal": {"href": "/orgs/1/auth_security_principals/…"},
  "role":                    {"href": "/orgs/1/roles/ruleset_manager"},
  "scope": [{"label": {"href": "/orgs/1/labels/27"}},
            {"label": {"href": "/orgs/1/labels/9"}}]
}
```

This is the same shape as a rule set scope, with the same semantics already
established for this provider: **OR within a label type, AND across types.** A
two-label scope of `app=erp` and `env=prod` grants the role over the
intersection.

An **empty scope array means unrestricted**. On the live PCE, all four `owner`
permissions have `scope: []` while every `ruleset_manager` has one or two labels.
This matters for §6.

## 4. Resources

### 4.1 `illumio-core_permission`

The core of this design.

```hcl
resource "illumio-core_permission" "erp_admins" {
  auth_security_principal_href = illumio-core_auth_security_principal.erp_team.href
  role                         = "ruleset_manager"

  scope {
    label { href = illumio-core_label.app_erp.href }
    label { href = illumio-core_label.env_prod.href }
  }
}
```

| Argument | Type | Required | Notes |
|---|---|---|---|
| `auth_security_principal_href` | string | yes | Forces new resource |
| `role` | string | yes | Validated against the nine built-ins |
| `scope` | block | no | Omitted or empty means unrestricted |
| `href` | string | computed | |

`role` accepts the bare name and builds `/orgs/{org}/roles/{name}` internally.
Validation is a static enum: the set has been stable across every archived
release, and a typo should fail at plan time, not as a 404 at apply time.

Update uses PUT, which accepts all three fields. Only `scope` and `role` are
practically mutable; changing the principal forces replacement, because a
permission is conceptually a grant *to* someone.

### 4.2 `illumio-core_auth_security_principal`

```hcl
resource "illumio-core_auth_security_principal" "erp_team" {
  type         = "group"
  name         = "CN=ERP Admins,OU=Groups,DC=example,DC=com"
  display_name = "ERP Admins"
}
```

| Argument | Type | Required | Notes |
|---|---|---|---|
| `type` | string | yes | `user`, `group`; `service_account` on 26.x — see §5 |
| `name` | string | 25.2 only | See §5 |
| `display_name` | string | no | 25.2 only — see §5 |
| `access_restriction_href` | string | no | |

This resource carries the design's only genuinely divergent schema — see §5 —
and its write shape must be confirmed against a live 26.x PCE before release.

### 4.3 `illumio-core_security_principal`

```hcl
resource "illumio-core_security_principal" "engineering" {
  sid  = "S-1-5-21-3967917948-4012392808-1110388204-1108"
  name = "Engineering"
}
```

| Argument | Type | Required | Notes |
|---|---|---|---|
| `sid` | string | yes | Forces new resource; becomes the href |
| `name` | string | yes | |
| `description` | string | no | |
| `deleted`, `used_by_ruleset` | bool | computed | |

Because the href **is** the SID, this resource gets two properties for free:

- **Natural import.** `terraform import illumio-core_security_principal.eng S-1-5-…`
  reads the way an operator expects, unlike the opaque-href imports that made
  workload import unusable.
- **Adopt-on-create.** A SID already registered on the PCE is the same principal.
  Reuse the existing `adoptOnConflict` helper so a conflict adopts rather than
  fails, consistent with the behaviour added in 2.0.3.

### 4.4 `consuming_security_principals` on `illumio-core_security_rule`

Not a new resource — a missing field on an existing one. It is a top-level array
of hrefs on the rule, **identical in 25.2.20 and 26.3.0**, so it needs no version
gating:

```hcl
resource "illumio-core_security_rule" "vpn_to_erp" {
  # … providers, consumers, ingress_services as today …

  consuming_security_principals = [
    illumio-core_security_principal.engineering.href,
  ]
}
```

This completes Adaptive User Segmentation: the rule applies only to traffic from
machines where a member of that AD group is logged in. Without it,
`illumio-core_security_principal` would be a resource with nothing to reference
it.

### 4.5 `illumio-core_service_account`

```hcl
resource "illumio-core_service_account" "ci" {
  name        = "terraform-ci"
  description = "Pipeline automation"

  permission {
    role = "ruleset_provisioner"
    scope {
      label { href = illumio-core_label.env_prod.href }
    }
  }
}
```

`service_accounts_post` requires `name`, `permissions`, **and `api_key`** — a
service account cannot be created without minting one key. The `api_key` object
requires `expires_in_seconds`, accepting either the string `"default"` or an
integer from `-1` (never expires) to `2147483647`.

Permissions are inline here rather than separate `illumio-core_permission`
resources, because the API models them as part of the account's creation payload.
The resource exposes `permission` blocks with the same `role` + `scope` shape as
§4.1.

**A service account's permissions belong to exactly one of the two forms.**
Because a service account's UUID is also its auth principal UUID (§3.3), the same
grant could be written either as an inline `permission` block here or as a
standalone `illumio-core_permission` pointing at the derived auth principal href.
Doing both would put two resources in charge of one object, and each plan would
fight the other. The rule is: **inline blocks own a service account's
permissions.** `illumio-core_permission` is for principals that are not service
accounts — real users and AD groups. The guide states this, and the service
account resource's `Read` reconciles only the permissions it created, so a grant
added out of band does not silently disappear on the next apply.

### 4.6 `illumio-core_service_account_api_key`

Keys are a real sub-resource — `POST` to mint, `DELETE` to revoke — so they get
their own Terraform resource. That makes **rotation a native Terraform
operation**: `terraform taint` or a changed `expires_in_seconds` replaces the key,
and the dependency graph orders creation before revocation.

```hcl
resource "illumio-core_service_account_api_key" "ci" {
  service_account_href = illumio-core_service_account.ci.href
  expires_in_seconds   = 7776000   # 90 days
}
```

**Secret handling.** `api_keys` comes back **empty on every read** — the probe
confirmed `api_keys: []` on both live accounts. The secret exists only in the
POST response. This is the same situation as pairing keys, and gets the same
answer, which is stronger than marking the attribute `Sensitive`: encrypt it with
`aesGcmEncrypt` under `ILLUMIO_AES_GCM_KEY` and store ciphertext plus nonce, so
the plaintext never reaches state.

| Attribute | Notes |
|---|---|
| `service_account_href` | required, forces new |
| `expires_in_seconds` | required, `"default"` or `-1`…`2147483647`, forces new |
| `encrypted_api_key`, `nonce` | computed, AES-GCM as above |
| `key_id`, `auth_username`, `created_at` | computed |

Read cannot recover the secret, so `Read` must not clear it or every plan would
show a spurious diff. It refreshes metadata only and leaves the ciphertext
untouched.

## 5. Version compatibility

This is the same bug class that broke deny rules on 25.2, where the provider sent
`all_ips_except_for_in_consumers` to a PCE that had never heard of it. The
countermeasure is established: gate every version-specific field behind
`isConfigured(d, key)` so it is transmitted only when the user set it explicitly,
and add the object to `schema_compat_test.go` so the archived schemas act as an
offline oracle.

Deltas found between `25.2.20` and `26.3.0`:

| Object | 25.2.20 | 26.3.0 | Action |
|---|---|---|---|
| `security_principals` (POST) | `sid`, `name`, `description` | identical | none |
| `security_principals_put` | `name`, `description` | adds `members` | gate `members` |
| `security_principals_get` | — | adds `id`, `members`, `created_at`, `updated_at` | read-only; tolerate absence |
| `orgs_permission` | `auth_security_principal`, `role`, `scope` | identical | none |
| `service_accounts_post` | — | adds `uuid` | gate `uuid` |
| `consuming_security_principals` | `href` | identical | none |
| `orgs_auth_security_principal` | `type` (`user`\|`group`), **`name` required**, `display_name`, `access_restriction` | `type` (`user`\|`group`\|`service_account`), `uuid`, `access_restriction` — **`name` and `display_name` absent, `name` no longer required** | see below |
| `security_principal_users` (whole collection) | absent | present | out of scope |

`orgs_auth_security_principal` is the one that diverges in **both** directions:
26.3 drops fields that 25.2 *requires*. A single write payload cannot satisfy
both, so the resource must select its shape by PCE version rather than merely
withholding new fields.

Two cautions on that row. First, the live 25.2 PCE returns `name` and
`display_name` on GET, matching its schema. Second, the 26.3.0 bundle is the
experimental one distributed with `illumio-py`, not published on Illumio's docs
host, so it is weaker evidence than the 25.2.20 bundle. Per this repository's
standing rule — schemas are authoritative about *fields*, a live PCE is
authoritative about *behaviour* — **§4.2 must not ship until its write shape is
confirmed against a live 26.x PCE.** Everything else here is confirmed on 25.2
and unchanged in 26.3, so it can ship first.

## 6. Lockout hazard

Terraform-managed permissions can remove the grant that the provider's own
credentials depend on. A `terraform destroy` that deletes the last `owner`
permission is not recoverable through the API — it needs PCE console access.

This is the same category as the workload `unpair` problem fixed in 2.0.2, where
a destroy quietly performed an irreversible action, and it gets the same shape of
answer:

1. `Delete` refuses to remove a permission whose role is `owner` or `admin`, or
   whose scope is empty (unrestricted), unless the resource sets
   `allow_privileged_removal = true`.
2. The refusal names the role and the flag, so the message states both what was
   refused and how to proceed deliberately.
3. The guide documents the failure mode with the recovery path, and recommends
   that the credentials Terraform itself authenticates with be managed outside
   Terraform.

The default protects the common case; the flag keeps the deliberate case
possible. Nothing is silently skipped.

## 7. Documentation

- A new guide, `docs/guides/access-governance.md`, covering the principal model
  (the two collections and why both exist), scoped grants and their AND/OR label
  semantics, service account key rotation, and the lockout hazard with its
  recovery path.
- A worked example showing a delegated-administration pattern: an AD group
  granted `ruleset_manager` over one application scope only.
- Per-resource reference pages generated by `tfplugindocs` as usual.

## 8. Testing

Every resource in §4 except `illumio-core_auth_security_principal` on 26.x is
testable against the 25.2 PCE already available, which carries 9 roles, 11
permissions, 5 security principals, 9 auth principals, and 2 service accounts.

1. **Unit** — expand/flatten round trips for permission scope and service account
   permission blocks, plus role-name-to-href construction.
2. **Schema compat** — extend `schema_compat_test.go` to assert, from the archived
   schemas, that no 26.3-only field appears in a 25.2 payload. This test caught
   the deny-rule regression when it was deliberately reintroduced, and must be
   shown to fail the same way here before it is trusted.
3. **Live, on 25.2** — add steps to `test-suite/` covering: create a security
   principal and reference it from a rule's `consuming_security_principals`;
   create an auth principal, grant it a scoped permission, widen the scope, then
   remove it; create a service account with an inline permission, mint a second
   API key, and revoke the first.
4. **Lockout guard** — assert that deleting an `owner` permission fails with the
   documented message, and succeeds once `allow_privileged_removal` is set.
   Perform this on a throwaway permission, never on the credentials in use.
5. **Import** — confirm `terraform import` by bare SID for security principals,
   and that adopt-on-create adopts an already-registered SID rather than failing.

The suite is stepwise with manual acknowledgement, as established, and every step
is verified by reading the object back from the API rather than trusting state.

## 9. Out of scope

- **Custom roles.** Not offered by the API in any archived release; `roles` is
  GET-only.
- **`security_principal_users`.** 26.3-only, and no 26.x PCE is available to test
  against.
- **`bulk_create` for security principals.** Bulk endpoints do not map cleanly
  onto per-resource state; individual resources cover the same ground.
- **`virtual_server` and `slb`.** Deferred deliberately: both collections are
  empty on the available PCE, `discovered_virtual_servers` is empty too, and
  `discovered_virtual_server` is a required field on virtual server creation — so
  there is nothing to adopt and no way to establish whether the resource can be
  created at all. Revisit when a PCE with an F5 or NEN-backed SLB is available.
- **`access_restriction` objects.** Referenced by href where the API accepts one,
  but not managed as a resource in this iteration.

## 10. Implementation order

Each step is independently shippable and independently testable.

1. `illumio-core_security_principal` + `consuming_security_principals` on
   `illumio-core_security_rule` — no version divergence, closes an existing gap,
   and delivers a complete feature on its own.
2. `illumio-core_permission` + the `role` name enum + the §6 lockout guard.
3. `illumio-core_auth_security_principal` — after its 26.x write shape is
   confirmed on a live PCE.
4. `illumio-core_service_account` + `illumio-core_service_account_api_key`,
   including the AES-GCM secret handling.
5. Guide, examples, and generated reference documentation.
