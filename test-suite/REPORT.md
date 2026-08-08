# Acceptance test report — PCE 25.2.10

~> This records the run made against provider **2.0.11**. Later releases are
not covered by it; rerun the suite to refresh.

Latest full run of `test-suite/`. Regenerate by running the suite and updating
this file; see [README.md](README.md) for how to run individual steps.

## Environment

| | |
|---|---|
| Date | 2026-08-08 |
| PCE | 25.2.10-24 |
| Terraform | v1.15.6 |
| Provider | `alexgoller/illumio-core` **2.0.11** |
| Installed from | Terraform Registry — `Installed alexgoller/illumio-core v2.0.11 (self-signed, key ID 74176BC264C8A136)` |
| Org | 1 |

The provider was installed from the public Registry rather than built locally, so
this run also exercises the release pipeline, the archive layout and the GPG
signature that Terraform verifies at install time.

## Result

**17 of 17 steps passed. No failures.**

Every step is verified by reading the object back from the PCE API after the
apply, so a pass means the object exists on the PCE, not merely in Terraform
state.

| # | Step | Result | Verified on the PCE |
|---|---|---|---|
| 1 | Label `app` | pass | `/orgs/1/labels/307` — `app` = `tf-suite-app` |
| 2 | Label `env` | pass | `/orgs/1/labels/308` — `env` = `tf-suite-env` |
| 3 | Label `role=web` | pass | `/orgs/1/labels/309` — `role` = `tf-suite-web` |
| 4 | Label `role=db` | pass | `/orgs/1/labels/310` — `role` = `tf-suite-db` |
| 5 | Label group | pass | `…/label_groups/4441297d-…` — key `role`, 2 labels |
| 6 | IP list (corporate) | pass | `…/ip_lists/145` — 1 range |
| 7 | IP list (untrusted, with FQDN) | pass | `…/ip_lists/147` — 1 range, 1 FQDN |
| 8 | Service HTTPS | pass | `…/services/593` — 1 service port |
| 9 | Service SSH | pass | `…/services/595` — 1 service port |
| 10 | Rule set with AND-ed scope | pass | `…/rule_sets/1056` — 1 scope holding both labels |
| 11 | Security rule (ring-fence, ams→ams) | pass | `…/sec_rules/1806` — providers, consumers, services each 1 |
| 12 | Security rule (web→db) | pass | `…/sec_rules/1808` — tiered by `role` labels |
| 13 | Deny rule | pass | `…/deny_rules/483` — `override = false` |
| 14 | Override-deny rule | pass | `…/deny_rules/485` — `override = true` |
| 15 | Enforcement boundary | pass | `…/enforcement_boundaries/487` |
| 16 | Unmanaged workload | pass | `/orgs/1/workloads/cada7ca2-…` — 3 labels |
| 17 | Provisioning (explicit HREFs) | pass | policy version **257**, pending after provisioning `[]` |

### Regression check: edits converge in one apply

Not part of the numbered steps. Added after an object listed only in `hrefs` was
found to be provisioned on creation and never again — see the
[policy provisioning guide](../docs/guides/policy-provisioning.md).

| Check | Result |
|---|---|
| Edit an IP list's description, then apply once | `Apply complete! Resources: 1 added, 1 changed, 1 destroyed` |
| Did provisioning run in that same apply? | yes — the provisioning resource was replaced |
| Objects left in draft afterwards | none |

The IP list is the case that used to fail silently: its HREF does not change when
its contents change, so before it was added to `replace_triggered_by` the apply
succeeded while leaving the object inactive.

### Teardown

Destroying provisioned policy takes more than one pass, because the still-active
policy holds references to the objects being deleted.

| Pass | Result |
|---|---|
| 1 — `terraform destroy` | partial, as expected. Refused with `label_referenced_by_label_group` and `label_still_has_associated_rule_set` |
| 2 — provision the pending deletions | `POST /sec_policy` → **201** |
| 3 — `terraform destroy` | complete |

Both refusal tokens appeared in the same run, which is why the guide documents
both: a label held by a rule set scope and a label held by a label group fail
differently.

## Post-run state

Checked independently after teardown, not using the suite's own reporting:

| Collection | `tf-suite` objects remaining |
|---|---|
| `labels` | 0 |
| `sec_policy/draft/label_groups` | 0 |
| `sec_policy/draft/ip_lists` | 0 |
| `sec_policy/draft/services` | 0 |
| `sec_policy/draft/rule_sets` | 0 |
| `sec_policy/active/rule_sets` | 0 |
| `sec_policy/draft/enforcement_boundaries` | 0 |
| `workloads` | 0 |

Terraform state: **0 resources**.

## Isolation

The PCE is shared, and three rule sets belonging to another user were pending
before the run began. That count was checked before the run, after provisioning,
after the update check, and after teardown:

```
before:            3 pending
after step 17:     3 pending
after the edit:    3 pending
after teardown:    3 pending
```

Provisioning was always scoped to an explicit `change_subset` naming only this
suite's own HREFs. `provision_all_pending` was never used — on a shared PCE it
would have activated the other user's unreviewed drafts.

## What this run does not cover

Stated so the passes above are not read as broader than they are.

- **`unpair_on_destroy`** for managed workloads. Deliberately untested: it
  triggers a full VEN deinstall on a real host.
- **Managed workload adoption**, which needs a VEN-paired workload on the PCE.
- **Nested label groups** (`sub_groups`).
- **AsyncGet truncation** above 500 results, verified on a 26.x PCE but not here.
- **RBAC and virtual servers**, which are designs rather than implementations.
- **26.x behaviour.** Every result above is 25.2.10 only. The provider gates
  26.x-only fields such as `all_ips_except_for_in_*` behind `isConfigured`, and
  `illumio-core/schema_compat_test.go` guards that offline against the archived
  schemas — but that is a unit test, not a live 26.x run.
