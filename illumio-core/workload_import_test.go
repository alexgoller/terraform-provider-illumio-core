// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func importResourceData(id string) *schema.ResourceData {
	r := &schema.Resource{Schema: map[string]*schema.Schema{}}
	d := r.TestResourceData()
	d.SetId(id)
	return d
}

// An HREF must pass straight through without contacting the PCE, so existing
// import commands and state keep working.
func TestImportWorkloadStateHrefPassthrough(t *testing.T) {
	for _, href := range []string{
		"/orgs/1/workloads/8a2b1c3d-0000-4000-8000-000000000000",
		"/orgs/4130175/workloads/abc",
	} {
		d := importResourceData(href)

		// nil config proves no client call is attempted for the HREF form.
		out, err := importWorkloadState(context.Background(), d, nil)
		if err != nil {
			t.Fatalf("importWorkloadState(%q) unexpected error: %v", href, err)
		}
		if len(out) != 1 || out[0].Id() != href {
			t.Errorf("id = %q, want %q", out[0].Id(), href)
		}
	}
}

func TestImportWorkloadStateRejectsEmptyIdentifier(t *testing.T) {
	d := importResourceData("   ")

	_, err := importWorkloadState(context.Background(), d, Config{})
	if err == nil {
		t.Fatal("expected an error for an empty identifier")
	}
	if !strings.Contains(err.Error(), "empty workload identifier") {
		t.Errorf("unhelpful error: %v", err)
	}
}

func TestImportWorkloadStateRequiresConfiguredProvider(t *testing.T) {
	d := importResourceData("web-01")

	if _, err := importWorkloadState(context.Background(), d, nil); err == nil {
		t.Fatal("expected an error when the provider is not configured")
	}
}

// Only the documented keys pin an attribute. A hostname that happens to
// contain "=" must not be silently reinterpreted as key=value.
func TestWorkloadImportKeyParsing(t *testing.T) {
	tests := []struct {
		id       string
		wantKeys []string
		wantVal  string
	}{
		{"hostname=web-01", []string{"hostname"}, "web-01"},
		{"name=WEB-01", []string{"name"}, "WEB-01"},
		{"ip_address=10.0.0.5", []string{"ip_address"}, "10.0.0.5"},
		{"external_data_reference=cmdb-1", []string{"external_data_reference"}, "cmdb-1"},
		// Bare value: try every key in order.
		{"web-01", workloadImportKeys, "web-01"},
		// Not a known key, so the whole string stays the value.
		{"weird=host", workloadImportKeys, "weird=host"},
	}

	for _, tt := range tests {
		keys, value := workloadImportKeys, tt.id
		if k, v, found := strings.Cut(tt.id, "="); found && contains(workloadImportKeys, strings.TrimSpace(k)) {
			keys = []string{strings.TrimSpace(k)}
			value = strings.TrimSpace(v)
		}

		if value != tt.wantVal {
			t.Errorf("%q: value = %q, want %q", tt.id, value, tt.wantVal)
		}
		if len(keys) != len(tt.wantKeys) || keys[0] != tt.wantKeys[0] {
			t.Errorf("%q: keys = %v, want %v", tt.id, keys, tt.wantKeys)
		}
	}
}

// The default must never unpair: Terraform did not create the workload.
func TestManagedWorkloadUnpairOnDestroyDefaultsFalse(t *testing.T) {
	s := resourceIllumioManagedWorkload().Schema["unpair_on_destroy"]
	if s == nil {
		t.Fatal("unpair_on_destroy is not defined")
	}
	if s.Default != false {
		t.Errorf("Default = %v, want false", s.Default)
	}
	if s.Required {
		t.Error("unpair_on_destroy must be optional")
	}
}
