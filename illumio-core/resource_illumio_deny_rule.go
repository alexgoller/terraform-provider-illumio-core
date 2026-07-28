// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/illumio/terraform-provider-illumio-core/models"
)

var (
	// The deny_rule ingress service schema restricts proto to TCP and UDP.
	// ICMP has no inline representation and must be referenced as a service
	// HREF.
	validDenyRuleIngSerProtos = []string{"6", "17"}

	// Network types a deny rule can apply to, per rule_network_type.
	validDenyRuleNetworkTypes = []string{"brn", "non_brn", "all"}

	// The authoritative deny_rule_actor schema permits only these selectors.
	// Notably it has no virtual_service or virtual_server, unlike allow rules.
	validDenyRuleActorKeys = []string{"label", "label_group", "workload", "ip_list"}

	// The only value the "actors" selector accepts on a deny rule.
	validDenyRuleActors = []string{"ams"}
)

func resourceIllumioDenyRule() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceIllumioDenyRuleCreate,
		ReadContext:   resourceIllumioDenyRuleRead,
		UpdateContext: resourceIllumioDenyRuleUpdate,
		DeleteContext: resourceIllumioDenyRuleDelete,
		SchemaVersion: 1,
		Description: "Manages Illumio Deny Rule. " +
			"A deny rule blocks the consumers (the sources that initiate the connection) " +
			"from reaching the providers (the destinations) on the given services. " +
			"Set override to true for an override-deny rule; the policy evaluation order is " +
			"override-deny > allow > deny.",
		Schema: denyRuleResourceSchemaMap(),
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
	}
}

func denyRuleResourceSchemaMap() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"rule_set_href": {
			Type:             schema.TypeString,
			Required:         true,
			ForceNew:         true,
			ValidateDiagFunc: isRuleSetHref,
			Description:      "URI of the rule set in which the deny rule will be created",
		},
		"href": {
			Type:        schema.TypeString,
			Computed:    true,
			Description: "URI of the Deny Rule",
		},
		"enabled": {
			Type:        schema.TypeBool,
			Optional:    true,
			Default:     true,
			Description: "Determines whether the deny rule is enabled in the rule set. Default value: true",
		},
		"override": {
			Type:     schema.TypeBool,
			Optional: true,
			Default:  false,
			Description: "When true, this is an override-deny rule that takes precedence over allow rules. " +
				"When false, it is an ordinary deny rule that allow rules can supersede. " +
				"An override-deny rule still blocks traffic. Default value: false",
		},
		"description": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Description of the Deny Rule",
		},
		"network_type": {
			Type:             schema.TypeString,
			Optional:         true,
			Computed:         true,
			ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(validDenyRuleNetworkTypes, false)),
			Description:      "Network types this rule applies to. Allowed values are \"brn\", \"non_brn\" and \"all\". The PCE defaults to \"brn\"",
		},
		"unscoped_consumers": {
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
			Description: "Set the scope for rule consumers to All",
		},
		"all_ips_except_for_in_consumers": {
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
			Description: "Treat the consumer side as everything except what the consumers resolve to",
		},
		"all_ips_except_for_in_providers": {
			Type:        schema.TypeBool,
			Optional:    true,
			Computed:    true,
			Description: "Treat the provider side as everything except what the providers resolve to",
		},
		"providers": {
			Type:        schema.TypeSet,
			Required:    true,
			MinItems:    1,
			Description: "Providers of the Deny Rule - the destinations that provide the service. Only one actor can be specified in one providers block",
			Elem:        denyRuleActorSchema("provider"),
		},
		"consumers": {
			Type:        schema.TypeSet,
			Required:    true,
			MinItems:    1,
			Description: "Consumers of the Deny Rule - the sources that initiate the connection. Only one actor can be specified in one consumers block",
			Elem:        denyRuleActorSchema("consumer"),
		},
		"ingress_services": {
			Type:     schema.TypeSet,
			Required: true,
			MinItems: 1,
			Description: "Collection of ingress services. Only one of the {\"href\"} or {\"proto\", \"port\", \"to_port\"} " +
				"parameter combinations is allowed. ICMP has no inline representation - reference an ICMP service by href instead",
			Elem: &schema.Resource{
				Schema: map[string]*schema.Schema{
					"proto": {
						Type:             schema.TypeString,
						Optional:         true,
						ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(validDenyRuleIngSerProtos, true)),
						Description:      "Protocol number. Allowed values are 6 (TCP) and 17 (UDP)",
					},
					"port": {
						Type:             schema.TypeString,
						Optional:         true,
						ValidateDiagFunc: isStringAPortNumber(),
						Description:      "Port number, or starting port when specifying a range. Allowed range is 0-65535",
					},
					"to_port": {
						Type:             schema.TypeString,
						Optional:         true,
						ValidateDiagFunc: isStringAPortNumber(),
						Description:      "Upper end of port range. Allowed range is 0-65535",
					},
					"href": {
						Type:             schema.TypeString,
						Optional:         true,
						ValidateDiagFunc: isServiceHref,
						Description:      "URI of Service",
					},
				},
			},
		},
		"egress_services": {
			Type:        schema.TypeSet,
			Optional:    true,
			Description: "Collection of egress services. Egress services are service references only and have no inline port/protocol form",
			Elem:        hrefSchemaRequired("Service", isServiceHref),
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

// denyRuleActorSchema returns the schema shared by the providers and consumers
// blocks. Both sides accept the same selectors on a deny rule.
func denyRuleActorSchema(side string) *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"actors": {
				Type:             schema.TypeString,
				Optional:         true,
				ValidateDiagFunc: validation.ToDiagFunc(validation.StringInSlice(validDenyRuleActors, false)),
				Description:      fmt.Sprintf("%s workloads filter. The only allowed value is \"ams\" (all workloads)", strings.Title(side)),
			},
			"exclusion": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the actor is an exclusion. Only valid for label and label_group actors",
			},
			"label": {
				Type:        schema.TypeSet,
				Optional:    true,
				MaxItems:    1,
				Description: fmt.Sprintf("Label %s filter", side),
				Elem:        labelOptionalKeyValue(true),
			},
			"label_group": {
				Type:        schema.TypeSet,
				Optional:    true,
				MaxItems:    1,
				Description: fmt.Sprintf("Label Group %s filter", side),
				Elem:        hrefSchemaRequired("Label Group", isLabelGroupHref),
			},
			"workload": {
				Type:        schema.TypeSet,
				Optional:    true,
				MaxItems:    1,
				Description: fmt.Sprintf("Workload %s filter", side),
				Elem:        hrefSchemaRequired("Workload", isWorkloadHref),
			},
			"ip_list": {
				Type:        schema.TypeSet,
				Optional:    true,
				MaxItems:    1,
				Description: fmt.Sprintf("IP List %s filter", side),
				Elem:        hrefSchemaRequired("IP List", isIPListHref),
			},
		},
	}
}

func resourceIllumioDenyRuleCreate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	ruleSetHref := d.Get("rule_set_href").(string)

	denyRule, diags := expandIllumioDenyRule(d)
	if diags.HasError() {
		return *diags
	}

	_, data, err := illumioClient.Create(fmt.Sprintf("%s/deny_rules", ruleSetHref), denyRule)
	if err != nil {
		return diag.FromErr(err)
	}

	// Deny rules are provisioned as part of their containing rule set.
	pConfig.StoreHref("rule_sets", ruleSetHref)
	d.SetId(data.S("href").Data().(string))

	return resourceIllumioDenyRuleRead(ctx, d, m)
}

func expandIllumioDenyRule(d *schema.ResourceData) (*models.DenyRule, *diag.Diagnostics) {
	var diags diag.Diagnostics

	denyRule := &models.DenyRule{
		Enabled:     d.Get("enabled").(bool),
		Override:    d.Get("override").(bool),
		NetworkType: d.Get("network_type").(string),
	}

	if desc, ok := d.GetOk("description"); ok {
		denyRule.Description = PtrTo(desc.(string))
	} else if d.HasChange("description") {
		// Allow clearing a description that was previously set.
		denyRule.Description = PtrTo("")
	}

	for key, target := range map[string]**bool{
		"unscoped_consumers":              &denyRule.UnscopedConsumers,
		"all_ips_except_for_in_consumers": &denyRule.AllIPsExceptForInConsumers,
		"all_ips_except_for_in_providers": &denyRule.AllIPsExceptForInProviders,
	} {
		// GetOkExists distinguishes an explicit false from an unset value, so
		// the PCE keeps ownership of defaults the provider does not set.
		if v, ok := d.GetOkExists(key); ok { //nolint:staticcheck // no SDKv2 replacement
			*target = PtrTo(v.(bool))
		}
	}

	providers, errs := expandIllumioDenyRuleActors(d.Get("providers").(*schema.Set).List(), "providers")
	diags = append(diags, errs...)
	denyRule.Providers = providers

	consumers, errs := expandIllumioDenyRuleActors(d.Get("consumers").(*schema.Set).List(), "consumers")
	diags = append(diags, errs...)
	denyRule.Consumers = consumers

	ingressServices, errs := expandIllumioDenyRuleIngressServices(d.Get("ingress_services").(*schema.Set).List())
	diags = append(diags, errs...)
	denyRule.IngressServices = ingressServices

	denyRule.EgressServices = models.GetHrefs(d.Get("egress_services").(*schema.Set).List())

	return denyRule, &diags
}

// expandIllumioDenyRuleActors converts a providers or consumers block.
//
// It is deliberately separate from the security rule's equivalent: deny rules
// permit a narrower actor set, and reusing the allow-rule expansion would let
// virtual_service and virtual_server through to the PCE.
func expandIllumioDenyRuleActors(actors []interface{}, blockName string) ([]*models.DenyRuleActor, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := []*models.DenyRuleActor{}

	for _, raw := range actors {
		a := raw.(map[string]interface{})

		actor := &models.DenyRuleActor{
			Actors:     a["actors"].(string),
			Label:      expandLabelOptionalKeyValue(a["label"], false),
			LabelGroup: getHrefObj(a["label_group"]),
			Workload:   getHrefObj(a["workload"]),
			IPList:     getHrefObj(a["ip_list"]),
		}

		if !models.HasOneActor(actor) {
			diags = append(diags, diag.Errorf(
				"[illumio-core_deny_rule] Each %s block must specify exactly one actor: one of actors, label, label_group, workload or ip_list",
				blockName,
			)...)
			continue
		}

		if exclusion, ok := a["exclusion"].(bool); ok && exclusion {
			if actor.Label == nil && actor.LabelGroup == nil {
				diags = append(diags, diag.Errorf(
					"[illumio-core_deny_rule] %s exclusions can only be applied to label or label_group actors",
					blockName,
				)...)
				continue
			}
			actor.Exclusion = PtrTo(true)
		}

		result = append(result, actor)
	}

	return result, diags
}

func expandIllumioDenyRuleIngressServices(services []interface{}) ([]models.IngressService, diag.Diagnostics) {
	var diags diag.Diagnostics
	result := []models.IngressService{}

	for _, raw := range services {
		s := raw.(map[string]interface{})

		href := s["href"].(string)
		proto := s["proto"].(string)
		port := s["port"].(string)
		toPort := s["to_port"].(string)

		switch {
		case href == "" && proto == "":
			diags = append(diags, diag.Errorf("[illumio-core_deny_rule] At least one of [href, proto] is required inside ingress_services")...)
			continue
		case href != "" && proto != "":
			diags = append(diags, diag.Errorf("[illumio-core_deny_rule] Exactly one of [href, proto] is allowed inside ingress_services")...)
			continue
		case href != "" && (port != "" || toPort != ""):
			diags = append(diags, diag.Errorf("[illumio-core_deny_rule] port/to_port is not allowed with href inside ingress_services")...)
			continue
		case proto != "" && port == "" && toPort != "":
			diags = append(diags, diag.Errorf("[illumio-core_deny_rule] port is required with to_port inside ingress_services")...)
			continue
		}

		service := models.IngressService{}
		if href != "" {
			service.Href = href
			result = append(result, service)
			continue
		}

		if v, ok := getInt(proto); ok {
			service.Proto = &v
		}
		if v, ok := getInt(port); ok {
			service.Port = &v

			if toPortValue, ok := getInt(toPort); ok {
				if toPortValue <= v {
					diags = append(diags, diag.Errorf("[illumio-core_deny_rule] Value of to_port must be greater than the value of port inside ingress_services")...)
					continue
				}
				service.ToPort = &toPortValue
			}
		}

		result = append(result, service)
	}

	return result, diags
}

func resourceIllumioDenyRuleRead(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	_, data, err := illumioClient.Get(d.Id(), nil)
	if err != nil {
		return handleReadError(err, d, "illumio-core_deny_rule")
	}

	// A deny rule href is {rule_set_href}/deny_rules/{id}, so the parent is
	// always recoverable from the id. This is what makes plain href
	// passthrough import work.
	if ruleSetHref := denyRuleSetHrefFromHref(d.Id()); ruleSetHref != "" {
		d.Set("rule_set_href", ruleSetHref)
	}

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
			d.Set(key, data.S(key).Data())
		} else {
			d.Set(key, nil)
		}
	}

	d.Set("providers", extractDenyRuleActors(data.S("providers")))
	d.Set("consumers", extractDenyRuleActors(data.S("consumers")))
	d.Set("ingress_services", extractDenyRuleIngressServices(data))

	if data.Exists("egress_services") {
		d.Set("egress_services", extractDataSourceAttrs(data, "egress_services", []string{"href"}))
	} else {
		d.Set("egress_services", nil)
	}

	return diagnostics
}

// denyRuleSetHrefFromHref recovers the containing rule set href from a deny
// rule href, i.e. everything before the trailing "/deny_rules/{id}".
func denyRuleSetHrefFromHref(href string) string {
	idx := strings.LastIndex(href, "/deny_rules/")
	if idx < 0 {
		return ""
	}
	return href[:idx]
}

// extractDenyRuleActors flattens a providers or consumers array.
//
// Only the selectors valid for a deny rule are read back, so an unexpected
// actor type cannot fail the d.Set with an opaque schema error.
func extractDenyRuleActors(data *gabs.Container) []map[string]interface{} {
	actors := []map[string]interface{}{}

	for _, entry := range data.Children() {
		actor := map[string]interface{}{}

		for key, value := range entry.ChildrenMap() {
			switch {
			case key == "actors":
				if v, ok := value.Data().(string); ok {
					actor[key] = v
				}
			case key == "exclusion":
				if v, ok := value.Data().(bool); ok {
					actor[key] = v
				}
			case contains(validDenyRuleActorKeys, key):
				v, ok := value.Data().(map[string]interface{})
				if !ok {
					continue
				}
				href, _ := v["href"].(string)
				actor[key] = []map[string]string{{"href": href}}
			}
		}

		actors = append(actors, actor)
	}

	return actors
}

// extractDenyRuleIngressServices flattens ingress_services, converting the
// PCE's numeric port and protocol values into the strings the schema uses.
func extractDenyRuleIngressServices(data *gabs.Container) []map[string]interface{} {
	if !data.Exists("ingress_services") {
		return nil
	}

	services := []map[string]interface{}{}

	for _, entry := range data.S("ingress_services").Children() {
		service := map[string]interface{}{}

		for key, value := range entry.ChildrenMap() {
			if key == "href" {
				if v, ok := value.Data().(string); ok {
					service[key] = v
				}
				continue
			}
			// port, to_port and proto arrive as JSON numbers.
			if v, ok := value.Data().(float64); ok {
				service[key] = strconv.Itoa(int(v))
			}
		}

		services = append(services, service)
	}

	return services
}

func resourceIllumioDenyRuleUpdate(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	denyRule, diags := expandIllumioDenyRule(d)
	if diags.HasError() {
		return *diags
	}

	// Toggling override updates the same object; it never moves the rule to a
	// different endpoint.
	_, err := illumioClient.Update(d.Id(), denyRule)
	if err != nil {
		return diag.FromErr(err)
	}

	pConfig.StoreHref("rule_sets", d.Get("rule_set_href").(string))

	return resourceIllumioDenyRuleRead(ctx, d, m)
}

func resourceIllumioDenyRuleDelete(ctx context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	var diagnostics diag.Diagnostics
	pConfig, _ := m.(Config)
	illumioClient := pConfig.IllumioClient

	ruleSetHref := d.Get("rule_set_href").(string)

	_, err := illumioClient.Delete(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	pConfig.StoreHref("rule_sets", ruleSetHref)
	d.SetId("")

	return diagnostics
}
