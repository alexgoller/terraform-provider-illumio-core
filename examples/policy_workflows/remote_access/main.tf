###############################################################################
# Network location awareness: rules for endpoints off the corporate network
#
# `network_type` scopes a rule by where the endpoint is. It is the UI's
# "All networks" option under Rule Options, and it exists because a laptop at
# home is not the same risk as the same laptop in the office.
#
#   brn       on the corporate network  (the PCE's default when unset)
#   non_brn   off it
#   all       either
#
# One constraint the API documentation does not state anywhere: `all` and
# `non_brn` accept ONLY IP lists as actors. Anything else is rejected with
#
#   406 non_brn_must_use_ip_list
#   A rule with Network Type "All" or "Non-Corporate" (Endpoints only) must
#   have only IP lists on consumers or providers
#
# So the consumer side here is an IP list, not a label.
###############################################################################

resource "illumio-core_label" "app" {
  key   = "app"
  value = var.app
}

resource "illumio-core_label" "env" {
  key   = "env"
  value = var.env
}

resource "illumio-core_ip_list" "vpn_pool" {
  name        = "${var.app}-vpn-pool"
  description = "Addresses issued to remote endpoints"

  dynamic "ip_ranges" {
    for_each = var.vpn_ranges
    content {
      from_ip = ip_ranges.value
    }
  }
}

resource "illumio-core_service" "https" {
  name = "${var.app}-https"

  service_ports {
    proto = 6
    port  = 443
  }
}

resource "illumio-core_rule_set" "this" {
  name        = "${var.app} | ${var.env} | remote access"
  description = "Access to ${var.app} from endpoints on and off the corporate network"
  enabled     = true

  scopes {
    label { href = illumio-core_label.app.href }
    label { href = illumio-core_label.env.href }
  }
}

# Reachable from the VPN pool wherever the endpoint happens to be. Requires
# provider 2.2.0 or later - before that, network_type was settable on deny
# rules but not on allow rules.
resource "illumio-core_security_rule" "from_anywhere" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "HTTPS from the VPN pool, on or off the corporate network"
  network_type  = "all"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }

  # An IP list, because network_type = "all" permits nothing else.
  consumers {
    ip_list { href = illumio-core_ip_list.vpn_pool.href }
  }

  ingress_services { href = illumio-core_service.https.href }
}

# For comparison: omitting network_type leaves the PCE's own default in place.
# The provider deliberately does not send the field unless the configuration
# sets it, so Terraform never writes "brn" over a value chosen elsewhere.
resource "illumio-core_security_rule" "corporate_only" {
  rule_set_href = illumio-core_rule_set.this.href
  enabled       = true
  description   = "Intra-application traffic, corporate network only"

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  providers { actors = "ams" }
  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.https.href }
}

resource "illumio-core_provisioning" "policy" {
  hrefs = [
    illumio-core_rule_set.this.href,
    illumio-core_ip_list.vpn_pool.href,
    illumio-core_service.https.href,
  ]

  update_description = "Remote access for ${var.app} ${var.env}"

  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.this,
      illumio-core_security_rule.from_anywhere,
      illumio-core_security_rule.corporate_only,
      illumio-core_ip_list.vpn_pool,
      illumio-core_service.https,
    ]
  }
}
