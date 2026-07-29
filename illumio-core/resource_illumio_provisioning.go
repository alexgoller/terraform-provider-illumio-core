// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/illumio/terraform-provider-illumio-core/client"
	"github.com/illumio/terraform-provider-illumio-core/models"
)

const defaultUpdateDescription = "Provisioned by Terraform"

func resourceIllumioProvisioning() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIllumioProvisioningCreate,
		ReadContext:   resourceIllumioProvisioningRead,
		UpdateContext: resourceIllumioProvisioningUpdate,
		DeleteContext: resourceIllumioProvisioningDelete,
		SchemaVersion: 1,
		Description: "Provisions draft security policy objects into the active policy version. " +
			"Reference policy object HREFs in `hrefs` so Terraform orders provisioning after the objects are created. " +
			"\n\n" +
			"IMPORTANT: to provision changes in the SAME apply that makes them, add a `lifecycle.replace_triggered_by` " +
			"block referencing the policy resources. A policy object's HREF does not change when its contents change, " +
			"so without it a contents-only change is not provisioned until the following apply. " +
			"\n\n" +
			"Destroying this resource does NOT revert policy - see the notes below.",

		// The core of this resource. A policy object's HREF does not change when
		// its contents change, so a static hrefs list would never produce a
		// diff and the resource would silently stop provisioning after the
		// first apply. Querying the pending set at plan time is what makes
		// re-provisioning happen, and makes it visible in the plan.
		CustomizeDiff: resourceIllumioProvisioningCustomizeDiff,

		Schema: map[string]*schema.Schema{
			"hrefs": {
				Type:         schema.TypeSet,
				Optional:     true,
				ExactlyOneOf: []string{"hrefs", "provision_all_pending"},
				Elem: &schema.Schema{
					Type:             schema.TypeString,
					ValidateDiagFunc: isProvisionableHref,
				},
				Description: "Set of security policy object HREFs to provision. Reference resource `href` attributes here so Terraform provisions them after they are created or updated",
			},
			"provision_all_pending": {
				Type:         schema.TypeBool,
				Optional:     true,
				ExactlyOneOf: []string{"hrefs", "provision_all_pending"},
				Description: "Provision every object with pending changes, including objects not managed by Terraform. " +
					"Defaults to false. WARNING: unsafe on a shared PCE - this activates any pending change, including a " +
					"partially complete policy left in draft by another user or by the PCE UI. Prefer an explicit `hrefs` set",
			},
			"update_description": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     defaultUpdateDescription,
				Description: "Commit message recorded against the provisioned policy version",
			},
			"pending_hrefs": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "HREFs this resource is responsible for that currently have pending changes. Empty once everything is provisioned",
			},
			"version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Version number of the security policy created by the last provisioning",
			},
			"version_href": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URI of the security policy version created by the last provisioning",
			},
			"provisioned_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp of the last successful provisioning",
			},
			"object_counts": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Count of objects by type in the provisioned policy version",
			},
			"workloads_affected": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of workloads affected by the last provisioning",
			},
		},
	}
}

// resourceIllumioProvisioningCustomizeDiff forces an update whenever an object
// this resource is responsible for has a pending change.
//
// Verified by spike: SetNew on a computed attribute produces a planned update
// even though no configured attribute changed, and the planned diff lists the
// exact HREFs that will be provisioned.
func resourceIllumioProvisioningCustomizeDiff(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
	// On create there is already a diff, and there is no id to reconcile.
	if d.Id() == "" {
		return nil
	}

	pConfig, ok := m.(Config)
	if !ok {
		return nil
	}

	pending, err := pConfig.IllumioClient.PendingPolicyHrefs()
	if err != nil {
		return fmt.Errorf("could not determine pending security policy while planning: %w", err)
	}

	targets := provisioningTargets(
		getStringList(d.Get("hrefs").(*schema.Set).List()),
		d.Get("provision_all_pending").(bool),
		pending,
	)

	if err := d.SetNew("pending_hrefs", targets); err != nil {
		return err
	}

	if len(targets) == 0 {
		return nil
	}

	// Something needs provisioning, so the version attributes are unknown until
	// after apply.
	for _, key := range []string{"version", "version_href", "provisioned_at", "object_counts", "workloads_affected"} {
		if err := d.SetNewComputed(key); err != nil {
			return err
		}
	}

	return nil
}

// provisioningTargets returns the sorted set of HREFs to provision.
//
// Sorting keeps the planned diff stable between runs; without it Terraform
// would show a spurious change every time map iteration order shifted.
func provisioningTargets(hrefs []string, allPending bool, pending map[string]bool) []string {
	targets := []string{}

	if allPending {
		for href := range pending {
			targets = append(targets, href)
		}
		sort.Strings(targets)
		return targets
	}

	for _, href := range hrefs {
		if pending[href] {
			targets = append(targets, href)
		}
	}
	sort.Strings(targets)

	return targets
}

// changeSubsetFor builds the provisioning request body for the given HREFs.
func changeSubsetFor(hrefs []string) (models.SecurityPolicyChangeSubset, error) {
	cs := models.NewSecurityPolicyChangeSubset()

	for _, href := range hrefs {
		rtype, err := models.ResourceTypeFromHref(href)
		if err != nil {
			return nil, err
		}
		if err := cs.AppendHref(rtype, href); err != nil {
			return nil, err
		}
	}

	return cs, nil
}

func resourceIllumioProvisioningCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	// Provisioning has no natural remote identity - a policy version is a
	// point-in-time event, not an object this resource owns - so the id is
	// synthetic and stable for the resource's lifetime.
	d.SetId(resource.UniqueId())

	return provisionAndRecord(ctx, d, m)
}

func resourceIllumioProvisioningUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	return provisionAndRecord(ctx, d, m)
}

// provisionAndRecord provisions whatever is currently pending and in scope,
// then records the resulting policy version.
func provisionAndRecord(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	// Re-read pending at apply time rather than trusting the planned value:
	// policy may have been provisioned by someone else between plan and apply.
	pending, err := illumioClient.PendingPolicyHrefs()
	if err != nil {
		return diag.FromErr(err)
	}

	targets := provisioningTargets(
		getStringList(d.Get("hrefs").(*schema.Set).List()),
		d.Get("provision_all_pending").(bool),
		pending,
	)

	if len(targets) == 0 {
		// Nothing pending is a legitimate no-op: the objects are already
		// active, or were created and deleted within the same apply.
		d.Set("pending_hrefs", []string{})
		return resourceIllumioProvisioningRead(ctx, d, m)
	}

	changeSubset, err := changeSubsetFor(targets)
	if err != nil {
		return diag.FromErr(err)
	}

	secPolicy := &models.SecurityPolicy{
		UpdateDesc:   d.Get("update_description").(string),
		ChangeSubset: changeSubset,
	}

	_, data, err := illumioClient.Create(fmt.Sprintf("/orgs/%d/sec_policy", illumioClient.OrgID), secPolicy)
	if err != nil {
		return diag.FromErr(err)
	}

	if err := setProvisionedVersion(d, illumioClient, data); err != nil {
		return diag.FromErr(err)
	}

	d.Set("provisioned_at", time.Now().UTC().Format(time.RFC3339))
	d.Set("pending_hrefs", []string{})

	return diagnostics
}

// setProvisionedVersion records the created policy version.
//
// The POST response is expected to carry the version object. Not every PCE
// version is confirmed to do so, so when the version is absent this falls back
// to reading the most recent entry from the sec_policy collection rather than
// silently leaving the attributes unset.
func setProvisionedVersion(d *schema.ResourceData, illumioClient *client.V2, data *gabs.Container) error {
	if data != nil && data.Exists("href") {
		applyVersionAttrs(d, data)
		return nil
	}

	_, versions, err := illumioClient.Get(
		fmt.Sprintf("/orgs/%d/sec_policy", illumioClient.OrgID),
		&map[string]string{"max_results": "1"},
	)
	if err != nil {
		return fmt.Errorf("provisioning succeeded but the resulting policy version could not be read: %w", err)
	}

	for _, version := range versions.Children() {
		applyVersionAttrs(d, version)
		return nil
	}

	return nil
}

func applyVersionAttrs(d *schema.ResourceData, data *gabs.Container) {
	if href := gabsString(data, "href"); href != "" {
		d.Set("version_href", href)
	}
	if version, ok := data.S("version").Data().(float64); ok {
		d.Set("version", int(version))
	}
	if affected, ok := data.S("workloads_affected").Data().(float64); ok {
		d.Set("workloads_affected", int(affected))
	}
	if counts, ok := data.S("object_counts").Data().(map[string]interface{}); ok {
		flattened := map[string]string{}
		for key, value := range counts {
			flattened[key] = fmt.Sprintf("%v", value)
		}
		d.Set("object_counts", flattened)
	}
}

func resourceIllumioProvisioningRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics

	pConfig, _ := m.(Config)

	pending, err := pConfig.IllumioClient.PendingPolicyHrefs()
	if err != nil {
		return diag.FromErr(err)
	}

	// The recorded version is a historical event and is never re-read: policy
	// versions are immutable and their absence is not drift.
	d.Set("pending_hrefs", provisioningTargets(
		getStringList(d.Get("hrefs").(*schema.Set).List()),
		d.Get("provision_all_pending").(bool),
		pending,
	))

	return diagnostics
}

// resourceIllumioProvisioningDelete removes the resource from state without
// calling the PCE.
//
// Provisioning cannot be undone. DELETE /sec_policy/pending reverts every
// pending draft change org-wide, and /sec_policy/{id}/restore rolls the whole
// policy back to an earlier version. Neither is an acceptable automatic
// consequence of destroying a bookkeeping resource, so this deliberately makes
// no request at all.
func resourceIllumioProvisioningDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	d.SetId("")
	return diag.Diagnostics{}
}
