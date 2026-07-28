// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func datasourceIllumioDenyRule() *schema.Resource {
	return &schema.Resource{
		ReadContext:   datasourceIllumioDenyRuleRead,
		SchemaVersion: 1,
		Description:   "Represents Illumio Deny Rule",

		Schema: denyRuleDatasourceSchema(true),
	}
}

// denyRuleDatasourceSchema returns the read-only deny rule schema.
//
// hrefRequired distinguishes the singular data source, which is looked up by
// href, from the elements of the plural data source, where the href is just
// another computed attribute.
func denyRuleDatasourceSchema(hrefRequired bool) map[string]*schema.Schema {
	hrefBlock := &schema.Schema{
		Type:        schema.TypeString,
		Description: "URI of the Deny Rule",
	}
	if hrefRequired {
		hrefBlock.Required = true
		hrefBlock.ValidateDiagFunc = isDenyRuleHref
	} else {
		hrefBlock.Computed = true
	}

	return map[string]*schema.Schema{
		"href": hrefBlock,
		"rule_set_href": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "URI of the rule set containing this Deny Rule",
		},
		"enabled": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "Determines whether the deny rule is enabled in the rule set",
		},
		"override": {
			Type:     schema.TypeBool,
			Computed: true,
			Description: "Whether this is an override-deny rule that takes precedence over allow rules. " +
				"The policy evaluation order is override-deny > allow > deny",
		},
		"description": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Description of the Deny Rule",
		},
		"network_type": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Network types this rule applies to",
		},
		"unscoped_consumers": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "Whether the scope for rule consumers is set to All",
		},
		"all_ips_except_for_in_consumers": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "Whether the consumer side is everything except what the consumers resolve to",
		},
		"all_ips_except_for_in_providers": {
			Type:        schema.TypeBool,
			Computed:    true,
			Description: "Whether the provider side is everything except what the providers resolve to",
		},
		"providers": {
			Type:        schema.TypeSet,
			Computed:    true,
			Description: "Providers of the Deny Rule - the destinations that provide the service",
			Elem:        denyRuleActorDatasourceSchema("provider"),
		},
		"consumers": {
			Type:        schema.TypeSet,
			Computed:    true,
			Description: "Consumers of the Deny Rule - the sources that initiate the connection",
			Elem:        denyRuleActorDatasourceSchema("consumer"),
		},
		"ingress_services": {
			Type:        schema.TypeSet,
			Computed:    true,
			Description: "Collection of ingress services",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"proto":   {Type: schema.TypeString, Computed: true, Description: "Protocol number"},
					"port":    {Type: schema.TypeString, Computed: true, Description: "Port number, or starting port of a range"},
					"to_port": {Type: schema.TypeString, Computed: true, Description: "Upper end of port range"},
					"href":    {Type: schema.TypeString, Computed: true, Description: "URI of Service"},
				},
			},
		},
		"egress_services": {
			Type:        schema.TypeSet,
			Computed:    true,
			Description: "Collection of egress services",
			Elem:        hrefSchemaComputed("Service", isServiceHref),
		},
		"created_at": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Timestamp when this Deny Rule was first created",
		},
		"updated_at": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Timestamp when this Deny Rule was last updated",
		},
		"deleted_at": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Timestamp when this Deny Rule was deleted",
		},
		"created_by": {
			Type:        schema.TypeMap,
			Computed:    true,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "User who created this Deny Rule",
		},
		"updated_by": {
			Type:        schema.TypeMap,
			Computed:    true,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "User who last updated this Deny Rule",
		},
		"deleted_by": {
			Type:        schema.TypeMap,
			Computed:    true,
			Elem:        &schema.Schema{Type: schema.TypeString},
			Description: "User who deleted this Deny Rule",
		},
		"update_type": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "Type of the update",
		},
	}
}

func denyRuleActorDatasourceSchema(side string) *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"actors":      {Type: schema.TypeString, Computed: true, Description: side + " workloads filter"},
			"exclusion":   {Type: schema.TypeBool, Computed: true, Description: "Whether the actor is an exclusion"},
			"label":       {Type: schema.TypeSet, Computed: true, Description: "Label " + side + " filter", Elem: labelOptionalKeyValue(false)},
			"label_group": {Type: schema.TypeSet, Computed: true, Description: "Label Group " + side + " filter", Elem: hrefSchemaComputed("Label Group", isLabelGroupHref)},
			"workload":    {Type: schema.TypeSet, Computed: true, Description: "Workload " + side + " filter", Elem: hrefSchemaComputed("Workload", isWorkloadHref)},
			"ip_list":     {Type: schema.TypeSet, Computed: true, Description: "IP List " + side + " filter", Elem: hrefSchemaComputed("IP List", isIPListHref)},
		},
	}
}

func datasourceIllumioDenyRuleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	href := d.Get("href").(string)

	_, data, err := illumioClient.Get(href, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(data.S("href").Data().(string))
	for key, value := range denyRuleToMap(data) {
		d.Set(key, value)
	}

	return diagnostics
}

// denyRuleToMap flattens a deny rule into the attribute map shared by the
// singular and plural data sources.
func denyRuleToMap(data *gabs.Container) map[string]interface{} {
	denyRule := map[string]interface{}{}

	for _, key := range []string{
		"href",
		"enabled",
		"override",
		"description",
		"network_type",
		"unscoped_consumers",
		"all_ips_except_for_in_consumers",
		"all_ips_except_for_in_providers",
		"created_at",
		"updated_at",
		"deleted_at",
		"created_by",
		"updated_by",
		"deleted_by",
		"update_type",
	} {
		if data.Exists(key) {
			denyRule[key] = data.S(key).Data()
		} else {
			denyRule[key] = nil
		}
	}

	if href, ok := data.S("href").Data().(string); ok {
		denyRule["rule_set_href"] = denyRuleSetHrefFromHref(href)
	}

	denyRule["providers"] = extractDenyRuleActors(data.S("providers"))
	denyRule["consumers"] = extractDenyRuleActors(data.S("consumers"))
	denyRule["ingress_services"] = extractDenyRuleIngressServices(data)

	if data.Exists("egress_services") {
		denyRule["egress_services"] = extractDataSourceAttrs(data, "egress_services", []string{"href"})
	} else {
		denyRule["egress_services"] = nil
	}

	return denyRule
}
