// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

import (
	"encoding/json"
	"testing"
)

// The previous implementation was a fixed 7-field struct whose AppendHref
// switch had no default case, so any resource type it did not know about was
// silently discarded. These tests pin the replacement's behaviour.
func TestChangeSubsetAppendHref(t *testing.T) {
	t.Run("retains a type the old fixed struct did not know", func(t *testing.T) {
		cs := NewSecurityPolicyChangeSubset()
		// virtual_servers is a provisionable sec_policy object that the old
		// struct had no field for, so it was dropped on the floor.
		if err := cs.AppendHref("virtual_servers", "/orgs/1/sec_policy/draft/virtual_servers/3"); err != nil {
			t.Fatalf("AppendHref returned unexpected error: %v", err)
		}
		if got := cs.Size(); got != 1 {
			t.Fatalf("Size() = %d, want 1 (href was silently dropped)", got)
		}
	})

	t.Run("groups multiple hrefs under their type", func(t *testing.T) {
		cs := NewSecurityPolicyChangeSubset()
		for _, h := range []string{
			"/orgs/1/sec_policy/draft/rule_sets/1",
			"/orgs/1/sec_policy/draft/rule_sets/2",
		} {
			if err := cs.AppendHref("rule_sets", h); err != nil {
				t.Fatalf("AppendHref(%q): %v", h, err)
			}
		}
		if err := cs.AppendHref("ip_lists", "/orgs/1/sec_policy/draft/ip_lists/9"); err != nil {
			t.Fatal(err)
		}
		if got := cs.Size(); got != 3 {
			t.Fatalf("Size() = %d, want 3", got)
		}
		if got := len(cs["rule_sets"]); got != 2 {
			t.Fatalf("len(rule_sets) = %d, want 2", got)
		}
	})

	t.Run("rejects empty resource type and href", func(t *testing.T) {
		cs := NewSecurityPolicyChangeSubset()
		if err := cs.AppendHref("", "/orgs/1/sec_policy/draft/rule_sets/1"); err == nil {
			t.Error("AppendHref with empty resource type: want error, got nil")
		}
		if err := cs.AppendHref("rule_sets", ""); err == nil {
			t.Error("AppendHref with empty href: want error, got nil")
		}
		if got := cs.Size(); got != 0 {
			t.Fatalf("Size() = %d, want 0 after rejected appends", got)
		}
	})
}

// The wire format must not change: the PCE expects
// {"change_subset": {"rule_sets": [{"href": "..."}]}}.
func TestSecurityPolicyJSONShape(t *testing.T) {
	cs := NewSecurityPolicyChangeSubset()
	if err := cs.AppendHref("rule_sets", "/orgs/1/sec_policy/draft/rule_sets/3"); err != nil {
		t.Fatal(err)
	}
	sp := &SecurityPolicy{UpdateDesc: "Provisioned by terraform", ChangeSubset: cs}

	b, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	want := `{"update_description":"Provisioned by terraform","change_subset":{"rule_sets":[{"href":"/orgs/1/sec_policy/draft/rule_sets/3"}]}}`
	if string(b) != want {
		t.Errorf("wire format changed:\n got: %s\nwant: %s", b, want)
	}
}

// An empty change subset must still marshal as an object, never as null.
func TestSecurityPolicyEmptyChangeSubsetIsObject(t *testing.T) {
	sp := &SecurityPolicy{UpdateDesc: "x", ChangeSubset: NewSecurityPolicyChangeSubset()}
	b, err := json.Marshal(sp)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"update_description":"x","change_subset":{}}`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}
