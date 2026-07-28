// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/illumio/terraform-provider-illumio-core/client"
)

func testResourceData() *schema.ResourceData {
	r := &schema.Resource{Schema: map[string]*schema.Schema{}}
	d := r.TestResourceData()
	d.SetId("/orgs/1/sec_policy/draft/ip_lists/7")
	return d
}

// A remotely deleted object must be dropped from state so the next plan
// recreates it. Returning an error instead wedges the configuration: plan,
// apply and destroy all fail until the user runs `terraform state rm`.
func TestHandleReadErrorRemovesDeletedResourceFromState(t *testing.T) {
	d := testResourceData()

	diags := handleReadError(&client.NotFoundError{Resource: "/orgs/1/sec_policy/draft/ip_lists/7"}, d, "illumio-core_ip_list")

	if diags.HasError() {
		t.Fatalf("expected no error, got %v", diags)
	}
	if d.Id() != "" {
		t.Errorf("Id() = %q, want empty (resource should be removed from state)", d.Id())
	}
}

func TestHandleReadErrorSurvivesWrapping(t *testing.T) {
	d := testResourceData()

	wrapped := fmt.Errorf("reading ip list: %w", &client.NotFoundError{Resource: "x"})
	if diags := handleReadError(wrapped, d, "illumio-core_ip_list"); diags.HasError() {
		t.Fatalf("expected no error, got %v", diags)
	}
	if d.Id() != "" {
		t.Error("wrapped not-found did not remove the resource from state")
	}
}

// Every other failure must still surface, and must NOT silently drop the
// resource from state - that would delete real infrastructure from Terraform's
// view on a transient outage.
func TestHandleReadErrorPreservesOtherErrors(t *testing.T) {
	for _, err := range []error{
		errors.New("unauthorized: please check your credentials"),
		errors.New("forbidden: you do not have permission OR org_id is invalid"),
		fmt.Errorf("failed: status code: %d", 500),
	} {
		d := testResourceData()

		diags := handleReadError(err, d, "illumio-core_ip_list")

		if !diags.HasError() {
			t.Errorf("handleReadError(%v) returned no error, want one", err)
		}
		if d.Id() == "" {
			t.Errorf("handleReadError(%v) removed the resource from state; only a 404 may do that", err)
		}
	}
}
