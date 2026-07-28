// Copyright 2021 Illumio, Inc. All Rights Reserved.

package main

import (
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

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
			got, err := resourceTypeFromHref(tt.href)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("resourceTypeFromHref(%q) = %q, want error", tt.href, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("resourceTypeFromHref(%q) unexpected error: %v", tt.href, err)
			}
			if got != tt.want {
				t.Errorf("resourceTypeFromHref(%q) = %q, want %q", tt.href, got, tt.want)
			}
		})
	}
}

func TestParseHrefFile(t *testing.T) {
	t.Run("does not panic on a line with no comma", func(t *testing.T) {
		// The previous implementation did row[0], row[1] after a Split on ",",
		// which panicked with index out of range on any comma-less line.
		entries, errs := parseHrefFile(strings.NewReader("/orgs/1/sec_policy/draft/rule_sets/3\n"))
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(entries) != 1 || entries[0].ResourceType != "rule_sets" {
			t.Fatalf("got %+v, want one rule_sets entry", entries)
		}
	})

	t.Run("accepts the legacy two-column format", func(t *testing.T) {
		entries, errs := parseHrefFile(strings.NewReader("rule_sets,/orgs/1/sec_policy/draft/rule_sets/3\n"))
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(entries) != 1 || entries[0].ResourceType != "rule_sets" {
			t.Fatalf("got %+v, want one rule_sets entry", entries)
		}
	})

	t.Run("derives the type from the href, not the stale first column", func(t *testing.T) {
		// The recorded column is advisory; the href is authoritative.
		entries, errs := parseHrefFile(strings.NewReader("wrong_type,/orgs/1/sec_policy/draft/ip_lists/7\n"))
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if entries[0].ResourceType != "ip_lists" {
			t.Errorf("ResourceType = %q, want ip_lists", entries[0].ResourceType)
		}
	})

	t.Run("skips blank and whitespace-only lines", func(t *testing.T) {
		in := "\n   \n\nrule_sets,/orgs/1/sec_policy/draft/rule_sets/3\n\n"
		entries, errs := parseHrefFile(strings.NewReader(in))
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
	})

	t.Run("deduplicates repeated hrefs preserving order", func(t *testing.T) {
		in := strings.Join([]string{
			"rule_sets,/orgs/1/sec_policy/draft/rule_sets/3",
			"ip_lists,/orgs/1/sec_policy/draft/ip_lists/7",
			"rule_sets,/orgs/1/sec_policy/draft/rule_sets/3",
		}, "\n")
		entries, errs := parseHrefFile(strings.NewReader(in))
		if len(errs) != 0 {
			t.Fatalf("unexpected errors: %v", errs)
		}
		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}
		if entries[0].Href != "/orgs/1/sec_policy/draft/rule_sets/3" || entries[1].Href != "/orgs/1/sec_policy/draft/ip_lists/7" {
			t.Errorf("order not preserved: %+v", entries)
		}
	})

	t.Run("reports an error per unusable line instead of dropping it", func(t *testing.T) {
		in := strings.Join([]string{
			"rule_sets,/orgs/1/sec_policy/draft/rule_sets/3",
			"labels,/orgs/1/labels/1",
			"garbage",
		}, "\n")
		entries, errs := parseHrefFile(strings.NewReader(in))
		if len(entries) != 1 {
			t.Fatalf("got %d entries, want 1", len(entries))
		}
		if len(errs) != 2 {
			t.Fatalf("got %d errors, want 2: %v", len(errs), errs)
		}
		// Errors must identify the offending line number to be actionable.
		for _, err := range errs {
			if !strings.Contains(err.Error(), "line ") {
				t.Errorf("error %q does not name the line number", err)
			}
		}
	})
}

func TestCollectPendingHrefs(t *testing.T) {
	// Shaped like GET /orgs/{id}/sec_policy/pending.
	pending := `{
		"rule_sets": [
			{"href": "/orgs/1/sec_policy/draft/rule_sets/3"},
			{"href": "/orgs/1/sec_policy/draft/rule_sets/4"}
		],
		"ip_lists": [{"href": "/orgs/1/sec_policy/draft/ip_lists/7"}],
		"firewall_settings": {"href": "/orgs/1/sec_policy/draft/firewall_settings"},
		"virtual_servers": [{"href": "/orgs/1/sec_policy/draft/virtual_servers/1"}],
		"some_future_type": [{"href": "/orgs/1/sec_policy/draft/some_future_type/1"}],
		"update_type": "modify",
		"empty_collection": []
	}`

	c, err := gabs.ParseJSON([]byte(pending))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	got := collectPendingHrefs(c)

	want := []string{
		"/orgs/1/sec_policy/draft/rule_sets/3",
		"/orgs/1/sec_policy/draft/rule_sets/4",
		"/orgs/1/sec_policy/draft/ip_lists/7",
		// Singleton object, not an array - previously special-cased by hand.
		"/orgs/1/sec_policy/draft/firewall_settings",
		// Was absent from the hardcoded allowlist entirely.
		"/orgs/1/sec_policy/draft/virtual_servers/1",
		// A type this binary has never heard of must still be discovered.
		"/orgs/1/sec_policy/draft/some_future_type/1",
	}
	for _, h := range want {
		if !got[h] {
			t.Errorf("pending href %q not collected", h)
		}
	}
	if len(got) != len(want) {
		t.Errorf("collected %d hrefs, want %d: %v", len(got), len(want), got)
	}
}

func TestCollectPendingHrefsEmpty(t *testing.T) {
	c, err := gabs.ParseJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if got := collectPendingHrefs(c); len(got) != 0 {
		t.Errorf("got %v, want empty", got)
	}
}
