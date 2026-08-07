## 2.0.8 (August 7, 2026)

**No shipped code changes.** Functionally identical to 2.0.7. Releases are signed
from this version onward.

BUG FIXES:

* Fix the release workflow producing unsigned releases even with a signing key
  configured. The GoReleaser arguments were built with
  `${{ cond && '' || '--skip=sign' }}`, and GitHub's ternary returns the
  right-hand side whenever the left is falsy — an empty string is falsy, so
  `--skip=sign` was passed on *both* paths. 2.0.7 was published unsigned as a
  result, with the key imported correctly and the log reading `skipping sign`.
  The non-empty branch now sits on the left of `&&`.
* Fail the release if a signing key is configured but no signature is produced.
  The workflow now checks that `_SHA256SUMS.sig` exists, is binary rather than
  ASCII armored, and verifies with `gpg --verify` before publishing. The
  inverted condition was invisible precisely because nothing checked the output.

## 2.0.7 (August 7, 2026)

**No shipped code changes.** The provider binary is functionally identical to
2.0.6. This release exists to be publishable to the Terraform Registry, which
requires every release to be signed.

BUILD:

* Sign the checksum file. A release signing key is now configured, so releases
  carry `terraform-provider-illumio-core_<version>_SHA256SUMS.sig` — a binary
  detached GPG signature, as the Registry requires. Attempting to publish 2.0.6
  failed with `missing SHA256SUMS.sig file`, because it was built before the key
  existed. The release workflow already handled both cases: it signs when a
  `GPG_PRIVATE_KEY` secret is present and skips signing when it is not, so no
  workflow change was needed.
* Publish `terraform-provider-illumio-core_<version>_manifest.json` as a release
  asset. The Registry requires the manifest under that versioned name, not only
  as a file in the repository root.

## 2.0.6 (August 7, 2026)

**No shipped code changes.** The provider binary is functionally identical to
2.0.5. What changed is how it is built and what a release contains.

~> **The archives differ from 2.0.5 in content, not just version.** Provider
archives now hold the binary alone; through 2.0.5 they also carried `README.md`,
`LICENSE` and `CHANGELOG.md`. Terraform's `h1:` lock hash is computed over the
archive contents, so upgrading changes the recorded hash. Run
`terraform providers lock` if a pinned lock file rejects the new archive.

BUILD:

* The release pipeline now runs. Every release from 2.0.0 to 2.0.5 failed at the
  GPG import step, which left GoReleaser skipped entirely — those artifacts were
  built by hand. Three faults were stacked: the GPG step was unconditional and no
  signing key is configured, `.goreleaser.yml` was a version 0 schema that
  GoReleaser v2 refuses to load, and the workflow passed `--rm-dist`, removed in
  v2 in favour of `--clean`. Signing is now optional — releases publish unsigned
  unless a `GPG_PRIVATE_KEY` secret is set, which is needed only for the
  Terraform Registry.
* Provider archives contain the binary alone. `files: []` reads as *unset* in
  GoReleaser and fell back to its defaults, so archives carried documentation
  files that changed their `h1:` hash and diverged from what
  `scripts/build-mirror.sh` produces for the same version.
* Build for 17 platforms, up from 10: adds `freebsd_386`, `freebsd_arm`,
  `freebsd_arm64`, `openbsd_386`, `openbsd_arm`, `openbsd_arm64` and
  `windows_arm64` alongside the existing linux, darwin and windows builds.
* Build the `provision` and `import` helpers with `CGO_ENABLED=0`. They were
  dynamically linked, so the linux builds would not run on musl-based
  distributions such as Alpine.
* Drop `-X main.version` and `-X main.commit`; neither symbol is declared in this
  module, so both were silent no-ops.
* Drop the `go mod tidy` release hook, which could leave `go.mod` inconsistent
  with the committed `vendor/` directory partway through a release.

TESTING:

* Add a `build` workflow running on every push and pull request: `go build`,
  `go vet`, `go test`, and a full snapshot release. It asserts that each provider
  archive holds exactly one correctly named file and that the linux binary is
  statically linked. Nothing exercised `.goreleaser.yml` until a tag was pushed,
  which is how the release pipeline stayed broken across six releases.

BUG FIXES:

* Fix `go test ./...` failing without PCE credentials. `TestAccResourceIllumioVS_StateUpgradeV1` calls its setup before the `resource.TestCase`, so the `TF_ACC` guard that skips acceptance tests never applied to it and the whole package failed. It now skips when `TF_ACC` is unset, and still fails loudly when `TF_ACC` is set without credentials.

## 2.0.5 (August 4, 2026)

**No shipped code changes.** Only tests and documentation, so the binary is
functionally identical to 2.0.4.

TESTING:

* Add `test-suite/`, a stepwise acceptance suite. It creates one resource at a time and reads each back from the API, so a step that reports OK exists on the PCE rather than only in Terraform state. Covers labels, a label group, IP lists with ranges and FQDNs, services, a rule set with an AND-ed app scope, a ring-fence rule, a tiered rule, a deny rule, an override-deny rule, an enforcement boundary, an unmanaged workload, and provisioning with explicit hrefs. `teardown` runs the three-pass sequence and provisions only the suite's own objects, never `provision_all_pending`.

DOCUMENTATION:

* `terraform destroy` on provisioned policy needs **two passes**. The guide previously said deletions stay in draft; running it against a live PCE showed that understates the problem — anything the still-active policy references cannot be deleted at all, so the destroy fails partway. Documents the working sequence, and to capture the HREFs before the first destroy since `terraform show -json` cannot recover them afterwards.
* Document both refusal tokens. A label held by a rule set scope fails with `label_still_has_associated_rule_set`; one held by a label group fails with `label_referenced_by_label_group`. The guide previously implied rule sets were the only thing that pins an object.

## 2.0.4 (August 3, 2026)

**No shipped code changes.** Only a test file was added, so the binary is
functionally identical to 2.0.3. The 2.0.1 deny-rule fix is now confirmed
against a live PCE 25.2.10 rather than inferred from the request payload.

TESTING:

* The 2.0.1 deny-rule fix is now confirmed against a live PCE 25.2.10. The pre-fix provider creates a deny rule and then fails the first update with `406 input_validation_error` naming `all_ips_except_for_in_consumers` and `all_ips_except_for_in_providers`; the current provider creates, updates and re-updates cleanly with an empty plan afterwards. Previously this was verified only by capturing the request payload, because no 25.2 PCE was available.

* Add a schema-driven compatibility guard. `illumio-core/schema_compat_test.go` builds the deny-rule payload with nothing version-specific configured and fails if any field is absent from the oldest archived release, using `api-schemas/25.2.20/common/deny_rules_get.schema.json` as the oracle. This makes PCE compatibility testable without an old PCE to point at, and the test cannot drift from what Illumio published. Verified by reintroducing the original bug and confirming the test fails.

## 2.0.3 (August 3, 2026)

**No provider code changes.** The binary is functionally identical to 2.0.2;
this release ships documentation only.

DOCUMENTATION:

* Add a ring-fencing example: a rule set scoped to an application plus an all-workloads-to-all-workloads rule inside it. `actors = "ams"` is bounded by the rule set's scope, so it resolves to that application's workloads rather than the whole organization.
* Document that a rule set scope takes several `label` blocks in **one** `scopes` block, producing one AND-ed scope. This is the one place the syntax differs from `providers`/`consumers`, which hold exactly one actor per block — repeating a `scopes` block creates a second, independent scope and *broadens* the rule set rather than narrowing it.
* Document scoping a rule set to several applications with one scope per application, and leaving a rule set unscoped when the rules should apply everywhere.

## 2.0.2 (August 1, 2026)

**No provider code changes.** The binary is functionally identical to 2.0.1;
this release ships documentation and tooling only.

DOCUMENTATION:

* Correct which deny-rule fields are 26.x-only. Only `all_ips_except_for_in_consumers` and `all_ips_except_for_in_providers` are — `network_type` and `unscoped_consumers` exist in 25.2, and deny rules themselves are not new in 26.2. The 2.0.1 notes generalised from the reported symptom without the 25.2 schema bundle to hand.
* Document how multiple actor labels combine: **OR within a label type, AND across types**, so `role:web` + `env:prod` + `env:staging` selects `role:web AND (env:prod OR env:staging)`. Adding a label of a new type *narrows* a rule, which on a deny rule means it can block less than intended.
* Document `dynamic` blocks for building several provider or consumer actors without repeating the block by hand. `dynamic "providers"` does not collide with Terraform's provider meta-argument.
* Record the 25.2.20 vs 26.3.0 schema audit. All 28 differing schemas were checked against what the provider sends; deny rules were the only defect, already fixed in 2.0.1.

TOOLING:

* `api-schemas/<version>/` archives Illumio's published API schemas so PCE compatibility is answerable offline. Includes 25.2.10, 25.2.20 and 26.3.0.
* `scripts/fetch-api-schema.sh <version>` downloads and unpacks a release bundle.
* `scripts/diff-api-schema.py <old> <new> [schema]` compares fields between archived releases.

## 2.0.1 (August 1, 2026)

BUG FIXES:

* Fix `illumio-core_deny_rule` failing against a PCE older than 26.2. `all_ips_except_for_in_consumers` and `all_ips_except_for_in_providers` were sent on every update even when unset — they are `Optional + Computed`, so a read puts a value in state and `GetOkExists` then reports them as configured. Comparing the `deny_rules_get` schema between the 25.2.20 and 26.3 bundles confirms those two fields are the only ones 26.x added, so the update was rejected. They are now sent only when the configuration actually sets them, determined from the raw configuration rather than from state. `unscoped_consumers` and `network_type` are gated the same way for consistency, though both exist in 25.2.

NOTES:

* Each `providers` or `consumers` block on a deny rule holds exactly one actor, matching `illumio-core_security_rule` and the PCE's actor array. Several actors means repeating the block; two `label` blocks inside one `providers` block fails with `Too many label blocks`. The schema descriptions and the deny-rules guide now say so.

## 2.0.0 (July 30, 2026)

First release of this fork. It is a superset of upstream 1.1.6 — no resource or
data source was removed and no schema changed incompatibly — with one
deliberate breaking change to runtime behaviour, noted below.

**Version note:** this fork is distributed via a provider mirror under the
`illumio/illumio-core` source address and is not published to the Terraform
Registry. Because `2.0.0` sorts above upstream's 1.x line, always pin it exactly
and scope the mirror with `include`/`exclude` so an unrelated configuration
cannot resolve to it. See the "Using this fork" guide.

FEATURES:

* **New Resource:** `illumio-core_deny_rule` — manages deny rules nested under a rule set at `{rule_set_href}/deny_rules`. Ordinary deny rules and override-deny rules are the same object at the same endpoint, distinguished by the `override` flag; the policy evaluation order is override-deny > allow > deny. Consumers are the sources that initiate the connection and providers are the destinations.
* **New Data Source:** `illumio-core_deny_rule`
* **New Data Source:** `illumio-core_deny_rules` — requires `rule_set_href`; returns both ordinary and override-deny rules in one list.
* **New Resource:** `illumio-core_provisioning` — provisions draft security policy objects into the active policy version from Terraform, replacing the external `provision` binary and the `hrefs.csv` side channel. State-tracked, visible in `terraform plan`, and usable in Terraform Cloud/CI where the binary cannot run.

ENHANCEMENTS:

* `scripts/build-mirror.sh` builds this fork into a Terraform provider mirror, so it can be installed with a normal `terraform init` while it is not published to a registry. Produces both a filesystem-mirror layout and the `index.json` / `<version>.json` files of the provider network mirror protocol, with `zh:` hashes. See the "Using this fork" guide.
* New provider setting `write_href_file` (default `true`, deprecated). Set to `false` when using `illumio-core_provisioning` so `hrefs.csv` is no longer written.
* `resourceTypeFromHref` and pending-policy collection now have a single implementation shared by the `provision` binary and the provider (`models.ResourceTypeFromHref` and `client.(*V2).PendingPolicyHrefs`), so the two can no longer disagree about what is provisionable.
* Client unit tests no longer require PCE credentials. The credential check was a `log.Fatal` in `init()`, which killed the whole test binary; tests needing a PCE now skip instead.

NOTES:

* Deny rules permit a narrower actor set than security rules: `actors = "ams"`, `label`, `label_group`, `ip_list` and `workload`. `virtual_service` and `virtual_server` are **not** valid deny-rule actors and are rejected at plan time.
* ICMP has no inline representation in a deny rule's `ingress_services` — `proto` accepts only `6` (TCP) and `17` (UDP). Reference an ICMP service by `href` instead.
* Deny rules are provisioned as part of their containing rule set.
* `illumio-core_provisioning` requires a `lifecycle.replace_triggered_by` block referencing the policy resources to provision in the same apply. A policy object's HREF does not change when its contents change, so without it a contents-only edit is not provisioned until the following apply.
* Destroying `illumio-core_provisioning` makes no API call and does not revert policy. `terraform destroy` does not provision deletions.

BREAKING CHANGES:

* Destroying an `illumio-core_managed_workload` no longer unpairs the VEN. Previously `terraform destroy` called `PUT /orgs/{org}/vens/unpair` with `firewall_restore: "default"`, uninstalling the agent from the host — despite Terraform never having created the workload, since managed workloads exist because a VEN paired with the PCE and the create function refuses outright. Destroying now relinquishes management: the resource leaves state, the VEN stays paired, and a warning says so. Set `unpair_on_destroy = true` to opt back in to the old behaviour.

ENHANCEMENTS:

* `illumio-core_label`, `illumio-core_ip_list`, `illumio-core_label_group` and `illumio-core_rule_set` now adopt an object that already exists on the PCE instead of failing the apply. The PCE rejects a duplicate with a 406 rather than returning the existing object, so declaring a label another team already created used to require `terraform import` with a numeric HREF. Adopted objects record `adopted = true` and are **left in place** when the resource is destroyed, since Terraform did not create them; objects Terraform created are deleted as before. `illumio-core_service` is excluded because the PCE permits duplicate service names, leaving no unique key to adopt by.
* `illumio-core_managed_workload` can adopt an existing workload on create. Setting `hostname` makes `terraform apply` find the workload with that hostname and start managing it, with no import step — so a fleet can be managed with `for_each` over a list of hostnames. `hostname` is `ForceNew`, since changing it adopts a different workload. Import remains available and unchanged; `illumio-core_firewall_settings` already used this adopt-on-create pattern.
* Workloads can be imported without an HREF. `terraform import illumio-core_managed_workload.web "web-01.example.com"` matches on `hostname`, then `name`, then `external_data_reference`; `hostname=`, `name=`, `ip_address=` and `external_data_reference=` pin a specific attribute. Matches must be exact and unique — the PCE matches these parameters partially, so substring hits are filtered out and ambiguous values fail with the candidate HREFs listed. HREFs still work unchanged. Applies to `illumio-core_unmanaged_workload` too, and to Terraform 1.5+ `import` blocks, which makes adopting a fleet from a list of hostnames practical.

BUG FIXES:

* Fix collection data sources silently returning a subset of results. They used the plain `Get`, which the PCE caps when `max_results` is unset, and the provider never read the `X-Total-Count` header that reports the real total. All 22 collection data sources now use `AsyncGet`, which switches to the PCE's async job API above 500 results. The PCE has no `offset` or `page` parameter, so this is the only way to retrieve a large collection. Measured on a PCE holding 550 workloads: upstream returned 500, this fork returns 550.
* Fix the provider panicking with `interface conversion: interface {} is nil, not string` when an API response omitted a field. 67 unchecked type assertions now read through a helper that returns an empty string, so an unexpected response shape no longer crashes Terraform mid-apply. One site also indexed `Children()[0]` without a length check.
* Fix `Config.StoreHref` leaking a file handle on every policy mutation, and calling `panic()` when `hrefs.csv` could not be opened or written — a full disk or read-only directory crashed the provider.
* Fix a failed VEN unpair reporting success when destroying an `illumio-core_managed_workload` with `unpair_on_destroy = true`. The diagnostics returned by the unpair call were discarded.
* Fix `terraform plan` hard-erroring with `not-found: <href>` when a managed object is deleted outside Terraform. Every resource returned the raw client error from its read, so the configuration became wedged — plan, apply and destroy all failed until the user ran `terraform state rm` by hand. Reads now remove the object from state so the next plan recreates it, as Terraform expects. Affects all resources that read from the PCE; previously only labels, label types and unmanaged workloads recovered, and only because Illumio soft-deletes those.
* Fix `go build ./...` failing on a fresh clone. The `**/terraform.*` pattern in `.gitignore` matched `vendor/**/terraform.go`, so the vendored files defining `hc-install`'s `simpleVersionRe` and `terraform-exec`'s `Terraform` type were never committed.
* `provision` no longer reports success when it fails to provision a policy object. Previously an unrecognised resource type was dropped by a `switch` with no `default` case, an href missing from the pending set was only logged as a warning, and the process exited 0 in both cases.
* `provision` no longer panics with an index-out-of-range error on a line in `hrefs.csv` that contains no comma.
* `provision` now derives the policy object type from the href instead of a hardcoded list of six resource types, so object types the binary does not know about — including `virtual_servers`, which had no field on the change subset at all — are provisioned correctly rather than silently discarded.
* `provision` now discovers pending objects by walking the full `sec_policy/pending` response rather than a fixed allowlist with `firewall_settings` special-cased.
* `provision` now exits 0 when there is nothing to provision. Previously a missing `hrefs.csv` was a fatal error, so `terraform apply && provision` failed on every apply that changed no policy objects. It also no longer requires PCE credentials for such a no-op run.
* `provision` now leaves `hrefs.csv` in place when a run fails, so it can be retried after the cause is fixed, and reports a failed cleanup as an error that explicitly states provisioning itself succeeded.

## 1.1.6 (Dec 7, 2023)

BUG FIXES:

* Fix errors caused by customdiff functions when plan/apply is run with a null state on the following resources:
    * `illumio-core_ip_list`
    * `illumio-core_container_cluster_workload_profile`

## 1.1.5 (Dec 7, 2023)

ENHANCEMENTS:

* Normalize CIDR notation `ip_ranges` in the `illumio-core_ip_list` resource on create/update to avoid plan diff between HCL and PCE from_ip values

## 1.1.4 (Nov 3, 2023)

BUG FIXES:

* Fix `illumio-core_pairing_profile` `agent_software_release` default to work when VEN library is not empty
* Fix `illumio-core_ip_list` resource to work properly when `ip_ranges` or `fqdns` blocks are not specified or removed from an existing resource

ENHANCEMENTS:

* add `container_cluster_id` and `container_cluster_token` vars to `illumio-core_container_cluster` resource
    * `container_cluster_id` is a convenience var containing the cluster UUID (also present in the HREF)
    * `container_cluster_token` is only returned when the resource is first created and is needed to pair with the remote container cluster
* add `match_type` parameter allowing users to specify exact or partial name matching to the following data sources:
    * `illumio-core_container_cluster_workload_profiles`
    * `illumio-core_container_clusters`
    * `illumio-core_enforcement_boundaries`
    * `illumio-core_ip_lists`
    * `illumio-core_label_groups`
    * `illumio-core_label_types`
    * `illumio-core_labels`
    * `illumio-core_pairing_profiles`
    * `illumio-core_rule_sets`
    * `illumio-core_services`
    * `illumio-core_vens`
    * `illumio-core_virtual_services`
    * `illumio-core_workloads`

## 1.1.3 (Jun 21, 2023)

BUG FIXES:

* update client to not pass an error back if a DELETE returns a 406 `already-deleted` response

ENHANCEMENTS:

* set `ForceNew` on the deleted flag for the following resources and add customizers to ensure a remote deletion prompts Terraform to recreate the resource
    * `illumio-core_label`
    * `illumio-core_label_type`
    * `illumio-core_unmanaged_workload`

## 1.1.2 (Apr 5, 2023)

BUG FIXES:

* Fix `illumio-core_rule_set` resource to allow more than 3 label/label group refs in each scope

## 1.1.1 (Mar 23, 2023)

BUG FIXES:

* Fix resource updates broken by model changes
* Fix bugs and missing fields in several resources
* Fix consumer/provider exclusions not being applied for `illumio-core_security_rule` resources

ENHANCEMENTS:

* The following resources now allow create operations, and will "adopt" the remote object rather than returning an error:
    * illumio-core_firewall_settings
    * illumio-core_organization_settings
    * illumio-core_workload_settings

## 1.1.0 (Feb 24, 2023)

BUG FIXES:

* Allow arbitrary string values for label keys to work with label changes introduced in PCE v22.5

**NOTE:** this change, while backwards compatible, removes validation guarantees for older PCE versions. Versions prior to 22.5 still restrict key values to `role`, `app`, `env`, and `loc`.

* Several resources/data sources have been fixed so that import state mirrors the resource schema

NEW FEATURES:

* **New resource:** `label_type`

As of PCE v22.5, new label types can be configured (up to a default hard limit of 20 types). The `illumio-core_label_type` resource has been added to manage these types through the provider.

`illumio-core_label_type` and `illumio-core_label_types` data sources have also been added.

ENHANCEMENTS:

* Broad refactor to simplify object conversions

SCHEMA UPDATES:

* `resource.illumio-core_virtual_service` v2 - The `network_href` attribute has been changed to match the API schema and data source. It is now a nested object with an HREF attribute:

```
resource "illumio-core_virtual_service" "example" {
    ...

    network {
        href = "/orgs/1/networks/..."
    }
}
```

This change was necessary to align the import state with virtual service resource definitions.

* added `ven_type` nested fields to settings blocks in `illumio-core_workload_settings`

* added `use_workload_subnets` field to `illumio-core_security_rule`

## 1.0.3 (Dec 16, 2022)

BUG FIXES:

* Fix missing label sets in `unmanaged_workload` and `managed_workload` resource state

## 1.0.2 (Oct 4, 2022)

BUG FIXES:

* Fix issue where Label SubGroups whose parent group is not in the Terraform state will fail to provision
    * Add `/member_of` call to Label Group resource delete function to add parent group HREFs to provision list

ENHANCEMENTS:

* Refactor `provision` binary

## 1.0.1 (Sep 9, 2022)

BUG FIXES:

* Remove valid port slice constraint for service creation in favour of bounded int validation (-1 <= N <= 255) to support any IANA proto number

## 1.0.0 (Aug 19, 2022)  

BREAKING CHANGES:  

* **resource/workload_interface has been REMOVED**  
* **data_source/workload_interface has been REMOVED**  

The PCE `/workloads/:uuid/interfaces/:name` endpoint is incompatible with Terraform in its current implementation.
The API uses the interface name as a unique identifier, but interfaces were later changed to support multiple addresses (IPv4 and IPv6) without a change to the underlying representation expecting names and addresses to be 1-1. Since Terraform expects a unique relationship for resources to remote objects, it's not possible to define an interface with both an IPv4 and IPv6 address using HCL.  

The `/workloads` API, on the other hand, *does* return multiple interface objects with the same name, and a `/workloads` POST can be used to create an interface with both IPv4 and IPv6 addresses (though they still need to be defined as distinct objects!).  

As such, and because interfaces have no interaction with objects outside of the workload they belong to, the workload interface object is removed and the schema for both managed and unmanaged workloads have been updated to include an `interfaces` set.  

* **resource/vens_unpair has been REMOVED**  

VEN unpairing has been moved to the delete call for managed_workload resources to provide a workflow that is more consistent with Terraform's resource lifecycle. As such, the dedicated unpair resources are no longer needed.  

* **resource/workloads_unpair has been REMOVED**  

The PCE `/workloads/unpair` API has been deprecated in favour of `/vens/unpair` for a while, and is made doubly redundant by the addition of the managed_workload resource.  

* **resource/vens_upgrade has been REMOVED**  

vens_upgrade was another example of a utility resource that didn't conform to the Terraform resource lifecycle. Terraform already has the `null_resource` type and `local-exec` provisioner to handle these special cases if needed. An example of how to perform an upgrade using `local-exec` has been added to the `examples/ven_upgrade` folder.  

* **resource/workload has been REMOVED**

Workloads were split into the distinct managed_workload/unmanaged_workload resources in version 0.2.0. The workload resource simply duplicated unmanaged workload functionality, and is no longer necessary.

ENHANCEMENTS:

* Add `interfaces` back into managed/unmanaged workload schema  
* Add import to several objects
  * `resource/container_cluster_workload_profile`
  * `resource/enforcement_boundary`
  * `resource/ip_list`
  * `resource/pairing_profile`
  * `resource/rule_set`
  * `resource/security_rule`
  * `resource/service_binding`
  * `resource/syslog_destination`
  * `resource/traffic_collector_settings`
  * `resource/virtual_service`
  * `resource/vulnerability`
  * `resource/vulnerability_report`
* Fix docs typos

NOTES:  

* Bump pinned go version to 1.18
* Add arm64 to gox build targets

BUG FIXES:

* Extract parent HREF for child objects (rules/container workload profiles) on read to set for imports

## 0.2.2 (May 19, 2022)

NOTES:

* Remove reflective mapping of workload objects in favour of splitting managed/unmanaged model mapping

BUG FIXES:

* Fix `ignored_interface_names` parameter to be editable for `managed_workload` resources

## 0.2.1 (May 17, 2022)

NOTES:

* Clean up commented code from previous change

BUG FIXES:

* Fix bug causing managed workload update to fail in some cases

## 0.2.0 (May 10, 2022)

NOTES:

* resource/workload: DEPRECATED - use `resource/unmanaged_workload`. Will be removed in v1.0.0
* resource/workloads_unpair: DEPRECATED. Will be removed in v1.0.0
* resource/vens_unpair: DEPRECATED. Will be removed in v1.0.0
* Users should instead use the `destroy` lifecycle operation on imported `managed_workload` resources to unpair their VENs
* resource/vens_upgrade: DEPRECATED. Will be removed in v1.0.0

FEATURES:

* **New resource:** `managed_workload`
* **New resource:** `unmanaged_workload`
* resource/workload: Split managed/unmanaged workloads into separate resources
* Update workload model logic to accommodate both payloads

ENHANCEMENTS:

* Restructure and rewrite tests to be PCE-agnostic
* Improve documentation
* Improve HCL examples and remove duplicate JSON examples

BUG FIXES:

* Remove auto-provision from `resource/virtual_service` as the provision calls if the virtual service references other draft objects

## 0.1.1 (Oct 28, 2021)

ENHANCEMENTS:

* Minor bug fixes 
* Update examples and documentation

BUG FIXES:

* Fix race conditions for nested dependent objects (rule_set/rule, workload/workload_interface) [GH-15]
* Fix state inconsistency after `terraform apply` on `resource/pairing_key` [GH-17]

## 0.1.0 (May 31, 2021)

* Initial Release
