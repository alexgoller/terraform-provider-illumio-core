###############################################################################
# Stepwise acceptance suite.
#
# Every resource is created and destroyed one at a time via suite.sh, so each
# step can be checked against the PCE before moving on. Everything is prefixed
# "tf-suite-" so it is unambiguous in the UI and safe to clean up.
###############################################################################

terraform {
  required_providers {
    illumio-core = { source = "illumio/illumio-core" }
  }
}

provider "illumio-core" {
  write_href_file = false
}

# 01-04 labels: the multidimensional model
resource "illumio-core_label" "app" {
  key   = "app"
  value = "tf-suite-app"
}

resource "illumio-core_label" "env" {
  key   = "env"
  value = "tf-suite-env"
}

resource "illumio-core_label" "role_web" {
  key   = "role"
  value = "tf-suite-web"
}

resource "illumio-core_label" "role_db" {
  key   = "role"
  value = "tf-suite-db"
}

# 05 label group
resource "illumio-core_label_group" "roles" {
  key         = "role"
  name        = "tf-suite-roles"
  description = "web and db roles"

  labels { href = illumio-core_label.role_web.href }
  labels { href = illumio-core_label.role_db.href }
}

# 06-07 ip lists
resource "illumio-core_ip_list" "corporate" {
  name        = "tf-suite-corporate"
  description = "corporate space"

  ip_ranges { from_ip = "10.0.0.0/8" }
}

resource "illumio-core_ip_list" "untrusted" {
  name        = "tf-suite-untrusted"
  description = "everything else"

  ip_ranges { from_ip = "0.0.0.0/0" }
  fqdns { fqdn = "tf-suite.example.com" }
}

# 08-09 services
resource "illumio-core_service" "https" {
  name = "tf-suite-https"

  service_ports {
    proto = 6
    port  = 443
  }
}

resource "illumio-core_service" "ssh" {
  name = "tf-suite-ssh"

  service_ports {
    proto = 6
    port  = 22
  }
}

# 10 rule set with an AND-ed app scope
resource "illumio-core_rule_set" "rs" {
  name        = "tf-suite | app | env"
  description = "stepwise suite"
  enabled     = true

  scopes {
    label { href = illumio-core_label.app.href }
    label { href = illumio-core_label.env.href }
  }
}

# 11 ring-fence rule
resource "illumio-core_security_rule" "ringfence" {
  rule_set_href = illumio-core_rule_set.rs.href
  enabled       = true
  description   = "ring-fence: intra-scope"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.https.href }
}

# 12 tiered rule, several actors via dynamic blocks
resource "illumio-core_security_rule" "web_to_db" {
  rule_set_href = illumio-core_rule_set.rs.href
  enabled       = true
  description   = "web tier to db tier"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers {
    label { href = illumio-core_label.role_db.href }
  }

  consumers {
    label { href = illumio-core_label.role_web.href }
  }

  ingress_services { href = illumio-core_service.https.href }
}

# 13 deny rule
resource "illumio-core_deny_rule" "no_ssh" {
  rule_set_href = illumio-core_rule_set.rs.href
  enabled       = true
  override      = false
  description   = "block ssh from untrusted"

  providers { actors = "ams" }

  consumers {
    ip_list { href = illumio-core_ip_list.untrusted.href }
  }

  ingress_services { href = illumio-core_service.ssh.href }
}

# 14 override-deny: same resource type, override = true
resource "illumio-core_deny_rule" "override_ssh" {
  rule_set_href = illumio-core_rule_set.rs.href
  enabled       = true
  override      = true
  description   = "override-deny: ssh, cannot be superseded"

  providers { actors = "ams" }

  consumers {
    ip_list { href = illumio-core_ip_list.corporate.href }
  }

  ingress_services { href = illumio-core_service.ssh.href }
}

# 15 enforcement boundary
resource "illumio-core_enforcement_boundary" "eb" {
  name = "tf-suite-boundary"

  ingress_services { href = illumio-core_service.ssh.href }
  providers { actors = "ams" }

  consumers {
    ip_list { href = illumio-core_ip_list.untrusted.href }
  }
}

# 16 unmanaged workload
resource "illumio-core_unmanaged_workload" "wl" {
  name     = "tf-suite-web-01"
  hostname = "tf-suite-web-01.example.com"

  labels { href = illumio-core_label.app.href }
  labels { href = illumio-core_label.env.href }
  labels { href = illumio-core_label.role_web.href }
}

# 17 provisioning — explicit hrefs only
resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.rs.href,
    illumio-core_ip_list.corporate.href,
    illumio-core_ip_list.untrusted.href,
    illumio-core_service.https.href,
    illumio-core_service.ssh.href,
    illumio-core_label_group.roles.href,
    illumio-core_enforcement_boundary.eb.href,
  ]

  update_description = "tf-suite"

  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.rs,
      illumio-core_security_rule.ringfence,
      illumio-core_security_rule.web_to_db,
      illumio-core_deny_rule.no_ssh,
      illumio-core_deny_rule.override_ssh,
      illumio-core_enforcement_boundary.eb,
    ]
  }
}
