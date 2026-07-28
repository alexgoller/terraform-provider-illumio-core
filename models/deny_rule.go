// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

// DenyRuleActor is a single provider or consumer entry on a deny rule.
//
// Deny rules permit a narrower set of actors than allow rules: the
// authoritative deny_rule_actor schema allows only "ams", label, label_group,
// ip_list and workload. It has no virtual_service or virtual_server, which is
// why this type is declared separately instead of reusing
// SecurityRuleProvider/SecurityRuleConsumer - sharing those would make it
// trivially easy to serialize an actor the PCE rejects.
//
// Providers and consumers accept the same selectors, so one type serves both.
//
// Exclusion is a modifier rather than an actor and applies only to label and
// label_group entries.
type DenyRuleActor struct {
	Actors     string                 `json:"actors,omitempty"`
	Exclusion  *bool                  `json:"exclusion,omitempty"`
	Label      *LabelOptionalKeyValue `json:"label,omitempty"`
	LabelGroup *Href                  `json:"label_group,omitempty"`
	Workload   *Href                  `json:"workload,omitempty"`
	IPList     *Href                  `json:"ip_list,omitempty"`
}

// DenyRule represents a deny rule nested under a rule set at
// {rule_set_href}/deny_rules.
//
// A deny rule blocks the consumers (the sources that initiate the connection)
// from reaching the providers (the destinations) on the given services.
//
// Override distinguishes the two kinds of deny rule, which are otherwise the
// same object at the same endpoint:
//
//   - false: an ordinary deny rule, superseded by allow and override-deny rules
//   - true:  an override-deny rule, which allow rules cannot override
//
// The policy evaluation order is override-deny > allow > deny. An override-deny
// rule still blocks traffic; it does not permit traffic that would otherwise be
// denied.
//
// This type deliberately omits fields that earlier iterations invented and
// live-PCE validation disproved: action, priority, overrides and
// resolve_labels_as. It also omits allow-only controls (sec_connect, stateless,
// machine_auth, consuming_security_principals, use_workload_subnets) and the
// name/external_data_* fields, none of which appear on a deny rule returned by
// the PCE.
type DenyRule struct {
	// Enabled and Override are non-pointer and have no omitempty: the provider
	// owns both values, so they are always sent. This keeps "not configured"
	// from being confused with false, which would otherwise let a server
	// default silently win and produce drift.
	Enabled  bool `json:"enabled"`
	Override bool `json:"override"`

	Description *string `json:"description,omitempty"`
	NetworkType string  `json:"network_type,omitempty"`

	// The provider does not own these defaults, so they stay pointers and are
	// omitted when unset, leaving the PCE to decide.
	UnscopedConsumers          *bool `json:"unscoped_consumers,omitempty"`
	AllIPsExceptForInConsumers *bool `json:"all_ips_except_for_in_consumers,omitempty"`
	AllIPsExceptForInProviders *bool `json:"all_ips_except_for_in_providers,omitempty"`

	Providers       []*DenyRuleActor `json:"providers"`
	Consumers       []*DenyRuleActor `json:"consumers"`
	IngressServices []IngressService `json:"ingress_services"`

	// EgressServices are service references only; unlike ingress services they
	// have no inline port/protocol form.
	EgressServices []Href `json:"egress_services,omitempty"`
}

// ToMap - Returns map for DenyRule model
func (dr *DenyRule) ToMap() (map[string]interface{}, error) {
	return toMap(dr)
}
