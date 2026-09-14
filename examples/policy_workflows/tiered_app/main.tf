###############################################################################
# A tiered application: allow the flows that exist, deny the ones that do not
#
# web -> app -> db, each tier its own role label. Two things this shows that a
# pure allow policy cannot:
#
#   * a deny rule that stops the database tier initiating connections back up
#     the stack, which is what lateral movement looks like
#   * an override-deny that cannot be superseded by anyone's allow rule
#
# Evaluation order is:  override-deny  >  allow  >  deny
###############################################################################

resource "illumio-core_label" "app" {
  key   = "app"
  value = var.app
}

resource "illumio-core_label" "env" {
  key   = "env"
  value = var.env
}

resource "illumio-core_label" "role" {
  for_each = toset(["web", "app", "db"])

  key   = "role"
  value = "${var.app}-${each.key}"
}

resource "illumio-core_service" "https" {
  name = "${var.app}-https"

  service_ports {
    proto = 6
    port  = 443
  }
}

resource "illumio-core_service" "app_tier" {
  name = "${var.app}-app-tier"

  service_ports {
    proto = 6
    port  = 8080
  }
}

resource "illumio-core_service" "postgres" {
  name = "${var.app}-postgres"

  service_ports {
    proto = 6
    port  = 5432
  }
}

resource "illumio-core_service" "smb" {
  name        = "${var.app}-smb"
  description = "Used only to deny it"

  service_ports {
    proto = 6
    port  = 445
  }
}

resource "illumio-core_rule_set" "this" {
  name        = "${var.app} | ${var.env}"
  description = "Tiered policy for ${var.app}"
  enabled     = true

  scopes {
    label { href = illumio-core_label.app.href }
    label { href = illumio-core_label.env.href }
  }
}

###############################################################################
# Allow: the flows the application actually needs
#
# Each block holds exactly one actor. Two labels of different types in one
# actor list would be AND-ed - "web AND db", which matches nothing.
###############################################################################

resource "illumio-core_security_rule" "web_to_app" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "Web tier to application tier"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers {
    label { href = illumio-core_label.role["app"].href }
  }

  consumers {
    label { href = illumio-core_label.role["web"].href }
  }

  ingress_services { href = illumio-core_service.app_tier.href }
}

resource "illumio-core_security_rule" "app_to_db" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "Application tier to database"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers {
    label { href = illumio-core_label.role["db"].href }
  }

  consumers {
    label { href = illumio-core_label.role["app"].href }
  }

  ingress_services { href = illumio-core_service.postgres.href }
}

###############################################################################
# Deny: flows that should never happen
###############################################################################

# A database initiating a connection back to the web tier is a strong signal of
# compromise. override = false means a deliberate allow rule could still permit
# it - useful when you want the default to be "no" but keep an escape hatch
# that shows up in a pull request diff.
resource "illumio-core_deny_rule" "db_must_not_reach_web" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  override      = false
  description   = "Database tier must not initiate connections to the web tier"

  providers {
    label { href = illumio-core_label.role["web"].href }
  }

  consumers {
    label { href = illumio-core_label.role["db"].href }
  }

  ingress_services { href = illumio-core_service.https.href }
}

# override = true cannot be superseded by any allow rule, anywhere, including
# rules written by another team in another rule set. Reserve it for things that
# must never be permitted by accident - SMB between application workloads is
# the classic ransomware path.
resource "illumio-core_deny_rule" "no_smb" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  override      = true
  description   = "SMB is not a peer-to-peer protocol between app workloads"

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.smb.href }
}

resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.this.href,
    illumio-core_service.https.href,
    illumio-core_service.app_tier.href,
    illumio-core_service.postgres.href,
    illumio-core_service.smb.href,
  ]

  update_description = "Tiered policy for ${var.app} ${var.env}"

  # Every href above, plus every rule.
  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.this,
      illumio-core_security_rule.web_to_app,
      illumio-core_security_rule.app_to_db,
      illumio-core_deny_rule.db_must_not_reach_web,
      illumio-core_deny_rule.no_smb,
      illumio-core_service.https,
      illumio-core_service.app_tier,
      illumio-core_service.postgres,
      illumio-core_service.smb,
    ]
  }
}
