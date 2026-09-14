terraform {
  required_providers {
    illumio-core = {
      source  = "alexgoller/illumio-core"
      version = ">= 2.3.0"
    }
  }
}

provider "illumio-core" {
  pce_host     = var.pce_url
  org_id       = var.pce_org_id
  api_username = var.pce_api_key
  api_secret   = var.pce_api_secret

  # This fork provisions through illumio-core_provisioning, so the legacy
  # hrefs.csv file is not needed.
  write_href_file = false
}
