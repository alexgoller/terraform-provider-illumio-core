// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

import "testing"

func TestResourceTypeFromHref(t *testing.T) {
	tests := []struct {
		name    string
		href    string
		want    string
		wantErr bool
	}{
		{"draft rule set", "/orgs/1/sec_policy/draft/rule_sets/3", "rule_sets", false},
		{"ip list", "/orgs/1/sec_policy/draft/ip_lists/7", "ip_lists", false},
		{"label group", "/orgs/2/sec_policy/draft/label_groups/abc-123", "label_groups", false},
		{"service", "/orgs/1/sec_policy/draft/services/3", "services", false},
		{"virtual service", "/orgs/1/sec_policy/draft/virtual_services/1", "virtual_services", false},
		// virtual_servers had no field on the old fixed change-subset struct.
		{"virtual server", "/orgs/1/sec_policy/draft/virtual_servers/1", "virtual_servers", false},
		{"enforcement boundary", "/orgs/1/sec_policy/draft/enforcement_boundaries/4", "enforcement_boundaries", false},
		// A singleton object with no trailing ID.
		{"firewall settings", "/orgs/1/sec_policy/draft/firewall_settings", "firewall_settings", false},
		// Nested objects provision via their parent rule set.
		{"nested sec rule", "/orgs/1/sec_policy/draft/rule_sets/3/sec_rules/5", "rule_sets", false},
		{"nested deny rule", "/orgs/1/sec_policy/draft/rule_sets/3/deny_rules/10", "rule_sets", false},
		// Any policy version, not just draft.
		{"active pversion", "/orgs/1/sec_policy/active/rule_sets/3", "rule_sets", false},
		{"numeric pversion", "/orgs/1/sec_policy/12/rule_sets/3", "rule_sets", false},
		// Leading/trailing whitespace and a full URL prefix must be tolerated.
		{"trailing slash", "/orgs/1/sec_policy/draft/rule_sets/3/", "rule_sets", false},

		{"not a policy object", "/orgs/1/labels/1", "", true},
		{"workload", "/orgs/1/workloads/abc", "", true},
		{"empty", "", "", true},
		{"sec_policy with no type", "/orgs/1/sec_policy/draft", "", true},
		{"sec_policy trailing", "/orgs/1/sec_policy/draft/", "", true},
		{"garbage", "not-an-href", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResourceTypeFromHref(tt.href)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("ResourceTypeFromHref(%q) = %q, want error", tt.href, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResourceTypeFromHref(%q) unexpected error: %v", tt.href, err)
			}
			if got != tt.want {
				t.Errorf("ResourceTypeFromHref(%q) = %q, want %q", tt.href, got, tt.want)
			}
		})
	}
}
