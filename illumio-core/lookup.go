// Copyright 2021 Illumio, Inc. All Rights Reserved.

package illumiocore

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"

	"github.com/illumio/terraform-provider-illumio-core/client"
)

// maxSuggestions caps how many near-misses an error lists. Enough to spot a
// typo, few enough to stay readable when a collection is large.
const maxSuggestions = 5

// notFoundError means nothing matched. It is a distinct type so the caller can
// widen its query and retry purely to collect suggestions - the PCE filters
// server-side, so a typo returns an empty collection with nothing to suggest.
type notFoundError struct {
	kind        string
	search      string
	suggestions []string
}

func (e *notFoundError) Error() string {
	return fmt.Sprintf("no %s matches %s%s", e.kind, e.search, suggestionText(e.suggestions))
}

// lookupHref finds exactly one object in a PCE collection by exact field match.
//
// The PCE's name filters are substring matches: asking for "prod" also returns
// "non-prod", "preprod" and "production". The server-side query narrows the
// request, and the exact comparison here is what makes the answer unambiguous.
//
// Matching nothing, or matching more than one object, is an error. A data
// source that quietly picked the first result would wire the wrong object into
// policy, which is exactly the failure the plural data sources invite when
// callers index items[0].
func lookupHref(objects *gabs.Container, exact map[string]string, kind string) (string, error) {
	var (
		matched     []string
		suggestions []string
	)

	for _, obj := range objects.Children() {
		if matchesExactly(obj, exact) {
			matched = append(matched, gabsString(obj, "href"))
			continue
		}
		if s := describeCandidate(obj, exact); s != "" {
			suggestions = append(suggestions, s)
		}
	}

	switch len(matched) {
	case 1:
		return matched[0], nil
	case 0:
		return "", &notFoundError{
			kind:        kind,
			search:      describeSearch(exact),
			suggestions: suggestions,
		}
	default:
		sort.Strings(matched)
		return "", fmt.Errorf(
			"%d %ss match %s: %s. Narrow the lookup, or use href to name one exactly",
			len(matched), kind, describeSearch(exact), strings.Join(matched, ", "))
	}
}

// matchesExactly reports whether every requested field is present on the object
// and equal to the requested value.
func matchesExactly(obj *gabs.Container, exact map[string]string) bool {
	for field, want := range exact {
		if gabsString(obj, field) != want {
			return false
		}
	}
	return true
}

// describeCandidate renders an object as a suggestion, so a typo produces a
// list of what does exist rather than a bare "not found".
func describeCandidate(obj *gabs.Container, exact map[string]string) string {
	parts := make([]string, 0, len(exact))
	for _, field := range sortedKeys(exact) {
		if v := gabsString(obj, field); v != "" {
			parts = append(parts, fmt.Sprintf("%s = %q", field, v))
		}
	}
	return strings.Join(parts, ", ")
}

func describeSearch(exact map[string]string) string {
	parts := make([]string, 0, len(exact))
	for _, field := range sortedKeys(exact) {
		parts = append(parts, fmt.Sprintf("%s = %q", field, exact[field]))
	}
	return strings.Join(parts, ", ")
}

func suggestionText(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}
	sort.Strings(suggestions)
	shown := suggestions
	trailer := ""
	if len(shown) > maxSuggestions {
		shown = shown[:maxSuggestions]
		trailer = fmt.Sprintf(", and %d more", len(suggestions)-maxSuggestions)
	}
	return fmt.Sprintf(". Did you mean one of: %s%s?", strings.Join(shown, "; "), trailer)
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lookupSpec describes how one data source finds its object by name.
type lookupSpec struct {
	// collection is the PCE path holding the objects. %d is the org id.
	collection string
	// policyScoped marks collections that live under sec_policy/{pversion}.
	// Lookups use the draft version, which is where the provider manages policy.
	policyScoped bool
	// fields are the arguments that together identify one object exactly.
	fields []string
	// kind names the object in error messages.
	kind string
}

// resolveDataSourceHref returns the HREF a data source should read.
//
// When href is set it is used unchanged, so existing configurations behave
// exactly as before. Otherwise the lookup arguments are resolved to exactly one
// object — see lookupHref for why anything else is an error.
func resolveDataSourceHref(d *schema.ResourceData, c *client.V2, spec lookupSpec) (string, error) {
	if href := d.Get("href").(string); href != "" {
		return href, nil
	}

	exact := map[string]string{}
	for _, f := range spec.fields {
		if v, ok := d.GetOk(f); ok {
			if s, isStr := v.(string); isStr && s != "" {
				exact[f] = s
			}
		}
	}
	if len(exact) == 0 {
		return "", fmt.Errorf("set href, or %s, to identify the %s",
			strings.Join(spec.fields, " and "), spec.kind)
	}

	endpoint := fmt.Sprintf(spec.collection, c.OrgID)
	if spec.policyScoped {
		endpoint = fmt.Sprintf(spec.collection, c.OrgID, "draft")
	}

	// The same values narrow the request server-side. The PCE matches names by
	// substring, so this only reduces what has to be compared here.
	query := map[string]string{}
	for k, v := range exact {
		query[k] = v
	}

	_, data, err := c.AsyncGet(endpoint, &query)
	if err != nil {
		return "", err
	}

	href, lookupErr := lookupHref(data, exact, spec.kind)
	if lookupErr == nil {
		return href, nil
	}

	// Nothing matched. The server-side filter means the collection came back
	// empty, so there was nothing to suggest. Ask again with the identifying
	// field dropped but any broader one kept - for a label that keeps key and
	// drops value, so the suggestions are other environments rather than every
	// label in the org. Costs a second request only on the failure path.
	var notFound *notFoundError
	if errors.As(lookupErr, &notFound) && len(spec.fields) > 0 {
		wider := map[string]string{}
		for _, f := range spec.fields[:len(spec.fields)-1] {
			if v, ok := exact[f]; ok {
				wider[f] = v
			}
		}
		if _, all, aErr := c.AsyncGet(endpoint, &wider); aErr == nil {
			if _, wideErr := lookupHref(all, exact, spec.kind); wideErr != nil {
				return "", wideErr
			}
		}
	}

	return "", lookupErr
}
