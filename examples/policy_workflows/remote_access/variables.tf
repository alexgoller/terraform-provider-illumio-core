# Configure the PCE connection information as TF variables.

variable "pce_url" {
  type        = string
  description = "URL of the Illumio PCE to connect to"
}

variable "pce_org_id" {
  type        = number
  description = "Illumio PCE Organization ID number"
  default     = 1
}

variable "pce_api_key" {
  type        = string
  description = "Illumio PCE API key username"
  sensitive   = true
}

variable "pce_api_secret" {
  type        = string
  description = "Illumio PCE API key secret"
  sensitive   = true
}

variable "app" {
  type        = string
  description = "Value of the app label"
  default     = "portal"
}

variable "env" {
  type        = string
  description = "Value of the env label"
  default     = "prod"
}

variable "vpn_ranges" {
  type        = list(string)
  description = "CIDR ranges issued to remote endpoints"
  default     = ["10.200.0.0/16"]
}
