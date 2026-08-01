# Ring-fencing an application.
#
# The rule set's scope selects the assets the policy applies to; an "all
# workloads to all workloads" rule inside it then permits traffic within that
# scope and nothing else. This is the smallest useful ring-fence.

resource "illumio-core_rule_set" "sap_prod" {
  name        = "RS-SAP-PROD-ringfence"
  description = "Ring-fence: SAP production talks to itself"
  enabled     = true

  # One scope, both labels: app:sap AND env:prod.
  # Several label blocks belong in ONE scopes block - repeating the scopes
  # block would create a second, independent scope instead.
  scopes {
    label {
      href = illumio-core_label.app_sap.href
    }
    label {
      href = illumio-core_label.env_prod.href
    }
  }
}

resource "illumio-core_security_rule" "intra_scope" {
  rule_set_href = illumio-core_rule_set.sap_prod.href
  enabled       = true

  # "ams" means all workloads. Bounded by the rule set's scope, so this is
  # every SAP production workload rather than every workload in the org.
  providers {
    actors = "ams"
  }

  consumers {
    actors = "ams"
  }

  resolve_labels_as {
    providers = ["workloads"]
    consumers = ["workloads"]
  }

  ingress_services {
    href = data.illumio-core_service.all_services.href
  }
}

data "illumio-core_service" "all_services" {
  href = var.all_services_href
}
