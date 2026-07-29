---
page_title: "Managed workloads"
subcategory: ""
description: |-
  Adopting VEN-paired workloads into Terraform by hostname or import, and what destroying one does.
---

# Managed workloads

A managed workload exists because a **VEN paired with the PCE**. Terraform
cannot create one — the create path has nothing to POST to. What Terraform can
do is *adopt* an existing workload and manage its labels, description and
enforcement settings.

That shapes the whole lifecycle:

| Terraform verb | What actually happens |
|---|---|
| create | Adopt an existing workload, found by hostname |
| read / update | Manage its labels and metadata |
| destroy | Stop managing it. The VEN stays paired |

## Adopting by hostname

Set `hostname` and apply. No import step:

```hcl
resource "illumio-core_managed_workload" "web" {
  hostname    = "web-01.example.com"
  description = "Managed by Terraform"

  labels {
    href = illumio-core_label.role_web.href
  }

  labels {
    href = illumio-core_label.env_prod.href
  }
}
```

Terraform finds the workload with that hostname and begins managing it. This is
the pattern `illumio-core_firewall_settings` already uses, where create resolves
the remote object rather than creating one.

### Adopting a fleet

Because adoption is just a resource argument, an estate is `for_each` over a
list of hostnames — no HREFs, no import commands:

```hcl
variable "managed_hostnames" {
  type = set(string)
  default = [
    "web-01.example.com",
    "web-02.example.com",
    "db-01.example.com",
  ]
}

resource "illumio-core_managed_workload" "fleet" {
  for_each = var.managed_hostnames

  hostname = each.value

  labels {
    href = illumio-core_label.env_prod.href
  }
}
```

### `hostname` is `ForceNew`

Changing `hostname` means adopting a **different** workload, not renaming one.
Terraform will therefore plan a replacement: it relinquishes the old workload
and adopts the new one. Neither is destroyed in the PCE.

To rename a workload, set `name` — that is a managed attribute. `hostname` is
set by the VEN and is only a selector here.

### When the workload does not exist

```
Error: No managed workload found to adopt

No workload has hostname "web-01.example.com". A managed workload only appears
once its VEN has paired with the PCE, so pair the host first, or correct the
hostname.
```

Adoption cannot pair a host. Pair the VEN first, then apply.

## Importing

Import remains available and is the better route for one-offs, for matching on
something other than hostname, or when the HREF is already known.

```bash
# Bare value: matched against hostname, then name, then external_data_reference
terraform import illumio-core_managed_workload.web "web-01.example.com"

# Pin the attribute explicitly
terraform import illumio-core_managed_workload.web "hostname=web-01.example.com"
terraform import illumio-core_managed_workload.web "name=WEB-01"
terraform import illumio-core_managed_workload.web "ip_address=10.0.0.5"
terraform import illumio-core_managed_workload.web "external_data_reference=cmdb-1234"

# The HREF still works
terraform import illumio-core_managed_workload.web \
  "/orgs/1/workloads/8a2b1c3d-0000-4000-8000-000000000000"
```

The same identifiers work in Terraform 1.5+ `import` blocks:

```hcl
import {
  for_each = var.managed_hostnames
  to       = illumio-core_managed_workload.fleet[each.value]
  id       = "hostname=${each.value}"
}
```

This all applies to `illumio-core_unmanaged_workload` too.

### Matches must be exact and unique

The PCE matches these query parameters **partially**, so a substring hit is
filtered out rather than importing the wrong host. An ambiguous value fails with
the candidates listed:

```
Error: 2 workloads have name="web", so the import is ambiguous.
Import by HREF instead, one of: /orgs/1/workloads/6c75..., /orgs/1/workloads/7c71...
```

`ip_address` is only tried for a bare value if that value parses as an IP
address — the PCE answers a non-IP with a 406, which would otherwise mask a
plain "not found".

## Destroying does not uninstall the agent

Destroying an `illumio-core_managed_workload` **relinquishes management**. The
resource leaves Terraform state; the VEN stays paired and the agent stays
installed. A warning confirms it:

```
Warning: Managed workload removed from Terraform state; the VEN is still paired
```

This differs from the upstream provider, which unpaired the VEN — uninstalling
the agent and restoring the host's firewall — on destroy. Since Terraform never
created the workload, uninstalling an agent it did not install is not a
reasonable consequence of removing a resource from a configuration.

### Decommissioning on purpose

```hcl
resource "illumio-core_managed_workload" "web" {
  hostname          = "web-01.example.com"
  unpair_on_destroy = true
}
```

With this set, `terraform destroy` unpairs the VEN, uninstalling the agent and
restoring the host's default firewall. Use it only when that is genuinely what
you want.

## Objects deleted outside Terraform

If a workload is deleted in the PCE, the next plan removes it from state and
plans to recreate it, rather than failing. For a managed workload "recreate"
means re-adopting by hostname, which fails cleanly if the VEN is no longer
paired.

## Unmanaged workloads

`illumio-core_unmanaged_workload` is a different thing: Terraform **does** create
those, and destroying one deletes it from the PCE. Only the import behaviour
described above is shared.
