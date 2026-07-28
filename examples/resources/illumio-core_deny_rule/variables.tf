variable "env_label_href" {
  type        = string
  description = "HREF of the label used as the rule set scope"
}

variable "internal_label_href" {
  type        = string
  description = "HREF of the label identifying internal workloads"
}

variable "contractor_label_href" {
  type        = string
  description = "HREF of the label identifying contractor workloads"
}

variable "external_ip_list_href" {
  type        = string
  description = "HREF of the IP list describing external sources"
}

variable "icmp_service_href" {
  type        = string
  description = "HREF of an ICMP service, used because ICMP has no inline representation"
}
