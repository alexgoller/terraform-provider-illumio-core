# Ring-fencing an application

The smallest useful Illumio policy: workloads inside one `(app, env)` scope may
talk to each other, and nothing else may reach them. This is usually the first
policy an application gets, and often the only one it ever needs.

```
terraform init
terraform apply -var app=erp -var env=prod
```

## What it shows

**Both labels in one `scopes` block.** Labels combine as OR within a label type
and AND across types, so one block holding `app` and `env` selects the
intersection. Two separate blocks would mean `app=erp` anywhere **or**
`env=prod` anywhere — far wider than intended, and it fails silently.

**`actors = "ams"` on both sides.** Inside a scoped rule set, All Managed
Systems means every workload in the scope, so the rule is the ring-fence itself.

**Provisioning wired correctly.** Policy lands in the PCE's draft version and
does nothing until provisioned. Every object in `hrefs` also appears in
`replace_triggered_by`, together with the rule — a rule's HREF does not change
when its contents change, so without that an edit is written to the draft and
never provisioned, while the apply still reports success.

## Tearing it down

`terraform destroy` takes more than one pass once policy is active: the PCE
refuses to delete objects the running policy still references. Provision the
pending deletions, then destroy again. See the
[policy provisioning guide](../../../docs/guides/policy-provisioning.md).
