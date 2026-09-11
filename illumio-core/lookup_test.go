// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

func parseObjects(t *testing.T, body string) *gabs.Container {
	t.Helper()
	c, err := gabs.ParseJSON([]byte(body))
	if err != nil {
		t.Fatalf("bad test fixture: %v", err)
	}
	return c
}

// The PCE filters names by substring, so a query for "prod" comes back holding
// "non-prod" and "preprod" too. Exact matching is the whole point of this
// helper: without it, indexing the first result picks the wrong environment.
const labelFixture = `[
  {"href":"/orgs/1/labels/1","key":"env","value":"non-prod"},
  {"href":"/orgs/1/labels/2","key":"env","value":"prod"},
  {"href":"/orgs/1/labels/3","key":"env","value":"preprod"},
  {"href":"/orgs/1/labels/4","key":"app","value":"prod"}
]`

func TestLookupHrefExactMatchIgnoresSubstrings(t *testing.T) {
	href, err := lookupHref(parseObjects(t, labelFixture),
		map[string]string{"key": "env", "value": "prod"}, "label")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if href != "/orgs/1/labels/2" {
		t.Errorf("href = %q, want /orgs/1/labels/2 — substring matches must not win", href)
	}
}

// Both key and value must match. "app"/"prod" and "env"/"prod" are different
// labels that the PCE would return from the same value filter.
func TestLookupHrefRequiresEveryField(t *testing.T) {
	href, err := lookupHref(parseObjects(t, labelFixture),
		map[string]string{"key": "app", "value": "prod"}, "label")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if href != "/orgs/1/labels/4" {
		t.Errorf("href = %q, want /orgs/1/labels/4", href)
	}
}

func TestLookupHrefNoMatchSuggests(t *testing.T) {
	_, err := lookupHref(parseObjects(t, labelFixture),
		map[string]string{"key": "env", "value": "prd"}, "label")
	if err == nil {
		t.Fatal("expected an error for a value that does not exist")
	}
	for _, want := range []string{"no label matches", `value = "prd"`, "Did you mean"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q\ngot: %v", want, err)
		}
	}
}

// Ambiguity must fail loudly. Silently returning the first match is the bug
// this helper exists to prevent.
func TestLookupHrefAmbiguousIsAnError(t *testing.T) {
	dupes := `[
      {"href":"/orgs/1/sec_policy/draft/services/1","name":"https"},
      {"href":"/orgs/1/sec_policy/draft/services/2","name":"https"}
    ]`
	_, err := lookupHref(parseObjects(t, dupes), map[string]string{"name": "https"}, "service")
	if err == nil {
		t.Fatal("expected an error when two objects match")
	}
	for _, want := range []string{"2 services match", "services/1", "services/2", "href"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q\ngot: %v", want, err)
		}
	}
}

func TestLookupHrefEmptyCollection(t *testing.T) {
	_, err := lookupHref(parseObjects(t, `[]`), map[string]string{"name": "nothing"}, "ip list")
	if err == nil {
		t.Fatal("expected an error for an empty collection")
	}
	if strings.Contains(err.Error(), "Did you mean") {
		t.Errorf("must not suggest anything when nothing exists\ngot: %v", err)
	}
}

// A long list of near-misses must stay readable.
func TestLookupHrefCapsSuggestions(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("[")
	for i := 0; i < 12; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`{"href":"/orgs/1/sec_policy/draft/services/` + string(rune('a'+i)) + `","name":"svc-` + string(rune('a'+i)) + `"}`)
	}
	sb.WriteString("]")

	_, err := lookupHref(parseObjects(t, sb.String()), map[string]string{"name": "missing"}, "service")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "and 7 more") {
		t.Errorf("expected the suggestion list to be capped\ngot: %v", err)
	}
}

// Every data source that gained a name lookup must leave href optional, or
// existing configurations that omit it would break.
func TestLookupDataSourcesAcceptEitherHrefOrName(t *testing.T) {
	want := map[string][]string{
		"illumio-core_label":                {"key", "value"},
		"illumio-core_label_type":           {"key"},
		"illumio-core_label_group":          {"name"},
		"illumio-core_service":              {"name"},
		"illumio-core_ip_list":              {"name"},
		"illumio-core_rule_set":             {"name"},
		"illumio-core_enforcement_boundary": {"name"},
		"illumio-core_virtual_service":      {"name"},
		"illumio-core_pairing_profile":      {"name"},
		"illumio-core_workload":             {"hostname", "name"},
	}

	ds := Provider().DataSourcesMap
	for name, fields := range want {
		r, ok := ds[name]
		if !ok {
			t.Errorf("%s is not registered", name)
			continue
		}
		href, ok := r.Schema["href"]
		if !ok {
			t.Errorf("%s has no href", name)
			continue
		}
		if href.Required {
			t.Errorf("%s: href is still Required, so a name lookup is impossible", name)
		}
		if !href.Optional {
			t.Errorf("%s: href must be Optional", name)
		}
		if len(href.ExactlyOneOf) == 0 {
			t.Errorf("%s: href has no ExactlyOneOf, so omitting every identifier would not be caught", name)
		}
		for _, f := range fields {
			s, ok := r.Schema[f]
			if !ok {
				t.Errorf("%s: no %s attribute", name, f)
				continue
			}
			if !s.Optional {
				t.Errorf("%s: %s must be Optional to be usable as a lookup", name, f)
			}
		}
	}
}
