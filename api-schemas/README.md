# Archived PCE API schemas

Illumio's published API schemas, one directory per PCE release.

The provider has to know which fields a given PCE supports. Sending one that a
release does not have is rejected — that is how deny rules broke on 25.2, where
the provider sent `all_ips_except_for_in_consumers`, added in 26.x. These files
make that answerable offline, without a PCE of that version to test against.

## Layout

```
api-schemas/
  25.2.10/openapi.json          only the OpenAPI index is published for this patch level
  25.2.20/openapi.json
          v2/*.schema.json      per-endpoint request and response schemas
          common/*.schema.json  shared objects (deny_rules_get, href_object, ...)
  26.3.0/ ...
```

**The `v2/` and `common/` files are the useful part.** `openapi.json` is only an
index: every one of its schema entries is a `$ref` to a file in those
directories, so on its own it says nothing about which fields exist.

## Comparing releases

```bash
# What changed for one object?
scripts/diff-api-schema.py 25.2.20 26.3.0 deny_rules_get

  deny_rules_get
    + all_ips_except_for_in_consumers   (only in 26.3.0 — do not send to 25.2.20)
    + all_ips_except_for_in_providers   (only in 26.3.0 — do not send to 25.2.20)

# Find a schema by name
scripts/diff-api-schema.py --list 26.3.0 deny

# Everything that differs between two releases
scripts/diff-api-schema.py 25.2.20 26.3.0
```

Between 25.2.20 and 26.3.0, 28 of 387 shared schemas differ and 16 schemas are
new — so this is worth checking before adding a field to a resource.

## Adding a release

```bash
scripts/fetch-api-schema.sh 25.3.0
```

Not every release publishes a bundle, and the path layout varies, so the script
tries the known shapes. To see what exists for a release:

```
https://product-docs-repo.illumio.com/Tech-Docs/Core/<major.minor>/REST-APIs/out/en/index-en.html
```

## Provenance

| Version | Source |
|---|---|
| 25.2.10 | [`Illumio_Open_API_25_2_10.json.zip`](https://product-docs-repo.illumio.com/Tech-Docs/Core/25.2/REST-APIs/Illumio_Open_API_25_2_10.json.zip) — contains only the OpenAPI index |
| 25.2.20 | [`Illumio_Open_API_25_2_20.zip`](https://product-docs-repo.illumio.com/Tech-Docs/Core/25.2/REST-APIs/REST_API_25.2.20/Illumio_Open_API_25_2_20.zip) |
| 26.3.0 | `webservices-v2-experimental-26.3.0`, distributed alongside `illumio-py`. Not published on the docs content host at the time of archiving — `…/Tech-Docs/Core/26.3/REST-APIs/…` returns 403. |

`docs.illumio.com/core/<version>/…` returns **401** for recent releases because
they are login-gated. The content host it redirects to,
`product-docs-repo.illumio.com`, serves the same files unauthenticated, and is
what `scripts/fetch-api-schema.sh` uses.

These files are Illumio's, reproduced here unmodified for offline compatibility
checking. Only macOS artefacts (`.DS_Store`, `._*`) were stripped.

## Caveat

A published schema is not proof of runtime behaviour. Deny rules are the
example: `deny_rules_get.schema.json` is present in both 25.2.20 and 26.3.0,
but the `{rule_set_href}/deny_rules` **endpoint** appears in neither release's
path list, because it is flagged `end_user_private_perm`. It was confirmed to
exist only by calling a live PCE. Treat these files as authoritative about
*fields*, and a live PCE as authoritative about *endpoints*.
