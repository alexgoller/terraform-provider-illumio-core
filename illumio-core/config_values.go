// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import "github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

// isConfigured reports whether an attribute was set in the *configuration*, as
// opposed to holding a value in state.
//
// d.GetOkExists cannot answer this. For an Optional+Computed attribute, Read
// writes the remote value into state, and GetOkExists then reports the
// attribute as set on the next update even though the user never configured
// it. For a deny rule that meant sending all_ips_except_for_in_consumers,
// all_ips_except_for_in_providers and unscoped_consumers on every update, which
// a PCE older than 26.2 rejects because those fields do not exist there.
//
// GetRawConfig is the configuration as written, so a value absent from the
// .tf file is null regardless of what state holds.
//
// It returns false when the raw config is unavailable — during import, for
// instance — so callers fall back to sending nothing, which is the safe
// direction.
func isConfigured(d *schema.ResourceData, key string) bool {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.IsKnown() || !raw.Type().IsObjectType() {
		return false
	}
	if !raw.Type().HasAttribute(key) {
		return false
	}

	value := raw.GetAttr(key)
	return !value.IsNull() && value.IsKnown()
}
