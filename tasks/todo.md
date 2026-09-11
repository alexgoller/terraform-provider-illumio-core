# Look up data sources by name, not just HREF

## Problem

18 of 20 singular data sources accept only `href`. To reference anything by its
real-world name you must use the plural data source, filter, index into `items`,
remember `match_type = "exact"`, and wrap the result in `one()`:

```hcl
data "illumio-core_labels" "env" {
  key = "env", value = "prod", match_type = "exact"
}
locals { env_href = one(data.illumio-core_labels.env.items[*].href) }
```

Without `match_type = "exact"` a filter for `prod` also matches `non-prod` and
`preprod`, and indexing `items[0]` picks one silently. Both are documented
anti-patterns in the policy architecture guide — they exist only because the
singular data sources cannot do this.

## Design

A shared resolver, `lookupHref`, that queries a collection and requires exactly
one **exact** match. Server-side filters narrow the request; the exact
comparison is what makes the answer unambiguous.

- 0 matches -> error naming the search, listing near-misses as suggestions
- >1 matches -> error listing the HREFs found
- 1 match   -> use it

Each data source gains optional lookup arguments, with `href` becoming
`Optional` and `ExactlyOneOf` pairing the two. Existing configurations that set
`href` are unaffected.

## Scope

Policy-authoring core plus workloads — the objects people actually reference:

- [x] `label` — `key` + `value`
- [x] `label_type` — `key`
- [x] `label_group` — `name`
- [x] `service` — `name`
- [x] `ip_list` — `name`
- [x] `rule_set` — `name`
- [x] `enforcement_boundary` — `name`
- [x] `virtual_service` — `name`
- [x] `pairing_profile` — `name`
- [x] `workload` — `hostname` or `name`

Out of scope: `security_rule` and `deny_rule` (no natural name — they are
children of a rule set), and the settings singletons.

## Tasks

- [x] Audit which data sources are HREF-only and what their natural key is
- [x] `lookupHref` helper with exact matching and useful errors
- [x] Unit tests: exact match, no match, ambiguous match, near-miss suggestions
- [x] Wire the 10 data sources, `href` -> Optional + `ExactlyOneOf`
- [x] Regenerate/extend docs for each
- [x] Verify against the live 25.2 PCE
- [x] Update the policy architecture guide to use the simpler form
- [x] Release

## Review

Done and released as 2.3.0.

**What shipped.** Ten singular data sources take their natural identifier as an
alternative to `href`, resolved by a shared `lookupHref` that requires exactly
one **exact** match.

**Why exactness is the whole feature.** Verified on a live 26.30.4 PCE by
creating `tf-lookup-prod`, `tf-lookup-prod-eu` and `tf-lookup-prod-us`. The PCE's
own filter for `value=tf-lookup-prod` returned all three, with the exact match
**last**:

```
tf-lookup-prod-eu   /orgs/5636114/labels/...255
tf-lookup-prod-us   /orgs/5636114/labels/...256
tf-lookup-prod      /orgs/5636114/labels/...252   <- the one asked for
```

An `items[0]` would have selected the wrong environment. The data source
returned `...252`.

**One thing the design got wrong first.** Suggestions were built from the
server-filtered result, so a typo returned an empty collection and produced a
bare "not found" — exactly the unhelpful error the feature was meant to avoid.
Fixed by re-querying on the failure path only, dropping the identifying field
but keeping any broader one, so a bad `env` value suggests other environments
rather than every label in the org.

**Not done.** `security_rule` and `deny_rule` have no natural name (they are
children of a rule set), and the settings singletons have nothing to look up.
