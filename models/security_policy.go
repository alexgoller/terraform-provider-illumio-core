// Copyright 2021 Illumio, Inc. All Rights Reserved.

package models

import "errors"

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
