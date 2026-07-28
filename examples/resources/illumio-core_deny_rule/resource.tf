# A deny rule blocks the consumers (the sources that initiate the connection)
# from reaching the providers (the destinations) on the given services.
#
# Deny rules exist only below a rule set, at {rule_set_href}/deny_rules.

resource "illumio-core_rule_set" "app" {
  name = "RS-APP"

  scopes {
    label {
      href = var.env_label_href
    }
  }
}

# Ordinary deny rule: block external clients from reaching internal SSH.
# In the evaluation order override-deny > allow > deny, an allow rule can
# supersede this rule.
resource "illumio-core_deny_rule" "block_ssh" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = false
  description   = "Block external clients from internal SSH"

  # Destinations providing the service.
  providers {
    label {
      href = var.internal_label_href
    }
  }

  # Sources initiating the connection.
  consumers {
    ip_list {
      href = var.external_ip_list_href
    }
  }

  ingress_services {
    proto = "6"
    port  = "22"
  }
}

# Override-deny rule: the same resource type and the same endpoint, with
# override set to true. It still blocks traffic, but allow rules cannot
# supersede it.
resource "illumio-core_deny_rule" "block_rdp_unconditionally" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true
  override      = true

  providers {
    actors = "ams"
  }

  consumers {
    label {
      href = var.contractor_label_href
    }
  }

  ingress_services {
    proto   = "6"
    port    = "3389"
    to_port = "3395"
  }
}

# ICMP has no inline protocol/port representation in a deny rule. Reference an
# ICMP service by HREF instead of setting proto = "1".
resource "illumio-core_deny_rule" "block_icmp" {
  rule_set_href = illumio-core_rule_set.app.href
  enabled       = true

  providers {
    actors = "ams"
  }

  consumers {
    ip_list {
      href = var.external_ip_list_href
    }
  }

  ingress_services {
    href = var.icmp_service_href
  }
}
