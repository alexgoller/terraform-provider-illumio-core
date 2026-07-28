// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"encoding/json"
	"reflect"
	"testing"
)

func pendingSet(hrefs ...string) map[string]bool {
	m := map[string]bool{}
	for _, h := range hrefs {
		m[h] = true
	}
	return m
}

const (
	ruleSet3 = "/orgs/1/sec_policy/draft/rule_sets/3"
	ipList7  = "/orgs/1/sec_policy/draft/ip_lists/7"
	svc9     = "/orgs/1/sec_policy/draft/services/9"
)

func TestProvisioningTargets(t *testing.T) {
	tests := []struct {
		name       string
		hrefs      []string
		allPending bool
		pending    map[string]bool
		want       []string
	}{
		{
			name:    "only pending hrefs are provisioned",
			hrefs:   []string{ruleSet3, ipList7},
			pending: pendingSet(ruleSet3),
			want:    []string{ruleSet3},
		},
		{
			name:    "nothing pending yields no targets",
			hrefs:   []string{ruleSet3, ipList7},
			pending: pendingSet(),
			want:    []string{},
		},
		{
			// An object pending but not managed by this resource must be left
			// alone - that is the whole point of the explicit hrefs mode.
			name:    "unmanaged pending objects are ignored",
			hrefs:   []string{ruleSet3},
			pending: pendingSet(ruleSet3, svc9),
			want:    []string{ruleSet3},
		},
		{
			name:       "provision_all_pending takes everything",
			hrefs:      nil,
			allPending: true,
			pending:    pendingSet(svc9, ruleSet3, ipList7),
			want:       []string{ipList7, ruleSet3, svc9},
		},
		{
			name:       "provision_all_pending with nothing pending",
			allPending: true,
			pending:    pendingSet(),
			want:       []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := provisioningTargets(tt.hrefs, tt.allPending, tt.pending)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("provisioningTargets() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Map iteration order is random, so without sorting the planned diff would
// churn between runs and show spurious changes.
func TestProvisioningTargetsAreSorted(t *testing.T) {
	pending := pendingSet(svc9, ruleSet3, ipList7)

	first := provisioningTargets(nil, true, pending)
	for i := 0; i < 50; i++ {
		if got := provisioningTargets(nil, true, pending); !reflect.DeepEqual(got, first) {
			t.Fatalf("unstable ordering: %v then %v", first, got)
		}
	}
}

func TestChangeSubsetFor(t *testing.T) {
	cs, err := changeSubsetFor([]string{
		ruleSet3,
		ipList7,
		// Nested objects provision via their parent rule set.
		"/orgs/1/sec_policy/draft/rule_sets/4/deny_rules/10",
		// virtual_servers had no field on the old fixed change-subset struct.
		"/orgs/1/sec_policy/draft/virtual_servers/2",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := len(cs["rule_sets"]); got != 2 {
		t.Errorf("rule_sets = %d, want 2 (nested deny rule should resolve to its parent)", got)
	}
	if got := len(cs["ip_lists"]); got != 1 {
		t.Errorf("ip_lists = %d, want 1", got)
	}
	if got := len(cs["virtual_servers"]); got != 1 {
		t.Errorf("virtual_servers = %d, want 1", got)
	}
	if got := cs.Size(); got != 4 {
		t.Errorf("Size() = %d, want 4", got)
	}
}

func TestChangeSubsetForRejectsNonPolicyHref(t *testing.T) {
	if _, err := changeSubsetFor([]string{"/orgs/1/labels/1"}); err == nil {
		t.Error("expected an error for a non-policy href, got nil")
	}
}

// The PCE expects {"change_subset": {"rule_sets": [{"href": "..."}]}}.
func TestChangeSubsetWireFormat(t *testing.T) {
	cs, err := changeSubsetFor([]string{ruleSet3})
	if err != nil {
		t.Fatal(err)
	}

	b, err := json.Marshal(cs)
	if err != nil {
		t.Fatal(err)
	}

	want := `{"rule_sets":[{"href":"/orgs/1/sec_policy/draft/rule_sets/3"}]}`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}
