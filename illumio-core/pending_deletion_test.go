// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"testing"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceDataForPendingDeletion builds the minimal ResourceData a caller of
// recordPendingDeletion needs.
func resourceDataForPendingDeletion(t *testing.T) *schema.ResourceData {
	t.Helper()

	r := &schema.Resource{
		Schema: map[string]*schema.Schema{
			"pending_deletion": pendingDeletionSchema(),
		},
	}
	d := r.TestResourceData()
	d.SetId("/orgs/1/sec_policy/draft/ip_lists/152")
	return d
}

func TestRecordPendingDeletion(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantPending bool
		wantWarning bool
	}{
		{
			name:        "marked for deletion warns",
			body:        `{"href":"/orgs/1/sec_policy/draft/ip_lists/152","update_type":"delete"}`,
			wantPending: true,
			wantWarning: true,
		},
		{
			name:        "a created object is not pending deletion",
			body:        `{"href":"/orgs/1/sec_policy/draft/ip_lists/152","update_type":"create"}`,
			wantPending: false,
		},
		{
			name:        "an updated object is not pending deletion",
			body:        `{"href":"/orgs/1/sec_policy/draft/ip_lists/152","update_type":"update"}`,
			wantPending: false,
		},
		{
			// An object with no pending change at all reports no update_type,
			// which is the common case and must not warn.
			name:        "absent update_type is not pending deletion",
			body:        `{"href":"/orgs/1/sec_policy/draft/ip_lists/152"}`,
			wantPending: false,
		},
		{
			name:        "null update_type is not pending deletion",
			body:        `{"href":"/orgs/1/sec_policy/draft/ip_lists/152","update_type":null}`,
			wantPending: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := gabs.ParseJSON([]byte(tt.body))
			if err != nil {
				t.Fatalf("could not parse test body: %v", err)
			}

			d := resourceDataForPendingDeletion(t)
			diags := recordPendingDeletion(d, data, "illumio-core_ip_list")

			if got := d.Get("pending_deletion").(bool); got != tt.wantPending {
				t.Errorf("pending_deletion = %v, want %v", got, tt.wantPending)
			}

			if tt.wantWarning {
				if len(diags) != 1 {
					t.Fatalf("expected exactly one diagnostic, got %d", len(diags))
				}
				if diags[0].Severity != diag.Warning {
					t.Errorf("severity = %v, want Warning", diags[0].Severity)
				}
				// A warning must never fail the run - the object is still
				// active and the configuration is still valid.
				if diags.HasError() {
					t.Error("pending deletion must warn, not error")
				}
				if diags[0].Detail == "" {
					t.Error("warning must explain how to recover")
				}
			} else if len(diags) != 0 {
				t.Errorf("expected no diagnostics, got %d: %v", len(diags), diags)
			}
		})
	}
}
