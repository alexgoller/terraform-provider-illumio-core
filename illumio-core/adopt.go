// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// adoptedKey records that Terraform started managing an object it did not
// create, so destroying the resource must not delete it.
const adoptedKey = "adopted"

// adoptedSchema is the attribute every adoptable resource carries.
func adoptedSchema(objectName string) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeBool,
		Computed: true,
		Description: fmt.Sprintf(
			"Whether Terraform adopted a pre-existing %s rather than creating it. "+
				"Adopted objects are left in place when the resource is destroyed, since Terraform did not create them",
			objectName),
	}
}

// adoptExisting finds the object a create collided with.
//
// The PCE rejects a duplicate with a 406 rather than returning the existing
// object, so this looks it up by the fields that make it unique. Those fields
// are matched partially by the PCE, so the result is filtered for an exact
// match and a unique one: adopting the wrong object would be worse than
// failing.
func adoptExisting(pConfig Config, endpoint string, match map[string]string) (string, error) {
	illumioClient := pConfig.IllumioClient

	params := map[string]string{"max_results": "500"}
	for key, value := range match {
		params[key] = value
	}

	_, data, err := illumioClient.Get(endpoint, &params)
	if err != nil {
		return "", fmt.Errorf("looking up the existing object to adopt: %w", err)
	}

	var matches []string
	for _, candidate := range data.Children() {
		exact := true
		for key, value := range match {
			if actual := gabsString(candidate, key); actual != value {
				exact = false
				break
			}
		}
		if !exact {
			continue
		}
		if href := gabsString(candidate, "href"); href != "" {
			matches = append(matches, href)
		}
	}

	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		return matches[0], nil
	default:
		sort.Strings(matches)
		return "", fmt.Errorf(
			"%d objects match %s, so it is not clear which to adopt. Import the intended one by HREF: %s",
			len(matches), describeMatch(match), strings.Join(matches, ", "))
	}
}

func describeMatch(match map[string]string) string {
	keys := make([]string, 0, len(match))
	for key := range match {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s=%q", key, match[key]))
	}
	return strings.Join(parts, " ")
}

// adoptOnConflict resolves a failed create into an adoption.
//
// It returns the href of the existing object and marks the resource as adopted.
// The caller is responsible for applying its configured attributes afterwards,
// so that the configuration wins over whatever the object currently holds.
func adoptOnConflict(pConfig Config, d *schema.ResourceData, endpoint string, match map[string]string, objectName string) diag.Diagnostics {
	href, err := adoptExisting(pConfig, endpoint, match)
	if err != nil {
		return diag.FromErr(err)
	}

	if href == "" {
		// The PCE said it exists but it cannot be found, so something more
		// interesting is going on than a simple duplicate.
		return diag.Errorf(
			"the PCE reports a %s matching %s already exists, but it could not be found to adopt. "+
				"It may be deleted-but-not-provisioned, or outside this organization",
			objectName, describeMatch(match))
	}

	log.Printf("[INFO] adopting existing %s %s matching %s", objectName, href, describeMatch(match))

	d.SetId(href)
	d.Set(adoptedKey, true)

	return nil
}

// deleteUnlessAdopted removes an adopted object from state without deleting it
// from the PCE.
//
// Returns true when the resource was adopted and the caller must not issue a
// delete. Terraform deleting an object it never created is destructive in a way
// removing a resource from a configuration should not be - another team may own
// it.
func deleteUnlessAdopted(d *schema.ResourceData, objectName string) (diag.Diagnostics, bool) {
	if !d.Get(adoptedKey).(bool) {
		return nil, false
	}

	diags := diag.Diagnostics{{
		Severity: diag.Warning,
		Summary:  fmt.Sprintf("Adopted %s removed from Terraform state, not deleted", objectName),
		Detail: fmt.Sprintf(
			"%s already existed when Terraform first managed it, so it was adopted rather than created. "+
				"It has been removed from state and still exists in the PCE. Delete it there if it is no longer "+
				"wanted.", d.Id()),
	}}

	d.SetId("")
	return diags, true
}
