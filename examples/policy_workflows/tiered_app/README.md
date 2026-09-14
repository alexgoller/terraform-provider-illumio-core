# A tiered application, with deny rules

`web -> app -> db`, each tier its own role label. Shows two things a pure allow
policy cannot express.

```
terraform init
terraform apply -var app=payments -var env=prod
```

## What it shows

**Allow rules for the flows that exist.** Web reaches the app tier on 8080, the
app tier reaches the database on 5432. Each actor block holds exactly one
selector: two labels of different types in one block would be AND-ed, matching
nothing.

**A deny rule for a flow that should not exist.** A database initiating a
connection back to the web tier is a strong signal of compromise.
`override = false` means a deliberate allow rule could still permit it — useful
when the default should be "no" but you want an escape hatch that shows up in a
pull request diff.

**An override-deny that nobody can undo.** `override = true` cannot be
superseded by any allow rule, including one written by another team in another
rule set. Evaluation order is:

```
override-deny  >  allow  >  deny
```

Reserve it for things that must never be permitted by accident. SMB between
application workloads is the classic ransomware path.

## Deny rules are a fork-only feature

Upstream `illumio/illumio-core` has no deny rule support at all. The endpoint is
also flagged as a private-permission feature on the PCE, so it may not be
enabled everywhere — see the [deny rules guide](../../../docs/guides/deny-rules.md).
