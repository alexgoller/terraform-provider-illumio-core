// Copyright 2021 Illumio, Inc. All Rights Reserved.

package client

import (
	"fmt"

	"github.com/Jeffail/gabs/v2"
)

// PendingPolicyHrefs returns the set of policy object hrefs that currently have
// pending (draft) changes awaiting provisioning.
//
// GET /orgs/{org}/sec_policy/pending returns an empty JSON object when nothing
// is pending, so an empty result is an ordinary success, not an error.
func (c *V2) PendingPolicyHrefs() (map[string]bool, error) {
	_, data, err := c.Get(fmt.Sprintf("/orgs/%d/sec_policy/pending", c.OrgID), nil)
	if err != nil {
		return nil, fmt.Errorf("error fetching pending security policy: %w", err)
	}

	return collectPendingHrefs(data), nil
}

// collectPendingHrefs extracts every href from a sec_policy/pending response.
//
// It walks every key in the response rather than a fixed allowlist, so policy
// object types added by future PCE versions are discovered automatically.
// Collections arrive as arrays of objects; singletons such as
// firewall_settings arrive as a bare object.
func collectPendingHrefs(data *gabs.Container) map[string]bool {
	pending := map[string]bool{}
	if data == nil {
		return pending
	}

	for _, value := range data.ChildrenMap() {
		switch value.Data().(type) {
		case []interface{}:
			for _, child := range value.Children() {
				if href, ok := child.S("href").Data().(string); ok && href != "" {
					pending[href] = true
				}
			}
		case map[string]interface{}:
			if href, ok := value.S("href").Data().(string); ok && href != "" {
				pending[href] = true
			}
		}
	}

	return pending
}
