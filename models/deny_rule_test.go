// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// ptr is a local test helper; the models package has no pointer utility of its
// own (illumio-core.PtrTo lives in the provider package).
func ptr[T any](v T) *T { return &v }

func marshalDenyRule(t *testing.T, dr *DenyRule) (string, map[string]interface{}) {
	t.Helper()
	b, err := json.Marshal(dr)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	return string(b), m
}

func minimalDenyRule() *DenyRule {
	return &DenyRule{
		Enabled:         true,
		Providers:       []*DenyRuleActor{{Actors: "ams"}},
		Consumers:       []*DenyRuleActor{{IPList: &Href{Href: "/orgs/1/sec_policy/draft/ip_lists/1"}}},
		IngressServices: []IngressService{{Port: ptr(22), Proto: ptr(6)}},
	}
}

// The brief lists fields that earlier iterations invented and that live-PCE
// validation disproved. None of them may ever reach the wire.
func TestDenyRuleNeverSerializesDisprovedFields(t *testing.T) {
	dr := minimalDenyRule()
	dr.Override = true
	dr.Description = ptr("x")
	dr.NetworkType = "brn"
	dr.UnscopedConsumers = ptr(true)
	dr.EgressServices = []Href{{Href: "/orgs/1/sec_policy/draft/services/3"}}

	raw, m := marshalDenyRule(t, dr)

	disproved := []string{
		// Invented by the original "Add deny lists" work.
		"action", "priority", "overrides", "resolve_labels_as",
		// Allow-only controls with no schema proof for deny rules.
		"sec_connect", "stateless", "machine_auth",
		"consuming_security_principals", "use_workload_subnets",
		// Not present on the live object; must not be inferred from other
		// policy objects.
		"name", "external_data_set", "external_data_reference",
		// There is no separate override-deny collection.
		"override_deny_rules",
	}
	for _, field := range disproved {
		if _, ok := m[field]; ok {
			t.Errorf("deny rule serialized disproved field %q: %s", field, raw)
		}
	}
}

// override=false is an ordinary deny rule and override=true an override-deny
// rule. Both are the same object at the same endpoint, so the flag must always
// be present and must never be dropped when false.
func TestDenyRuleOverrideAlwaysSerialized(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"ordinary deny", false},
		{"override deny", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dr := minimalDenyRule()
			dr.Override = tc.want

			raw, m := marshalDenyRule(t, dr)

			got, ok := m["override"]
			if !ok {
				t.Fatalf("override missing from payload: %s", raw)
			}
			if got != tc.want {
				t.Errorf("override = %v, want %v", got, tc.want)
			}
		})
	}
}

// enabled=false must survive too - the provider owns this value.
func TestDenyRuleEnabledFalseSerialized(t *testing.T) {
	dr := minimalDenyRule()
	dr.Enabled = false

	raw, m := marshalDenyRule(t, dr)
	got, ok := m["enabled"]
	if !ok {
		t.Fatalf("enabled missing from payload: %s", raw)
	}
	if got != false {
		t.Errorf("enabled = %v, want false", got)
	}
}

// egress_services are service references, never inline port/proto.
func TestDenyRuleEgressServicesAreHrefObjects(t *testing.T) {
	dr := minimalDenyRule()
	dr.EgressServices = []Href{{Href: "/orgs/1/sec_policy/draft/services/3"}}

	raw, m := marshalDenyRule(t, dr)

	es, ok := m["egress_services"].([]interface{})
	if !ok || len(es) != 1 {
		t.Fatalf("egress_services not a 1-element array: %s", raw)
	}
	entry := es[0].(map[string]interface{})
	if entry["href"] != "/orgs/1/sec_policy/draft/services/3" {
		t.Errorf("egress_services[0] = %v, want an href object", entry)
	}
	if len(entry) != 1 {
		t.Errorf("egress_services[0] has extra keys: %v", entry)
	}
}

// Omitted optional fields must not be sent, so the PCE applies its own
// defaults rather than the provider silently forcing a value.
func TestDenyRuleOmitsUnsetOptionalFields(t *testing.T) {
	raw, m := marshalDenyRule(t, minimalDenyRule())

	for _, field := range []string{
		"description",
		"network_type",
		"unscoped_consumers",
		"all_ips_except_for_in_consumers",
		"all_ips_except_for_in_providers",
		"egress_services",
	} {
		if _, ok := m[field]; ok {
			t.Errorf("unset optional field %q was sent: %s", field, raw)
		}
	}

	// ...but the required collections are always present.
	for _, field := range []string{"providers", "consumers", "ingress_services", "enabled", "override"} {
		if _, ok := m[field]; !ok {
			t.Errorf("required field %q missing: %s", field, raw)
		}
	}
}

// An explicit false on an optional boolean must be sent, not swallowed.
func TestDenyRuleExplicitFalseOptionalBooleans(t *testing.T) {
	dr := minimalDenyRule()
	dr.UnscopedConsumers = ptr(false)
	dr.AllIPsExceptForInConsumers = ptr(false)
	dr.AllIPsExceptForInProviders = ptr(false)

	raw, m := marshalDenyRule(t, dr)
	for _, field := range []string{
		"unscoped_consumers",
		"all_ips_except_for_in_consumers",
		"all_ips_except_for_in_providers",
	} {
		v, ok := m[field]
		if !ok {
			t.Errorf("explicit false %q was dropped: %s", field, raw)
			continue
		}
		if v != false {
			t.Errorf("%s = %v, want false", field, v)
		}
	}
}

// DenyRuleActor must not carry virtual_service or virtual_server: the
// authoritative deny_rule_actor schema does not permit them, though the
// allow-rule actor types do.
func TestDenyRuleActorRejectsVirtualActors(t *testing.T) {
	dr := minimalDenyRule()
	dr.Providers = []*DenyRuleActor{{Label: &LabelOptionalKeyValue{Href: "/orgs/1/labels/1"}}}

	raw, _ := marshalDenyRule(t, dr)
	for _, field := range []string{"virtual_service", "virtual_server"} {
		if strings.Contains(raw, field) {
			t.Errorf("payload mentions %q, which is not a valid deny-rule actor: %s", field, raw)
		}
	}
}

func TestDenyRuleActorHasOneActor(t *testing.T) {
	tests := []struct {
		name  string
		actor *DenyRuleActor
		want  bool
	}{
		{"ams only", &DenyRuleActor{Actors: "ams"}, true},
		{"label only", &DenyRuleActor{Label: &LabelOptionalKeyValue{Href: "/orgs/1/labels/1"}}, true},
		{"ip_list only", &DenyRuleActor{IPList: &Href{Href: "/orgs/1/sec_policy/draft/ip_lists/1"}}, true},
		{"label_group only", &DenyRuleActor{LabelGroup: &Href{Href: "/orgs/1/sec_policy/draft/label_groups/x"}}, true},
		{"workload only", &DenyRuleActor{Workload: &Href{Href: "/orgs/1/workloads/x"}}, true},
		// exclusion is a modifier, not an actor, so it must not count.
		{"label plus exclusion", &DenyRuleActor{Label: &LabelOptionalKeyValue{Href: "/orgs/1/labels/1"}, Exclusion: ptr(true)}, true},
		{"none", &DenyRuleActor{}, false},
		{"two actors", &DenyRuleActor{Actors: "ams", IPList: &Href{Href: "/orgs/1/sec_policy/draft/ip_lists/1"}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasOneActor(tt.actor); got != tt.want {
				t.Errorf("HasOneActor() = %v, want %v", got, tt.want)
			}
		})
	}
}
