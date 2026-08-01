// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"encoding/json"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Regression: a deny rule must not send the 26.2-only fields unless the
// configuration sets them. The provider used GetOkExists, which reports an
// Optional+Computed attribute as set once a read has put a value in state, so
// every update carried all_ips_except_for_in_consumers,
// all_ips_except_for_in_providers and unscoped_consumers - and a PCE older
// than 26.2 rejects them because the fields do not exist there.
func TestDenyRuleOmitsUnconfiguredVersionSpecificFields(t *testing.T) {
	r := resourceIllumioDenyRule()

	// State holds values for the Optional+Computed attributes, as it would
	// after a read. None of them are in the configuration.
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

	b, err := json.Marshal(denyRule)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(b, &payload); err != nil {
		t.Fatal(err)
	}

	for _, field := range []string{
		"all_ips_except_for_in_consumers",
		"all_ips_except_for_in_providers",
		"unscoped_consumers",
		"network_type",
	} {
		if v, ok := payload[field]; ok {
			t.Errorf("%s = %v was sent although the configuration does not set it; a PCE older than 26.2 rejects it", field, v)
		}
	}

	// The fields the PCE always needs must still be there.
	for _, field := range []string{"enabled", "override", "providers", "consumers", "ingress_services"} {
		if _, ok := payload[field]; !ok {
			t.Errorf("required field %q missing from the payload: %s", field, b)
		}
	}
}

// isConfigured must answer "no" rather than guess when the raw configuration is
// unavailable, which is the case during import.
func TestIsConfiguredWithoutRawConfig(t *testing.T) {
	r := resourceIllumioDenyRule()
	d := r.TestResourceData()

	for _, key := range []string{"network_type", "unscoped_consumers", "nonexistent"} {
		if isConfigured(d, key) {
			t.Errorf("isConfigured(%q) = true with no raw config, want false", key)
		}
	}
}
