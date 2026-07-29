# Managed workloads cannot be created by Terraform: they exist because a VEN
# paired with the PCE. Terraform adopts them by import.
#
# Importing by HREF means looking up a PCE-generated UUID for every host, so
# this provider also accepts the identifiers you already know.

# By hostname (tried first), then name, then external_data_reference:
terraform import illumio-core_managed_workload.web "web-01.example.com"

# Pin the attribute explicitly when a bare value would be ambiguous:
terraform import illumio-core_managed_workload.web "hostname=web-01.example.com"
terraform import illumio-core_managed_workload.web "name=WEB-01"
terraform import illumio-core_managed_workload.web "ip_address=10.0.0.5"
terraform import illumio-core_managed_workload.web "external_data_reference=cmdb-1234"

# The HREF still works, and is the way to resolve an ambiguous match:
terraform import illumio-core_managed_workload.web "/orgs/1/workloads/8a2b1c3d-0000-4000-8000-000000000000"

# Adopting a fleet. Terraform 1.5+ import blocks take the same identifiers, so a
# whole estate can be adopted from a list of hostnames without looking up a
# single HREF:
#
#   variable "managed_hostnames" {
#     type    = set(string)
#     default = ["web-01.example.com", "web-02.example.com"]
#   }
#
#   import {
#     for_each = var.managed_hostnames
#     to       = illumio-core_managed_workload.fleet[each.value]
#     id       = "hostname=${each.value}"
#   }
#
#   resource "illumio-core_managed_workload" "fleet" {
#     for_each = var.managed_hostnames
#     labels { href = var.env_label_href }
#   }
