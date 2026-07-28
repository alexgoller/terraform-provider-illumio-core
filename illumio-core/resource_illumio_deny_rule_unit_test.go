// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Jeffail/gabs/v2"
)

// actorBlock builds a providers/consumers entry with every schema key present,
// mirroring what the SDK hands to the expand function.
func actorBlock(overrides map[string]interface{}) map[string]interface{} {
	block := map[string]interface{}{
		"actors":      "",
		"exclusion":   false,
		"label":       nil,
		"label_group": nil,
		"workload":    nil,
		"ip_list":     nil,
	}
	for k, v := range overrides {
		block[k] = v
	}
	return block
}

func href(h string) map[string]interface{} { return map[string]interface{}{"href": h} }

func TestExpandDenyRuleActors(t *testing.T) {
	tests := []struct {
		name      string
		block     map[string]interface{}
		wantErr   bool
		wantCount int
	}{
		{"ams", actorBlock(map[string]interface{}{"actors": "ams"}), false, 1},
		{"label", actorBlock(map[string]interface{}{"label": href("/orgs/1/labels/1")}), false, 1},
		{"ip_list", actorBlock(map[string]interface{}{"ip_list": href("/orgs/1/sec_policy/draft/ip_lists/1")}), false, 1},
		{"label_group", actorBlock(map[string]interface{}{"label_group": href("/orgs/1/sec_policy/draft/label_groups/x")}), false, 1},
		{"workload", actorBlock(map[string]interface{}{"workload": href("/orgs/1/workloads/x")}), false, 1},

		{"no actor", actorBlock(nil), true, 0},
		{"two actors", actorBlock(map[string]interface{}{
			"actors":  "ams",
			"ip_list": href("/orgs/1/sec_policy/draft/ip_lists/1"),
		}), true, 0},

		// Exclusions apply only to label and label_group.
		{"exclusion on label", actorBlock(map[string]interface{}{
			"label":     href("/orgs/1/labels/1"),
			"exclusion": true,
		}), false, 1},
		{"exclusion on label_group", actorBlock(map[string]interface{}{
			"label_group": href("/orgs/1/sec_policy/draft/label_groups/x"),
			"exclusion":   true,
		}), false, 1},
		{"exclusion on ip_list", actorBlock(map[string]interface{}{
			"ip_list":   href("/orgs/1/sec_policy/draft/ip_lists/1"),
			"exclusion": true,
		}), true, 0},
		{"exclusion on ams", actorBlock(map[string]interface{}{
			"actors":    "ams",
			"exclusion": true,
		}), true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actors, diags := expandIllumioDenyRuleActors([]interface{}{tt.block}, "providers")

			if tt.wantErr {
				if !diags.HasError() {
					t.Fatalf("expected an error, got none (actors=%d)", len(actors))
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if len(actors) != tt.wantCount {
				t.Fatalf("got %d actors, want %d", len(actors), tt.wantCount)
			}
		})
	}
}

// A deny rule actor has no virtual_service or virtual_server, so a config that
// tried to set one could never reach the PCE.
func TestExpandDenyRuleActorsSerializesNoVirtualActors(t *testing.T) {
	actors, diags := expandIllumioDenyRuleActors([]interface{}{
		actorBlock(map[string]interface{}{"label": href("/orgs/1/labels/1")}),
	}, "providers")
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}

	b, err := json.Marshal(actors)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, forbidden := range []string{"virtual_service", "virtual_server"} {
		if strings.Contains(string(b), forbidden) {
			t.Errorf("actor payload contains %q: %s", forbidden, b)
		}
	}
}

func serviceBlock(proto, port, toPort, hrefValue string) map[string]interface{} {
	return map[string]interface{}{
		"proto":   proto,
		"port":    port,
		"to_port": toPort,
		"href":    hrefValue,
	}
}

func TestExpandDenyRuleIngressServices(t *testing.T) {
	tests := []struct {
		name    string
		block   map[string]interface{}
		wantErr bool
	}{
		{"service href", serviceBlock("", "", "", "/orgs/1/sec_policy/draft/services/3"), false},
		{"tcp port", serviceBlock("6", "22", "", ""), false},
		{"udp port range", serviceBlock("17", "3389", "3395", ""), false},
		{"proto only", serviceBlock("6", "", "", ""), false},

		{"empty block", serviceBlock("", "", "", ""), true},
		{"href and proto", serviceBlock("6", "", "", "/orgs/1/sec_policy/draft/services/3"), true},
		{"href with port", serviceBlock("", "22", "", "/orgs/1/sec_policy/draft/services/3"), true},
		{"to_port without port", serviceBlock("6", "", "3395", ""), true},
		{"to_port equal to port", serviceBlock("6", "22", "22", ""), true},
		{"to_port below port", serviceBlock("6", "3395", "3389", ""), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			services, diags := expandIllumioDenyRuleIngressServices([]interface{}{tt.block})

			if tt.wantErr {
				if !diags.HasError() {
					t.Fatalf("expected an error, got none (services=%+v)", services)
				}
				return
			}
			if diags.HasError() {
				t.Fatalf("unexpected error: %v", diags)
			}
			if len(services) != 1 {
				t.Fatalf("got %d services, want 1", len(services))
			}
		})
	}
}

// The port range must survive the string/int conversion the schema requires.
func TestExpandDenyRuleIngressServicePortRange(t *testing.T) {
	services, diags := expandIllumioDenyRuleIngressServices([]interface{}{
		serviceBlock("6", "3389", "3395", ""),
	})
	if diags.HasError() {
		t.Fatalf("unexpected error: %v", diags)
	}
	s := services[0]
	if s.Proto == nil || *s.Proto != 6 {
		t.Errorf("Proto = %v, want 6", s.Proto)
	}
	if s.Port == nil || *s.Port != 3389 {
		t.Errorf("Port = %v, want 3389", s.Port)
	}
	if s.ToPort == nil || *s.ToPort != 3395 {
		t.Errorf("ToPort = %v, want 3395", s.ToPort)
	}
	if s.Href != "" {
		t.Errorf("Href = %q, want empty", s.Href)
	}
}

func TestDenyRuleSetHrefFromHref(t *testing.T) {
	tests := []struct {
		href string
		want string
	}{
		{
			"/orgs/1/sec_policy/draft/rule_sets/3/deny_rules/10",
			"/orgs/1/sec_policy/draft/rule_sets/3",
		},
		{
			"/orgs/4130175/sec_policy/active/rule_sets/17732923532872140/deny_rules/17732923532791163",
			"/orgs/4130175/sec_policy/active/rule_sets/17732923532872140",
		},
		// Not a deny rule href.
		{"/orgs/1/sec_policy/draft/rule_sets/3/sec_rules/5", ""},
		{"/orgs/1/sec_policy/draft/rule_sets/3", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := denyRuleSetHrefFromHref(tt.href); got != tt.want {
			t.Errorf("denyRuleSetHrefFromHref(%q) = %q, want %q", tt.href, got, tt.want)
		}
	}
}

// Flattening must read back only the selectors a deny rule can hold, so an
// unexpected actor type cannot break d.Set with an opaque schema error.
func TestExtractDenyRuleActors(t *testing.T) {
	input := `[
		{"label": {"href": "/orgs/1/labels/1"}, "exclusion": false},
		{"ip_list": {"href": "/orgs/1/sec_policy/draft/ip_lists/1"}},
		{"actors": "ams"},
		{"virtual_service": {"href": "/orgs/1/sec_policy/draft/virtual_services/x"}}
	]`

	c, err := gabs.ParseJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	actors := extractDenyRuleActors(c)
	if len(actors) != 4 {
		t.Fatalf("got %d actors, want 4", len(actors))
	}

	if _, ok := actors[0]["label"]; !ok {
		t.Error("label actor not extracted")
	}
	if v, ok := actors[0]["exclusion"]; !ok || v != false {
		t.Errorf("exclusion = %v (present=%v), want false", v, ok)
	}
	if _, ok := actors[1]["ip_list"]; !ok {
		t.Error("ip_list actor not extracted")
	}
	if actors[2]["actors"] != "ams" {
		t.Errorf("actors = %v, want ams", actors[2]["actors"])
	}
	// virtual_service is not a valid deny-rule actor and must be ignored.
	if _, ok := actors[3]["virtual_service"]; ok {
		t.Error("virtual_service was extracted but is not a valid deny-rule actor")
	}
	if len(actors[3]) != 0 {
		t.Errorf("unexpected keys extracted for invalid actor: %v", actors[3])
	}
}

func TestExtractDenyRuleIngressServices(t *testing.T) {
	input := `{"ingress_services": [
		{"port": 22, "proto": 6},
		{"port": 3389, "to_port": 3395, "proto": 6},
		{"href": "/orgs/1/sec_policy/draft/services/3"}
	]}`

	c, err := gabs.ParseJSON([]byte(input))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}

	services := extractDenyRuleIngressServices(c)
	if len(services) != 3 {
		t.Fatalf("got %d services, want 3", len(services))
	}

	// The schema stores ports and protocols as strings.
	if services[0]["port"] != "22" || services[0]["proto"] != "6" {
		t.Errorf("services[0] = %v, want string port/proto", services[0])
	}
	if services[1]["to_port"] != "3395" {
		t.Errorf("services[1][to_port] = %v, want \"3395\"", services[1]["to_port"])
	}
	if services[2]["href"] != "/orgs/1/sec_policy/draft/services/3" {
		t.Errorf("services[2] = %v, want href form", services[2])
	}
}

func TestExtractDenyRuleIngressServicesAbsent(t *testing.T) {
	c, err := gabs.ParseJSON([]byte(`{}`))
	if err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if got := extractDenyRuleIngressServices(c); got != nil {
		t.Errorf("got %v, want nil", got)
	}
}
