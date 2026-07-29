// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// workloadImportKeys are the workload attributes that can be used to import a
// workload instead of its HREF, in the order a bare value is tried.
//
// A managed workload's HREF contains a PCE-generated UUID, so importing by HREF
// means looking the UUID up in the UI first, for every host. These are the
// identifiers an operator already knows.
var workloadImportKeys = []string{"hostname", "name", "ip_address", "external_data_reference"}

// importWorkloadState resolves an import ID to a workload HREF.
//
// Accepted forms:
//
//	/orgs/1/workloads/8a2b...          an HREF, passed through unchanged
//	web-01.example.com                 matched against hostname, then name
//	hostname=web-01.example.com        matched against that attribute only
//	name=WEB-01                        "
//	ip_address=10.0.0.5                "
//	external_data_reference=cmdb-1234  "
//
// The match must be unique and exact. The PCE does partial matching on these
// query parameters, so a substring hit is filtered out here rather than
// silently importing the wrong host.
func importWorkloadState(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	id := strings.TrimSpace(d.Id())

	// An HREF is already unambiguous.
	if strings.HasPrefix(id, "/orgs/") {
		return []*schema.ResourceData{d}, nil
	}

	pConfig, ok := m.(Config)
	if !ok {
		return nil, fmt.Errorf("provider is not configured")
	}

	keys := workloadImportKeys
	value := id

	// An explicit key=value form pins the attribute.
	if k, v, found := strings.Cut(id, "="); found && contains(workloadImportKeys, strings.TrimSpace(k)) {
		keys = []string{strings.TrimSpace(k)}
		value = strings.TrimSpace(v)
	}

	if value == "" {
		return nil, fmt.Errorf(
			"empty workload identifier: supply an HREF, a hostname, or one of %s",
			strings.Join(workloadImportKeys, "="),
		)
	}

	explicit := len(keys) == 1
	var attempted []string
	var firstErr error

	for _, key := range keys {
		// The PCE rejects a non-IP value for ip_address with a 406, so a bare
		// hostname must not be probed against it: the resulting "Invalid IP
		// address" error would mask the real problem. Asked for explicitly,
		// say plainly what is wrong instead.
		if key == "ip_address" && net.ParseIP(value) == nil {
			if explicit {
				return nil, fmt.Errorf("ip_address=%q is not a valid IP address", value)
			}
			continue
		}
		attempted = append(attempted, key)

		href, err := findWorkloadHref(pConfig, key, value)
		if err != nil {
			// With an explicit key there is nothing to fall back to, so the
			// error is the answer. While probing, keep trying other keys but
			// remember the failure in case nothing matches.
			if explicit {
				return nil, err
			}
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if href != "" {
			d.SetId(href)
			return []*schema.ResourceData{d}, nil
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	return nil, fmt.Errorf(
		"no workload found matching %q. Tried %s. "+
			"Import also accepts a full HREF, or an explicit key such as hostname=%s",
		value, strings.Join(attempted, ", "), value,
	)
}

// findWorkloadHref returns the HREF of the single workload whose key exactly
// equals value, or "" when there is no exact match.
func findWorkloadHref(pConfig Config, key, value string) (string, error) {
	illumioClient := pConfig.IllumioClient

	params := map[string]string{
		key: value,
		// The PCE matches these parameters partially, so several rows can come
		// back for one exact value. Fetch enough to detect ambiguity.
		"max_results": "500",
	}

	_, data, err := illumioClient.Get(fmt.Sprintf("/orgs/%d/workloads", illumioClient.OrgID), &params)
	if err != nil {
		return "", fmt.Errorf("looking up workload by %s=%q: %w", key, value, err)
	}

	var matches []string
	for _, workload := range data.Children() {
		if actual, ok := workload.S(key).Data().(string); ok && actual == value {
			if href, ok := workload.S("href").Data().(string); ok {
				matches = append(matches, href)
			}
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
			"%d workloads have %s=%q, so the import is ambiguous. Import by HREF instead, one of: %s",
			len(matches), key, value, strings.Join(matches, ", "),
		)
	}
}
