// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func datasourceIllumioLabel() *schema.Resource {
	return &schema.Resource{
		ReadContext:   dataSourceIllumioLabelRead,
		SchemaVersion: 1,
		Description:   "Represents Illumio Label",

		Schema: map[string]*schema.Schema{
			"href": {
				Type:             schema.TypeString,
				Optional:         true,
				Description:      "URI of this label",
				ValidateDiagFunc: isLabelHref,
				Computed:         true,
				ExactlyOneOf:     []string{"href", "key"},
			},
			"deleted": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Flag to indicate whether deleted or not",
			},
			"key": {
				Optional:     true,
				Computed:     true,
				ExactlyOneOf: []string{"href", "key"},
				RequiredWith: []string{"value"},
				Type:         schema.TypeString,
				Description:  "Key in key-value pair. Set together with value to look the label up by name instead of href",
			},
			"value": {
				Optional:     true,
				RequiredWith: []string{"key"},
				Type:         schema.TypeString,
				Computed:     true,
				Description:  "Value in key-value pair. Set together with key to look the label up by name instead of href",
			},
			"external_data_set": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The data source from which a resource originates",
			},
			"external_data_reference": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "A unique identifier within the external data source",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when this label was first created",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when this label was last updated",
			},
			"created_by": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "User who created this label",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"updated_by": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "User who last updated this label",
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
		},
	}
}

func dataSourceIllumioLabelRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	// orgID := pConfig.OrgID
	href, err := resolveDataSourceHref(d, illumioClient, lookupSpec{
		collection:   "/orgs/%d/labels",
		policyScoped: false,
		fields:       []string{"key", "value"},
		kind:         "label",
	})
	if err != nil {
		return diag.FromErr(err)
	}

	_, data, err := illumioClient.Get(href, nil)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(gabsString(data, "href"))
	for _, key := range []string{
		"href",
		"deleted",
		"key",
		"value",
		"external_data_set",
		"external_data_reference",
		"created_at",
		"updated_at",
		"created_by",
		"updated_by",
	} {
		if data.Exists(key) {
			d.Set(key, data.S(key).Data())
		} else {
			d.Set(key, nil)
		}
	}
	return diagnostics
}
