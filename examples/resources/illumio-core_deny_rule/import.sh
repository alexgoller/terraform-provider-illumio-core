# Deny Rule objects can be imported from the Illumio PCE.
# The imported ID must match the HREF of the remote object.
# rule_set_href is recovered from the HREF during import.

terraform import illumio-core_deny_rule.example "/orgs/1/sec_policy/draft/rule_sets/1/deny_rules/1"
