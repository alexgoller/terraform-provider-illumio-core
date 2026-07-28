# Deny rules exist only below a rule set, so rule_set_href is required.
# Both ordinary deny rules and override-deny rules are returned; they are
# distinguished by the override attribute.

data "illumio-core_deny_rules" "example" {
  rule_set_href = "/orgs/1/sec_policy/draft/rule_sets/1"
}

output "override_deny_rules" {
  value = [for r in data.illumio-core_deny_rules.example.items : r.href if r.override]
}
