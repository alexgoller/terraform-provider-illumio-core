---
page_title: "Using this fork instead of illumio/illumio-core"
subcategory: ""
description: |-
  How to install this fork of terraform-provider-illumio-core from the Terraform Registry, what it adds over the upstream provider, and how to migrate to and from it.
---

# Using this fork instead of `illumio/illumio-core`

This is a fork of [`illumio/terraform-provider-illumio-core`](https://github.com/illumio/terraform-provider-illumio-core),
published to the Terraform Registry as **`alexgoller/illumio-core`**.

It is a **separate provider** from the upstream `illumio/illumio-core`: a
different namespace, an independent version stream, and its own signing key.
Publishing it changed nothing about the upstream provider, and the two can even
be used side by side in different configurations.

## Installing

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "alexgoller/illumio-core"
      version = "2.0.11"
    }
  }
}
```

```bash
terraform init
```

That is the whole installation. Releases are GPG-signed, so Terraform verifies
them and writes a normal `.terraform.lock.hcl`.

The provider is published for 17 platforms — linux, darwin, windows, freebsd and
openbsd across `386`, `amd64`, `arm` and `arm64`. Linux builds are statically
linked, so they run on glibc and musl distributions alike.

~> **`source` is the only thing that distinguishes the two providers.** Both are
configured as `illumio-core` in HCL and expose the same resource names, so a
configuration reads identically either way. If you are unsure which one a
workspace uses, `terraform providers` prints the full source address.

## What this fork adds

| Change | Why it matters |
|---|---|
| `illumio-core_deny_rule` resource + `illumio-core_deny_rule` / `illumio-core_deny_rules` data sources | Deny rules and override-deny rules are not supported upstream at all |
| `illumio-core_provisioning` resource | Provisioning becomes a Terraform resource: state-tracked, visible in `plan`, and works in Terraform Cloud/CI, where the `provision` binary cannot |
| Adopt-on-create, and import by hostname | Existing PCE objects can be brought under Terraform without opaque-HREF imports |
| `provision` binary correctness fixes | Upstream could drop a policy change and still exit 0 |
| Build fix | Upstream `main` does not compile from a fresh clone |

Each of these is described in detail in
[Differences from the upstream provider](differences-from-upstream.html).

~> **Version numbers are not comparable between the two providers.** This fork's
`2.x` line and upstream's `1.x` line are unrelated: a higher number does not mean
newer or a superset. The `2.x` numbering predates publication, when the fork had
to sort above upstream to be served from a mirror under the same address. It is
kept for continuity.

## Migrating from the upstream provider

Change the `source`, then rewrite the provider address recorded in state:

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "alexgoller/illumio-core" # was "illumio/illumio-core"
      version = "2.0.11"
    }
  }
}
```

```bash
terraform state replace-provider \
  registry.terraform.io/illumio/illumio-core \
  registry.terraform.io/alexgoller/illumio-core

terraform init
terraform plan
```

`terraform plan` should report **no changes**. The resource schemas are
compatible, so nothing is recreated. If a plan proposes changes, stop and read
[Differences from the upstream provider](differences-from-upstream.html) before
applying.

Migrating back is the same operation with the arguments reversed:

```bash
terraform state replace-provider \
  registry.terraform.io/alexgoller/illumio-core \
  registry.terraform.io/illumio/illumio-core
```

Configurations that use this fork's extra resources must have those removed from
the configuration and from state before moving back, since upstream does not
define them:

```bash
terraform state rm illumio-core_provisioning.policy
terraform state rm illumio-core_deny_rule.example
```

Neither removal touches the PCE. `illumio-core_provisioning`'s delete is a no-op
by design, and removing a deny rule from state leaves the rule in place on the
PCE — delete it in the PCE UI if you no longer want it.

### Verifying which provider you are using

```bash
terraform providers
```

The source address is printed in full, so `registry.terraform.io/alexgoller/illumio-core`
confirms the fork.

A functional check, for a workspace whose configuration you cannot see: the fork
registers resources upstream does not have, so this succeeds only on the fork.

```hcl
data "illumio-core_deny_rules" "check" {
  rule_set_href = "/orgs/1/sec_policy/draft/rule_sets/1"
}
```

On the upstream provider it fails with `Invalid data source type`.

## Installing without the registry

Only needed for airgapped networks, or to test an unreleased build. Everyone
else should use the registry, which is simpler and verifies signatures.

### A provider mirror

`scripts/build-mirror.sh` builds one from source:

```bash
# Host platform only
scripts/build-mirror.sh

# A specific set, or everything
PLATFORMS="darwin_arm64 linux_amd64" scripts/build-mirror.sh
PLATFORMS=all VERSION=2.0.11 scripts/build-mirror.sh
```

It produces `mirror/registry.terraform.io/alexgoller/illumio-core/` containing a
`.zip` per platform plus the `index.json` and `<version>.json` files of the
provider network mirror protocol. The same directory serves as either a
filesystem mirror or a network mirror.

Point `~/.terraformrc` at it:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/absolute/path/to/mirror"
    include = ["registry.terraform.io/alexgoller/illumio-core"]
  }
  direct {
    exclude = ["registry.terraform.io/alexgoller/illumio-core"]
  }
}
```

For a team, serve the same directory over HTTPS and use `network_mirror` with
the identical `include`/`exclude` pair:

```hcl
provider_installation {
  network_mirror {
    url     = "https://example.com/terraform/"
    include = ["registry.terraform.io/alexgoller/illumio-core"]
  }
  direct {
    exclude = ["registry.terraform.io/alexgoller/illumio-core"]
  }
}
```

The `include`/`exclude` pair routes **only** this provider to the mirror, leaving
every other provider coming from the registry as normal.

### Development overrides

For iterating on the provider itself. `dev_overrides` bypasses `init` entirely
and prints a warning on every command, so it is not suitable for real use:

```hcl
provider_installation {
  dev_overrides {
    "alexgoller/illumio-core" = "/absolute/path/to/built/binary/dir"
  }
  direct {}
}
```

```bash
TF_CLI_CONFIG_FILE=dev.tfrc terraform plan
```

### Things to know about mirrors

- **`(unauthenticated)` is expected from a mirror.** Registry installs are
  GPG-signed and verified; mirrored ones are not. Installing from
  `alexgoller/illumio-core` on the registry *is* verified.
- **Network mirrors must be HTTPS.** Terraform rejects `http://` with
  `Invalid URL for provider installation source`, so a plain local HTTP server
  cannot be used for testing.
- **`terraform providers lock` does not work against a scoped mirror.** It fails
  with `the previously-selected version is no longer available`, because the
  provider is excluded from `direct` installation. Use the registry if you need
  a complete multi-platform lock file.
- **Bump the version whenever you rebuild**, or clear `.terraform/` in consuming
  configurations. Terraform caches by version, so rebuilding the same version
  does not reliably reach existing workspaces.

## Provider configuration

Configuration is unchanged from upstream, plus one new setting.

```hcl
provider "illumio-core" {
  pce_host     = "https://pce.example.com:8443"
  api_username = "api_xxxxxxxx"
  api_secret   = "..."
  org_id       = 1

  # New in this fork. Set to false when using illumio-core_provisioning,
  # so hrefs.csv is not written for the deprecated provision binary.
  write_href_file = false
}
```

All the usual environment variables still work: `ILLUMIO_PCE_HOST`,
`ILLUMIO_API_KEY_USERNAME`, `ILLUMIO_API_KEY_SECRET`, `ILLUMIO_PCE_ORG_ID`,
`ILLUMIO_ALLOW_INSECURE_TLS`, `ILLUMIO_CA_FILE`, `ILLUMIO_PROXY_URL`,
`ILLUMIO_PROXY_CREDENTIALS`.

## Using the new features

Each has its own guide:

- **[Deny rules](deny-rules.html)** — `illumio-core_deny_rule`, override-deny
  rules, the narrower actor set, and why ICMP needs a service reference.
- **[Policy provisioning](policy-provisioning.html)** —
  `illumio-core_provisioning` in place of `terraform apply && provision`, and
  why `replace_triggered_by` is required.
- **[Adopting existing objects](adopting-existing-objects.html)** — declaring
  labels, IP lists, label groups and rule sets that already exist on the PCE,
  without importing them.
- **[Managed workloads](managed-workloads.html)** — adopting VEN-paired
  workloads by hostname or import, and what destroying one does.

For a complete list of how this fork differs from upstream, including the
correctness fixes, see
**[Differences from the upstream provider](differences-from-upstream.html)**.

## Known limitations of the fork

- **Deny rules depend on an undocumented PCE feature.** The
  `{rule_set_href}/deny_rules` endpoint does not appear in the 26.3 OpenAPI
  specification, and its schema is flagged as a private-permission feature. It
  was verified against a live PCE at 26.30.2, but it will not work on a PCE
  where the feature is not enabled.
- **`terraform plan` needs PCE credentials** if you use
  `illumio-core_provisioning`, because it queries the pending policy set to
  decide whether provisioning is required.
- **`terraform destroy` does not provision deletions.** See the resource
  documentation for the workaround.
- **Not on the Terraform Registry.** You are responsible for distributing and
  updating the binary, and for keeping it in sync with upstream releases.
