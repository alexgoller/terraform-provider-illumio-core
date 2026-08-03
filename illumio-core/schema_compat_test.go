// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// oldestArchivedRelease is the earliest PCE the provider claims to work with.
// Its schemas are archived under api-schemas/, so compatibility is testable
// without a PCE of that version to point at — which is the situation in
// practice, since old PCEs are not usually available to test against.
const oldestArchivedRelease = "25.2.20"

// schemaProperties reads the property names of an archived schema.
func schemaProperties(t *testing.T, release, name string) map[string]bool {
	t.Helper()

	var path string
	for _, sub := range []string{"common", "v2"} {
		p := filepath.Join("..", "api-schemas", release, sub, name+".schema.json")
		if _, err := os.Stat(p); err == nil {
			path = p
			break
		}
	}
	if path == "" {
		t.Fatalf("%s.schema.json not archived for %s; run scripts/fetch-api-schema.sh %s", name, release, release)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc["type"] == "array" {
		if items, ok := doc["items"].(map[string]interface{}); ok {
			doc = items
		}
	}

	props, _ := doc["properties"].(map[string]interface{})
	out := map[string]bool{}
	for k := range props {
		out[k] = true
	}
	if len(out) == 0 {
		t.Fatalf("no properties found in %s for %s", name, release)
	}
	return out
}

// A deny rule that configures nothing version-specific must be sendable to the
// oldest PCE the provider supports. This is what broke on 25.2: the payload
// carried all_ips_except_for_in_consumers and all_ips_except_for_in_providers,
// which 26.x added, on every update.
//
// The archived schema is the oracle, so this keeps working when a new release
// is archived and cannot drift from what Illumio published.
func TestDenyRulePayloadIsCompatibleWithOldestRelease(t *testing.T) {
	allowed := schemaProperties(t, oldestArchivedRelease, "deny_rules_get")

	r := resourceIllumioDenyRule()

	// State holds values for the Optional+Computed attributes, as it would
	// after a read — the exact condition under which the bug appeared. None of
	// them are configured.
	d := schema.TestResourceDataRaw(t, r.Schema, map[string]interface{}{
		"rule_set_href":                   "/orgs/1/sec_policy/draft/rule_sets/3",
		"enabled":                         true,
		"unscoped_consumers":              false,
		"all_ips_except_for_in_consumers": false,
		"all_ips_except_for_in_providers": false,
		"network_type":                    "brn",
		"providers":                       []interface{}{map[string]interface{}{"actors": "ams"}},
		"consumers":                       []interface{}{map[string]interface{}{"actors": "ams"}},
		"ingress_services": []interface{}{
			map[string]interface{}{"proto": "6", "port": "22"},
		},
	})

	denyRule, diags := expandIllumioDenyRule(d)
	if diags.HasError() {
		t.Fatalf("expand failed: %v", *diags)
	}

	body, err := json.Marshal(denyRule)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}

	var rejected []string
	for field := range payload {
		if !allowed[field] {
			rejected = append(rejected, field)
		}
	}
	sort.Strings(rejected)

	if len(rejected) > 0 {
		t.Errorf("deny rule payload carries %v, which PCE %s does not have. "+
			"A PCE at that version rejects the request. Send a field only when the "+
			"configuration sets it — see isConfigured in config_values.go.\npayload: %s",
			rejected, oldestArchivedRelease, body)
	}
}

// The guard is only meaningful if it would have caught the original bug, so
// prove the oracle rejects the fields that broke 25.2.
func TestOldestReleaseLacksTheFieldsThatBrokeIt(t *testing.T) {
	allowed := schemaProperties(t, oldestArchivedRelease, "deny_rules_get")

	for _, field := range []string{"all_ips_except_for_in_consumers", "all_ips_except_for_in_providers"} {
		if allowed[field] {
			t.Errorf("%s is present in %s — this test's premise is wrong, recheck the archive",
				field, oldestArchivedRelease)
		}
	}
	// ...and that fields the provider always sends genuinely do exist there.
	for _, field := range []string{"enabled", "override", "providers", "consumers", "ingress_services"} {
		if !allowed[field] {
			t.Errorf("%s is missing from %s, but the provider always sends it", field, oldestArchivedRelease)
		}
	}
}
