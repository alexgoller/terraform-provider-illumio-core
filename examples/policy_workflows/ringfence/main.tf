###############################################################################
# Ring-fencing an application
#
# The smallest useful policy: workloads inside one (app, env) scope may talk to
# each other, and nothing else may reach them. This is usually the first thing
# to apply to an application, and often the only thing it ever needs.
#
# Everything is scoped by the (app, env) tuple. Labels combine as OR within a
# label type and AND across types, so the two labels in one `scopes` block
# select the intersection: workloads that are both app=erp and env=prod.
###############################################################################

resource "illumio-core_label" "app" {
  key   = "app"
  value = var.app
}

resource "illumio-core_label" "env" {
  key   = "env"
  value = var.env
}

resource "illumio-core_service" "intra_app" {
  name        = "${var.app}-intra"
  description = "Ports the application uses internally"

  service_ports {
    proto = 6
    port  = 8080
  }

  service_ports {
    proto = 6
    port  = 8443
  }
}

resource "illumio-core_rule_set" "this" {
  name        = "${var.app} | ${var.env}"
  description = "Ring-fence for ${var.app} in ${var.env}"
  enabled     = true

  # Both labels in ONE scopes block. Two separate blocks would mean two
  # scopes - app=erp anywhere, OR env=prod anywhere - which is far wider
  # than intended and fails silently.
  scopes {
    label { href = illumio-core_label.app.href }
    label { href = illumio-core_label.env.href }
  }
}

# "ams" is All Managed Systems. Inside a scoped rule set it means every
# workload in the scope, so this rule is the ring-fence itself: anything in
# the scope may reach anything else in the scope, on these services.
resource "illumio-core_security_rule" "ringfence" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "Intra-scope: ${var.app} workloads may talk to each other"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.intra_app.href }
}

# Policy lands in the PCE's draft version and does nothing until provisioned.
#
# Every object in `hrefs` must also appear in `replace_triggered_by`, along with
# the rules - a rule's HREF does not change when its contents change, so without
# this an edit is applied to the draft and never provisioned. The apply still
# reports success, which is what makes it easy to miss.
resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.this.href,
    illumio-core_service.intra_app.href,
  ]

  update_description = "Ring-fence ${var.app} ${var.env}"

  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.this,
      illumio-core_security_rule.ringfence,
      illumio-core_service.intra_app,
    ]
  }
}
