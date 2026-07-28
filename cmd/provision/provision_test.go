// Copyright 2021 Illumio, Inc. All Rights Reserved.

package main

import (
	"strings"
	"testing"
)

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
