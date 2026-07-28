// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

import (
	"errors"
	"fmt"
	"strings"
)

// secPolicySegment marks the start of a security policy object path. Only
// objects below it can be provisioned.
const secPolicySegment = "sec_policy"

// ResourceTypeFromHref derives the security policy change-subset collection
// name from a policy object href.
//
//	/orgs/1/sec_policy/draft/rule_sets/3             -> rule_sets
//	/orgs/1/sec_policy/draft/firewall_settings       -> firewall_settings
//	/orgs/1/sec_policy/draft/rule_sets/3/sec_rules/5 -> rule_sets
//
// Nested objects such as sec rules and deny rules resolve to their parent rule
// set, which is the object the PCE actually provisions.
//
// Deriving the type from the href removes any need for a hardcoded list of
// known resource types, so a policy object type this provider has never heard
// of is handled correctly rather than being silently discarded.
func ResourceTypeFromHref(href string) (string, error) {
	trimmed := strings.Trim(strings.TrimSpace(href), "/")
	if trimmed == "" {
		return "", errors.New("empty href")
	}

	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		if part != secPolicySegment {
			continue
		}
		// parts[i+1] is the policy version, parts[i+2] is the collection name.
		if i+2 >= len(parts) || parts[i+2] == "" {
			return "", fmt.Errorf("href %q ends at the policy version and names no policy object", href)
		}
		return parts[i+2], nil
	}

	return "", fmt.Errorf("href %q is not a security policy object (no %q path segment); only security policy objects can be provisioned", href, secPolicySegment)
}

// SecurityPolicyChangeSubset represents the change subset of a security policy
// provisioning request, keyed by the PCE policy object collection name
// ("rule_sets", "ip_lists", "virtual_servers", ...).
//
// This is deliberately an open map rather than a fixed struct. The set of
// provisionable object types is defined by the PCE, not by this provider, and
// the previous fixed-field implementation dispatched through a switch with no
// default case: any type it did not know about was silently discarded, and
// provisioning still reported success. virtual_servers, for one, had no field
// at all. Keying by name means a new or unrecognised policy object type is
// carried through to the PCE and rejected loudly there, rather than dropped
// here in silence.
//
// The JSON shape is unchanged: {"rule_sets": [{"href": "..."}], ...}
type SecurityPolicyChangeSubset map[string][]Href

// NewSecurityPolicyChangeSubset returns an initialised, empty change subset.
// Use this rather than a nil map so an empty subset marshals as {} and not null.
func NewSecurityPolicyChangeSubset() SecurityPolicyChangeSubset {
	return make(SecurityPolicyChangeSubset)
}

// AppendHref records href under the given policy object collection name.
//
// It returns an error rather than ignoring empty input, so a caller that fails
// to determine a resource type cannot quietly contribute nothing to the
// request.
func (cs SecurityPolicyChangeSubset) AppendHref(rtype, href string) error {
	if rtype == "" {
		return errors.New("cannot append href with an empty resource type")
	}
	if href == "" {
		return errors.New("cannot append an empty href")
	}
	cs[rtype] = append(cs[rtype], Href{Href: href})
	return nil
}

// Size returns the total number of hrefs across all collections.
func (cs SecurityPolicyChangeSubset) Size() int {
	var n int
	for _, hrefs := range cs {
		n += len(hrefs)
	}
	return n
}

// SecurityPolicy represents security policy resource
type SecurityPolicy struct {
	UpdateDesc   string                     `json:"update_description"`
	ChangeSubset SecurityPolicyChangeSubset `json:"change_subset"`
}

// ToMap - Returns map for SecurityPolicy model
func (sp *SecurityPolicy) ToMap() (map[string]interface{}, error) {
	return toMap(sp)
}
