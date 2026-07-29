---
page_title: "Using this fork instead of illumio/illumio-core"
subcategory: ""
description: |-
  How to install and use this fork of terraform-provider-illumio-core, what it adds over the upstream provider, and how to migrate to and from it.
---

# Using this fork instead of `illumio/illumio-core`

This is a fork of [`illumio/terraform-provider-illumio-core`](https://github.com/illumio/terraform-provider-illumio-core).
It is **not published to the Terraform Registry**, so `source = "illumio/illumio-core"`
will always resolve to the upstream provider, never to this one. Installing the
fork therefore means telling Terraform explicitly where to find it.

## What this fork adds

| Change | Why it matters |
|---|---|
| `illumio-core_deny_rule` resource + `illumio-core_deny_rule` / `illumio-core_deny_rules` data sources | Deny rules and override-deny rules are not supported upstream at all |
| `illumio-core_provisioning` resource | Provisioning becomes a Terraform resource: state-tracked, visible in `plan`, and works in Terraform Cloud/CI, where the `provision` binary cannot |
| `provision` binary correctness fixes | Upstream could drop a policy change and still exit 0 |
| Build fix | Upstream `main` does not compile from a fresh clone |

### The build fix

Upstream's `.gitignore` contains `**/terraform.*`, which matches
`vendor/**/terraform.go`. Two vendored files were therefore never committed, and
`go build ./...` fails on a clean checkout:

```
hc-install/product/consul.go:18:60: undefined: simpleVersionRe
terraform-exec/tfexec/apply.go:109:11: undefined: Terraform
```

This fork narrows the pattern and restores the files. This affects anyone
building from source, not users of a released binary.

### The `provision` binary fixes

Upstream's binary had four stacked silent-failure paths: a hardcoded list of six
provisionable resource types, a warning-and-skip for unknown hrefs, a `switch`
with no `default` case in the change-subset model, and no non-zero exit on any
of them. A new provisionable object type would be recorded, warned about once,
dropped, and reported as success while the policy stayed in draft.

This fork derives the resource type from the href instead of a hardcoded list,
fails loudly, and returns a non-zero exit status. It also no longer panics on a
malformed `hrefs.csv` line, and no longer treats a missing `hrefs.csv` as fatal
— so `terraform apply && provision` stops failing on applies that changed no
policy objects.

## Installing the fork

### Option 1 — build from source with a filesystem mirror (recommended)

Best for real use: it works with `terraform init` and lockfiles, and does not
print a warning on every command.

```bash
git clone https://github.com/alexgoller/terraform-provider-illumio-core.git
cd terraform-provider-illumio-core
go build -o terraform-provider-illumio-core
```

Install it into the local plugin directory. Pick a version number for the fork
— it does not have to match upstream, and using a distinctive one such as
`0.0.1-fork` makes it obvious which provider is in use:

```bash
# Linux / macOS. Adjust OS_ARCH to match your machine, e.g. darwin_arm64,
# darwin_amd64, linux_amd64.
OS_ARCH="$(go env GOOS)_$(go env GOARCH)"
DEST=~/.terraform.d/plugins/registry.terraform.io/illumio/illumio-core/0.0.1-fork/${OS_ARCH}

mkdir -p "$DEST"
cp terraform-provider-illumio-core "$DEST/"
```

Then pin that version in your configuration:

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "illumio/illumio-core"
      version = "0.0.1-fork"
    }
  }
}
```

`terraform init` will find the local build. Because the version is pinned to one
that does not exist upstream, Terraform can never silently fall back to the
registry copy.

### Option 2 — development overrides

Best for quick evaluation. No `terraform init` required, but Terraform prints a
warning on every command and **ignores the dependency lock file**, so do not use
it for production state.

Create a CLI configuration file:

```hcl
# dev.tfrc
provider_installation {
  dev_overrides {
    "illumio/illumio-core" = "/absolute/path/to/build/directory"
  }
  direct {}
}
```

The path is the **directory** containing the binary, not the binary itself.

```bash
export TF_CLI_CONFIG_FILE=/absolute/path/to/dev.tfrc
terraform plan   # no terraform init needed
```

### Option 3 — private registry

If you run a private Terraform registry (Terraform Enterprise, Artifactory,
Cloudsmith), publish the fork there under your own namespace and reference it
normally:

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "your-org/illumio-core"
      version = ">= 0.0.1"
    }
  }
}
```

This is the only approach that works cleanly for Terraform Cloud remote runs,
because dev overrides and filesystem mirrors are not available there.

### Verifying which provider you are using

```bash
terraform providers
terraform version
```

A quick functional check: the fork registers resources upstream does not have,
so this succeeds only on the fork.

```hcl
data "illumio-core_deny_rules" "check" {
  rule_set_href = "/orgs/1/sec_policy/draft/rule_sets/1"
}
```

On the upstream provider it fails with `Invalid data source type`.

## Distributing the fork to testers (no registry)

While the fork is not published to a registry, the supported way to install it
is a **provider mirror**. `scripts/build-mirror.sh` builds one:

```bash
# Host platform only
scripts/build-mirror.sh

# A specific set, or everything
PLATFORMS="darwin_arm64 linux_amd64" scripts/build-mirror.sh
PLATFORMS=all VERSION=0.0.2 scripts/build-mirror.sh
```

It produces `mirror/registry.terraform.io/illumio/illumio-core/` containing a
`.zip` per platform plus the `index.json` / `<version>.json` files of the
provider network mirror protocol. The same directory works either way.

Serving the fork under `registry.terraform.io/illumio/illumio-core` is
deliberate: existing configurations keep `source = "illumio/illumio-core"` and
need no edits. Only the version pin changes.

### Local testing — filesystem mirror

Put this in `~/.terraformrc`:

```hcl
provider_installation {
  filesystem_mirror {
    path    = "/absolute/path/to/mirror"
    include = ["registry.terraform.io/illumio/illumio-core"]
  }
  direct {
    exclude = ["registry.terraform.io/illumio/illumio-core"]
  }
}
```

```hcl
terraform {
  required_providers {
    illumio-core = {
      source  = "illumio/illumio-core"
      version = "0.0.1"
    }
  }
}
```

`terraform init` then installs the fork:

```
- Installing illumio/illumio-core v0.0.1...
- Installed illumio/illumio-core v0.0.1 (unauthenticated)
```

Unlike `dev_overrides`, this is a real install: `init` works normally and a
`.terraform.lock.hcl` is written.

### Sharing with a team — network mirror

Serve the `mirror/` directory from any static HTTPS host (S3, GitHub Pages,
nginx):

```hcl
provider_installation {
  network_mirror {
    url     = "https://example.com/terraform/"
    include = ["registry.terraform.io/illumio/illumio-core"]
  }
  direct {
    exclude = ["registry.terraform.io/illumio/illumio-core"]
  }
}
```

The `include`/`exclude` pair matters: it routes **only** this provider to the
mirror, leaving every other provider coming from the registry as normal.

### Things to know

- **`(unauthenticated)` is expected.** Registry providers are GPG-signed by the
  publisher; mirrored providers are not. It is a statement of fact, not a
  warning that something is wrong.
- **Network mirrors must be HTTPS.** Terraform rejects `http://` URLs with
  `Invalid URL for provider installation source`, so a plain local HTTP server
  cannot be used for testing.
- **`terraform providers lock` does not work against a scoped mirror.** It
  fails with `the previously-selected version 0.0.1 is no longer available`,
  because the provider is excluded from `direct` installation. `terraform init`
  still records a hash for the platform it ran on. For a mixed-platform team,
  run `init` once per platform and combine the resulting lock entries, or accept
  the "incomplete lock file information" warning during testing.
- **Pick a version that upstream will never publish** (`0.0.1` here). Terraform
  resolves by version, so a distinctive one makes it impossible to silently get
  the upstream provider instead.
- **Bump the version whenever you rebuild**, or clear `.terraform/` in
  consuming configurations. Terraform caches by version, so rebuilding the same
  version does not reliably reach existing workspaces.

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

## Migrating from the upstream provider

The fork is a superset of upstream: no resource or data source was removed, and
no schema changed incompatibly. Existing configuration and state work unchanged.

1. Install the fork (above).
2. Run `terraform plan`. It should report no changes. If it does not, stop and
   investigate before applying — that would indicate an unintended schema
   difference and should be reported as a bug.
3. Adopt the new features when you want them; nothing is mandatory.

### Migrating back to upstream

Also non-destructive, **provided you are not using the fork-only resources**.
Remove any `illumio-core_deny_rule` and `illumio-core_provisioning` resources
from your configuration and state first:

```bash
terraform state rm illumio-core_provisioning.policy
terraform state rm illumio-core_deny_rule.example
```

Removing `illumio-core_provisioning` from state has no effect on the PCE — its
delete is a no-op by design. Removing a deny rule from state leaves the rule in
place on the PCE; delete it in the PCE UI if you no longer want it.

Then reinstate the upstream provider and re-run `terraform init -upgrade`.

## Using the new features

### Deny rules

Deny rules block traffic. Consumers are the sources that initiate the
connection; providers are the destinations.

```hcl
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false

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

Set `override = true` for an override-deny rule. It is the same object at the
same endpoint — the evaluation order is `override-deny > allow > deny`, and an
override-deny rule still *blocks* traffic; it does not permit traffic that would
otherwise be denied.

Two constraints differ from security rules:

- Valid actors are `actors = "ams"`, `label`, `label_group`, `ip_list` and
  `workload`. `virtual_service` and `virtual_server` are **not** valid and are
  rejected at plan time.
- ICMP has no inline form. `proto` accepts only `6` (TCP) and `17` (UDP);
  reference an ICMP service by `href` instead.

### Provisioning without the binary

```hcl
resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.app.href,
    illumio-core_ip_list.external.href,
  ]

  # Provision in the same apply that changes the policy.
  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.app,
      illumio-core_ip_list.external,
    ]
  }
}
```

The `replace_triggered_by` block is **not optional in practice**. A policy
object's HREF does not change when its contents change, so without it a
contents-only edit is not provisioned until the *following* apply — leaving
policy in draft after a successful apply.

This replaces `terraform apply && provision`. Set `write_href_file = false` on
the provider so `hrefs.csv` is no longer written.

## Workloads

Two upstream behaviours are changed here, both around managed workloads.

### Destroying a managed workload no longer uninstalls the agent

Upstream, `terraform destroy` on an `illumio-core_managed_workload` called
`PUT /orgs/{org}/vens/unpair` with `firewall_restore: "default"` — **uninstalling
the VEN from the host**.

That is a surprising amount of damage for removing a resource from a
configuration, and it is asymmetric: managed workloads cannot be created by
Terraform. They exist because a VEN paired with the PCE, and the provider's
create function refuses outright. Terraform adopts them by import — so it was
uninstalling an agent it never installed.

In this fork, destroying a managed workload **relinquishes management**: the
resource leaves Terraform state, the VEN stays paired, and a warning says so.

To decommission the agent as part of a destroy, opt in explicitly:

```hcl
resource "illumio-core_managed_workload" "web" {
  unpair_on_destroy = true
}
```

A related bug is fixed at the same time: the unpair call's result was discarded,
so a *failed* unpair reported success.

### Importing workloads without HREFs

A managed workload's HREF contains a PCE-generated UUID, so importing by HREF
means looking up a UUID in the PCE UI for every host. This fork accepts the
identifiers you already know:

```bash
terraform import illumio-core_managed_workload.web "web-01.example.com"
terraform import illumio-core_managed_workload.web "hostname=web-01.example.com"
terraform import illumio-core_managed_workload.web "name=WEB-01"
terraform import illumio-core_managed_workload.web "ip_address=10.0.0.5"
terraform import illumio-core_managed_workload.web "external_data_reference=cmdb-1234"
```

A bare value is matched against `hostname`, then `name`, then
`external_data_reference`. HREFs still work unchanged.

Matches must be **exact and unique**. The PCE matches these query parameters
partially, so a substring hit is filtered out rather than importing the wrong
host, and an ambiguous value fails with the candidate HREFs listed:

```
Error: 2 workloads have name="web", so the import is ambiguous.
Import by HREF instead, one of: /orgs/1/workloads/..., /orgs/1/workloads/...
```

This applies to `illumio-core_unmanaged_workload` too.

#### Adopting without importing

Setting `hostname` on a managed workload adopts the existing workload with that
hostname on `terraform apply` — no import step at all. `illumio-core_firewall_settings`
already behaves this way, resolving the remote object on create rather than
creating one.

```hcl
resource "illumio-core_managed_workload" "fleet" {
  for_each = toset(var.managed_hostnames)

  hostname = each.value

  labels {
    href = var.env_label_href
  }
}
```

`terraform apply` finds each workload and starts managing it. `terraform destroy`
relinquishes them again, leaving the VENs paired.

`hostname` is `ForceNew`: changing it means adopting a *different* workload, not
renaming one. If the hostname has no workload — usually because the VEN has not
paired yet — the apply fails with:

```
Error: No managed workload found to adopt
No workload has hostname "web-01.example.com". A managed workload only appears
once its VEN has paired with the PCE, so pair the host first, or correct the
hostname.
```

Both routes remain available. Use adoption for fleets you can name, and import
for one-offs, for workloads you would rather match on `name` or
`external_data_reference`, or when you already have the HREF.

#### Adopting a fleet

Terraform 1.5+ `import` blocks take the same identifiers, so an estate can be
adopted from a list of hostnames with no HREF lookups at all:

```hcl
variable "managed_hostnames" {
  type    = set(string)
  default = ["web-01.example.com", "web-02.example.com"]
}

import {
  for_each = var.managed_hostnames
  to       = illumio-core_managed_workload.fleet[each.value]
  id       = "hostname=${each.value}"
}

resource "illumio-core_managed_workload" "fleet" {
  for_each = var.managed_hostnames

  labels {
    href = var.env_label_href
  }
}
```

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
