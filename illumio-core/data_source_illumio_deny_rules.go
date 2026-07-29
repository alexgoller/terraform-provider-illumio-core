// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func datasourceIllumioDenyRules() *schema.Resource {
	return &schema.Resource{
		ReadContext:   datasourceIllumioDenyRulesRead,
		SchemaVersion: 1,
		Description:   "Represents Illumio Deny Rules",

		Schema: map[string]*schema.Schema{
			"rule_set_href": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateDiagFunc: isRuleSetHref,
				Description:      "URI of the rule set whose deny rules will be returned",
			},
			"items": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "List of Deny Rules in the rule set. Both ordinary deny rules and override-deny rules are returned; they are distinguished by the override attribute",
				Elem: &schema.Resource{
					Schema: denyRuleDatasourceSchema(false),
				},
			},
		},
	}
}

func datasourceIllumioDenyRulesRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	ruleSetHref := d.Get("rule_set_href").(string)

	// Deny rules exist only below a rule set. There is no policy-version-wide
	// collection to query.
	endpoint := fmt.Sprintf("%s/deny_rules", ruleSetHref)

	_, data, err := illumioClient.AsyncGet(endpoint, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	denyRules := []map[string]interface{}{}
	for _, child := range data.Children() {
		denyRules = append(denyRules, denyRuleToMap(child))
	}

	d.SetId(endpoint)
	d.Set("items", denyRules)

	return diagnostics
}
