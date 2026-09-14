###############################################################################
# Estate-wide ransomware containment
#
# Closes the protocols lateral movement relies on across every workload, with a
# single exemption list that the security team owns.
#
# Two things make this work:
#
#   * an UNSCOPED rule set - a `scopes` block with no labels in it is the PCE's
#     "All | All | All" global scope. The block is required; omitting it is a
#     validation error.
#   * override = true, so no allow rule written by any application team can
#     punch through it. That is the difference between a containment layer and
#     a suggestion.
#
# Servers that legitimately serve these protocols go in a label group, added as
# an EXCLUSION on the providers side. Requests to be exempted are then a pull
# request against this file, reviewable by whoever owns it.
###############################################################################

resource "illumio-core_label" "exempt" {
  for_each = toset(var.exempt_roles)

  key   = "role"
  value = each.key
}

resource "illumio-core_label_group" "exempt" {
  key         = "role"
  name        = "containment-exempt"
  description = "Roles permitted to serve lateral-movement protocols"

  dynamic "labels" {
    for_each = illumio-core_label.exempt
    content {
      href = labels.value.href
    }
  }
}

resource "illumio-core_service" "smb" {
  name        = "containment-smb"
  description = "SMB / CIFS"

  service_ports {
    proto = 6
    port  = 445
  }
}

resource "illumio-core_service" "rdp" {
  name        = "containment-rdp"
  description = "Remote Desktop"

  service_ports {
    proto = 6
    port  = 3389
  }
}

resource "illumio-core_service" "winrm" {
  name        = "containment-winrm"
  description = "Windows Remote Management"

  service_ports {
    proto = 6
    port  = 5985
  }

  service_ports {
    proto = 6
    port  = 5986
  }
}

# No labels in the scopes block: this applies across the whole estate.
resource "illumio-core_rule_set" "containment" {
  name        = "containment | fleet"
  description = "Estate-wide lateral movement controls"
  enabled     = true

  scopes {}
}

# One override-deny per protocol. Excluding the label group on the providers
# side means "every workload EXCEPT these roles may not be reached on this
# service" - so a file server still serves SMB, and nothing else does.
#
# exclusion is valid only for label and label_group actors, not workloads or
# IP lists, which is why the exemption list is a label group.
resource "illumio-core_deny_rule" "no_smb" {
  rule_set_href = illumio-core_rule_set.containment.href
  enabled       = true
  override      = true
  description   = "SMB only to designated file servers"

  providers {
    exclusion = true
    label_group {
      href = illumio-core_label_group.exempt.href
    }
  }

  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.smb.href }
}

resource "illumio-core_deny_rule" "no_rdp" {
  rule_set_href = illumio-core_rule_set.containment.href
  enabled       = true
  override      = true
  description   = "RDP only to designated jump hosts"

  providers {
    exclusion = true
    label_group {
      href = illumio-core_label_group.exempt.href
    }
  }

  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.rdp.href }
}

resource "illumio-core_deny_rule" "no_winrm" {
  rule_set_href = illumio-core_rule_set.containment.href
  enabled       = true
  override      = true
  description   = "WinRM only to designated management hosts"

  providers {
    exclusion = true
    label_group {
      href = illumio-core_label_group.exempt.href
    }
  }

  consumers { actors = "ams" }

  ingress_services { href = illumio-core_service.winrm.href }
}

resource "illumio-core_provisioning" "containment" {
  hrefs = [
    illumio-core_rule_set.containment.href,
    illumio-core_label_group.exempt.href,
    illumio-core_service.smb.href,
    illumio-core_service.rdp.href,
    illumio-core_service.winrm.href,
  ]

  update_description = "Estate-wide containment"

  lifecycle {
    replace_triggered_by = [
      illumio-core_rule_set.containment,
      illumio-core_deny_rule.no_smb,
      illumio-core_deny_rule.no_rdp,
      illumio-core_deny_rule.no_winrm,
      illumio-core_label_group.exempt,
      illumio-core_service.smb,
      illumio-core_service.rdp,
      illumio-core_service.winrm,
    ]
  }
}
