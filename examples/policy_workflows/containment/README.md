# Estate-wide ransomware containment

Closes the protocols lateral movement relies on — SMB, RDP, WinRM — across every
workload, with one exemption list that the security team owns.

```
terraform init
terraform apply -var 'exempt_roles=["file-server","jump-host","backup"]'
```

> Apply this to a test PCE first. It denies three protocols estate-wide, and the
> exemption list is the only thing standing between that and a support call.

## What it shows

**An unscoped rule set.** A `scopes` block with no labels in it is the PCE's
"All | All | All" global scope. The block itself is required — omitting it is a
validation error, not a shortcut.

**`override = true`, so application teams cannot punch through.** That is the
whole difference between a containment layer and a suggestion.

**Exclusions as the exemption mechanism.** Each rule denies the protocol to
every workload *except* members of a label group:

```hcl
providers {
  exclusion = true
  label_group {
    href = illumio-core_label_group.exempt.href
  }
}
```

Note that `exclusion` sits **beside** `label_group`, not inside it, and is valid
only for `label` and `label_group` actors — not workloads or IP lists. That is
why the exemption list is a label group.

Adding a server to the list is then a pull request against this file, reviewable
by whoever owns it. The request path for an exception is enforced by the policy
engine rather than by a wiki page.

## Enforcement mode still decides what actually blocks

Rules are programmed onto every workload regardless of its enforcement mode.
What changes is the last-rule behaviour: in **Visibility Only** nothing is
blocked, in **Selective** the default-deny applies only within enforced
boundaries, and in **Full** it applies to everything. Applying this file changes
what the PCE *would* do; moving workloads to full enforcement is a separate,
deliberate step.
