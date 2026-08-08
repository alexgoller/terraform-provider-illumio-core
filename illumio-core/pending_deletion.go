// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"fmt"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// updateTypeDelete is the value the PCE reports in `update_type` once a
// provisioned policy object has been deleted but the deletion has not yet been
// provisioned.
const updateTypeDelete = "delete"

// pendingDeletionSchema is the computed attribute every provisionable policy
// resource carries so that a deletion made outside Terraform is visible in
// state, not only in a warning that scrolls past.
func pendingDeletionSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeBool,
		Computed: true,
		Description: "True when the object has been deleted in the PCE but the deletion has not been provisioned yet. " +
			"The object is still active and Terraform still manages it; provisioning the pending deletion removes it for real",
	}
}

// recordPendingDeletion records whether the PCE has this object marked for
// deletion, and warns when it has.
//
// Deleting a *provisioned* policy object does not remove it. The PCE marks the
// draft copy `update_type: delete` and leaves the active copy in place until
// someone provisions the deletion. Both a draft and an active GET still return
// 200, so nothing 404s, `handleReadError` never fires, and `terraform plan`
// reports "No changes" while the object is on its way out.
//
// Terraform cannot repair this on its own:
//
//   - there is no per-object way to cancel a pending deletion. A PUT of the
//     original body returns 204 and leaves `update_type` at `delete`;
//     `DELETE /sec_policy/pending` reverts every pending change org-wide, and
//     `POST /sec_policy/{version}/restore` rolls back a whole policy version.
//   - removing the resource from state so the next apply recreates it does not
//     work either: the replacement cannot be created while the original is
//     pending deletion, because names must be unique. The create fails with
//     `ip_list_name_not_unique` or its equivalent, which would turn a silent
//     drift into a broken apply.
//
// So this warns and records, and leaves the decision to a human. See the
// "Objects deleted outside Terraform" section of the policy provisioning guide
// for the recovery procedure.
func recordPendingDeletion(d *schema.ResourceData, data *gabs.Container, resourceName string) diag.Diagnostics {
	pending := gabsString(data, "update_type") == updateTypeDelete

	if err := d.Set("pending_deletion", pending); err != nil {
		return diag.FromErr(err)
	}

	if !pending {
		return nil
	}

	return diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  fmt.Sprintf("[%s] %s is marked for deletion in the PCE", resourceName, d.Id()),
		Detail: "This object was deleted outside Terraform. It is still active, so plan reports no " +
			"changes, but the next provisioning will remove it and Terraform's state will then point " +
			"at an object that no longer exists.\n\n" +
			"Terraform cannot undo this: the PCE offers no per-object way to cancel a pending " +
			"deletion, and the object cannot be recreated under the same name until the deletion is " +
			"provisioned.\n\n" +
			"To recover: provision the pending deletion, then run `terraform apply` to recreate the " +
			"object. It will be created with a new HREF, and anything referencing it is updated in " +
			"the same apply.",
	}}
}
