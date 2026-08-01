#!/usr/bin/env bash
#
# Fetch an Illumio PCE API schema bundle into api-schemas/<version>/.
#
# Illumio publishes the OpenAPI spec and the per-object JSON Schema files per
# release. The schema bundle is the one that matters: the OpenAPI JSON is only
# an index of $refs and says nothing about which fields exist.
#
# docs.illumio.com/core/<version>/... returns 401 for recent releases, because
# they are login-gated. The content host it redirects to serves the same files
# without authentication, and that is what this script uses.
#
# Usage:
#   scripts/fetch-api-schema.sh 25.2.20
#   scripts/fetch-api-schema.sh 25.3.0
#
# Not every release publishes a bundle. Browse what exists for a release at:
#   https://product-docs-repo.illumio.com/Tech-Docs/Core/<major.minor>/REST-APIs/out/en/index-en.html
#
set -euo pipefail

VERSION="${1:-}"
if [ -z "$VERSION" ]; then
  echo "usage: $(basename "$0") <version>   e.g. 25.2.20" >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="${ROOT}/api-schemas/${VERSION}"
HOST="https://product-docs-repo.illumio.com/Tech-Docs/Core"

# 25.2.20 -> 25.2
MINOR="$(echo "$VERSION" | cut -d. -f1,2)"
UNDER="$(echo "$VERSION" | tr '.' '_')"

command -v unzip >/dev/null 2>&1 || { echo "error: unzip is required" >&2; exit 1; }

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

# Paths differ between releases: some sit under a REST_API_<version>/
# subdirectory, some at the release root. Try both rather than guess.
found=""
for base in "${HOST}/${MINOR}/REST-APIs/REST_API_${VERSION}" "${HOST}/${MINOR}/REST-APIs"; do
  for name in "Illumio_Open_API_${UNDER}.zip" "Illumio_Open_API_${UNDER}.json.zip"; do
    if curl -fsSL --max-time 180 -o "${tmp}/bundle.zip" "${base}/${name}" 2>/dev/null; then
      echo "  bundle:  ${base}/${name}"
      found="$base"
      break 2
    fi
  done
done

if [ -z "$found" ]; then
  echo "error: no schema bundle published for ${VERSION}" >&2
  echo "       check ${HOST}/${MINOR}/REST-APIs/out/en/index-en.html" >&2
  exit 1
fi

mkdir -p "$DEST"
unzip -q -o "${tmp}/bundle.zip" -x "__MACOSX/*" -d "$tmp"

# The bundle unpacks to a single top-level directory whose name varies by
# release (webservices-api-experimental-<version>, webservices-v2-..., ...).
inner="$(find "$tmp" -maxdepth 1 -type d -name 'webservices*' | head -1)"
if [ -n "$inner" ]; then
  for sub in v2 common; do
    [ -d "${inner}/${sub}" ] && { rm -rf "${DEST}/${sub}"; cp -R "${inner}/${sub}" "${DEST}/${sub}"; }
  done
else
  # Some patch levels ship only the OpenAPI index, with no schema files.
  echo "  note:    this bundle contains no v2/ or common/ schema files"
fi

for spec in "${base}/Illumio_${VERSION}.json" "${HOST}/${MINOR}/REST-APIs/Illumio_${VERSION}.json"; do
  if curl -fsSL --max-time 180 -o "${DEST}/openapi.json" "$spec" 2>/dev/null; then
    echo "  openapi: ${spec}"
    break
  fi
done

find "$DEST" \( -name '.DS_Store' -o -name '._*' \) -delete 2>/dev/null || true

echo "  wrote:   api-schemas/${VERSION}/ ($(find "$DEST" -name '*.schema.json' | wc -l | tr -d ' ') schema files)"
