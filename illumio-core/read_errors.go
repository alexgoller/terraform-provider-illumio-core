// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"errors"
	"log"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/illumio/terraform-provider-illumio-core/client"
)

// handleReadError converts a failure from a resource's read into the behaviour
// Terraform expects.
//
// When the object no longer exists in the PCE the resource must be removed from
// state so the next plan recreates it. Returning an error instead leaves the
// configuration wedged: plan, apply and destroy all fail until the user runs
// `terraform state rm` by hand.
//
// Any other error is returned unchanged.
//
// This is for managed resources only. A data source that resolves to nothing
// genuinely is an error and must keep reporting one.
func handleReadError(err error, d *schema.ResourceData, resourceName string) diag.Diagnostics {
	if errors.Is(err, client.ErrNotFound) {
		log.Printf("[WARN] %s %q was not found in the PCE, removing it from state", resourceName, d.Id())
		d.SetId("")
		return nil
	}

	return diag.FromErr(err)
}
