# Stepwise acceptance suite

Creates one resource at a time against a real PCE, reads each one back from the
API to confirm it, then tears everything down in reverse.

Useful for accepting a provider build against a specific PCE version, where
running a 17-resource apply in one go tells you little about which resource
misbehaved.

```bash
cd test-suite

ENV_FILE=../.env-25.2 ./suite.sh list          # the 17 steps
ENV_FILE=../.env-25.2 ./suite.sh create 1      # one step, verified via the API
ENV_FILE=../.env-25.2 ./suite.sh create 1-5    # a range
ENV_FILE=../.env-25.2 ./suite.sh status        # what exists so far
ENV_FILE=../.env-25.2 ./suite.sh verify 10     # re-read a step from the PCE
ENV_FILE=../.env-25.2 ./suite.sh teardown      # the full two-pass teardown
ENV_FILE=../.env-25.2 ./suite.sh leftovers     # confirm the PCE is clean
```

`ENV_FILE` supplies `PCE_HOST`, `PCE_PORT`, `PCE_ORG_ID`, `API_KEY`,
`API_SECRET`. It defaults to `../.env`.

Provider resolution uses whatever `TF_CLI_CONFIG_FILE` points at, so this works
against a dev override or an installed build.

## What it covers

Labels, label group, IP lists (ranges and FQDNs), services, a rule set with an
AND-ed app scope, a ring-fence rule, a tiered role-to-role rule, a deny rule, an
override-deny rule, an enforcement boundary, an unmanaged workload, and
provisioning with explicit hrefs.

Everything is prefixed `tf-suite-`.

## Notes

Each step is confirmed by reading the object back from the PCE, so a step that
reports OK exists remotely, not merely in Terraform state.

`teardown` is deliberately three passes: destroy, provision the pending
deletions, destroy again. Objects the still-active policy references cannot be
deleted until their deletion is provisioned — see the
[policy provisioning guide](../docs/guides/policy-provisioning.md).

Provisioning always uses **explicit hrefs**, never `provision_all_pending`, so
unrelated draft work on a shared PCE is never activated.
