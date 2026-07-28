# Policy objects created by Terraform land in the PCE's "draft" version and only
# take effect once provisioned. This resource replaces the external `provision`
# binary and the hrefs.csv side channel.

# Set write_href_file = false on the provider so hrefs.csv, which is only
# consumed by the deprecated provision binary, is no longer written.

resource "illumio-core_ip_list" "external" {
  name = "External-IPs"

  ip_ranges {
    from_ip = "10.0.0.0/8"
  }
}

resource "illumio-core_rule_set" "app" {
  name = "RS-APP"

  scopes {
    label {
      href = var.env_label_href
    }
  }
}

resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.app.href,
    illumio-core_ip_list.external.href,
  ]

  update_description = "Provisioned by Terraform"

  # REQUIRED IN PRACTICE. A policy object's HREF does not change when its
  # contents change, so without this a contents-only edit is not provisioned
  # until the *following* apply, leaving policy in draft after a successful
  # apply. replace_triggered_by makes Terraform plan the provisioning run in the
  # same apply as the change.
  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.app,
      illumio-core_ip_list.external,
    ]
  }
}

output "policy_version" {
  value = illumio-core_provisioning.policy.version
}
