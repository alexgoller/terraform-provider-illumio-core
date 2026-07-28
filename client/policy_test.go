// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"testing"

	"github.com/Jeffail/gabs/v2"
)

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
