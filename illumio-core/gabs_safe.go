// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import "github.com/Jeffail/gabs/v2"

// gabsString reads a string from a JSON path without panicking.
//
// c.S("name").Data().(string) panics with
//
//	interface conversion: interface {} is nil, not string
//
// whenever the PCE omits the field - a different PCE version, an optional
// attribute, or a permission that hides it. That crashes the provider
// mid-apply, which risks leaving Terraform state diverged from reality.
// A missing value is not exceptional here, so it reads as the empty string.
func gabsString(c *gabs.Container, path ...string) string {
	if c == nil {
		return ""
	}

	value := c
	if len(path) > 0 {
		value = c.S(path...)
	}
	if value == nil {
		return ""
	}

	s, _ := value.Data().(string)
	return s
}
