#!/usr/bin/env bash
#
# Build this fork into a Terraform provider mirror.
#
# The fork is not on the Terraform Registry, so `source = "illumio/illumio-core"`
# would otherwise always resolve to the upstream provider. A mirror serves the
# fork under that same address, so existing configurations work unchanged.
#
# Produces, under ./mirror:
#
#   registry.terraform.io/illumio/illumio-core/
#     index.json                                  <- network mirror: version list
#     <version>.json                              <- network mirror: archive list
#     terraform-provider-illumio-core_<version>_<os>_<arch>.zip
#
# That single directory works two ways:
#
#   filesystem_mirror  - point Terraform at ./mirror on disk
#   network_mirror     - serve ./mirror over HTTPS (S3, GitHub Pages, nginx)
#
# Usage:
#   scripts/build-mirror.sh                       # default version, host platform
#   VERSION=0.0.2 scripts/build-mirror.sh         # pick a version
#   PLATFORMS="darwin_arm64 linux_amd64" scripts/build-mirror.sh
#   PLATFORMS=all scripts/build-mirror.sh         # everything upstream ships
#
set -euo pipefail

VERSION="${VERSION:-2.0.0}"
NAMESPACE="${NAMESPACE:-illumio}"
TYPE="illumio-core"
HOSTNAME_="${HOSTNAME_:-registry.terraform.io}"
OUT="${OUT:-mirror}"

ALL_PLATFORMS="darwin_amd64 darwin_arm64 linux_amd64 linux_arm64 linux_386 linux_arm windows_amd64 windows_386 freebsd_amd64 openbsd_amd64"

if [ "${PLATFORMS:-}" = "all" ]; then
  PLATFORMS="$ALL_PLATFORMS"
elif [ -z "${PLATFORMS:-}" ]; then
  PLATFORMS="$(go env GOOS)_$(go env GOARCH)"
fi

command -v zip >/dev/null 2>&1 || { echo "error: zip is required" >&2; exit 1; }

DEST="${OUT}/${HOSTNAME_}/${NAMESPACE}/${TYPE}"
rm -rf "${DEST}"
mkdir -p "${DEST}"

echo "building terraform-provider-${TYPE} ${VERSION}"

BUILT=()
for platform in $PLATFORMS; do
  os="${platform%_*}"
  arch="${platform#*_}"

  binary="terraform-provider-${TYPE}_v${VERSION}"
  [ "$os" = "windows" ] && binary="${binary}.exe"

  tmp="$(mktemp -d)"
  # CGO off so the binary runs anywhere, matching .goreleaser.yml.
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w" -o "${tmp}/${binary}" .

  zipname="terraform-provider-${TYPE}_${VERSION}_${os}_${arch}.zip"
  ( cd "$tmp" && zip -q "${zipname}" "${binary}" )
  mv "${tmp}/${zipname}" "${DEST}/"
  rm -rf "$tmp"

  BUILT+=("$platform")
  echo "  ${platform}"
done

# --- network mirror protocol index files -------------------------------------
#
# https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol
#
# "hashes" uses the same format as .terraform.lock.hcl. A zh: hash is simply the
# SHA-256 of the .zip, so it can be produced here; h1: is a dirhash of the
# extracted contents and cannot be produced portably in shell. Network mirror
# clients verify the zh: hashes below.

python3 - "$DEST" "$VERSION" "$TYPE" "${BUILT[@]}" <<'PY'
import hashlib, json, os, sys

dest, version, ptype = sys.argv[1], sys.argv[2], sys.argv[3]
platforms = sys.argv[4:]

index_path = os.path.join(dest, "index.json")
versions = {}
if os.path.exists(index_path):
    with open(index_path) as fh:
        versions = json.load(fh).get("versions", {})
versions[version] = {}

with open(index_path, "w") as fh:
    json.dump({"versions": versions}, fh, indent=2, sort_keys=True)
    fh.write("\n")

def zh(path):
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            digest.update(chunk)
    return "zh:" + digest.hexdigest()

archives = {}
for p in platforms:
    name = f"terraform-provider-{ptype}_{version}_{p}.zip"
    archives[p] = {"url": name, "hashes": [zh(os.path.join(dest, name))]}
with open(os.path.join(dest, f"{version}.json"), "w") as fh:
    json.dump({"archives": archives}, fh, indent=2, sort_keys=True)
    fh.write("\n")
PY

cat <<EOF

mirror written to ${OUT}/

Use it locally (filesystem mirror) - put this in ~/.terraformrc:

  provider_installation {
    filesystem_mirror {
      path    = "$(cd "${OUT}" && pwd)"
      include = ["${HOSTNAME_}/${NAMESPACE}/${TYPE}"]
    }
    direct {
      exclude = ["${HOSTNAME_}/${NAMESPACE}/${TYPE}"]
    }
  }

Or serve ${OUT}/ over HTTPS and use a network mirror:

  provider_installation {
    network_mirror {
      url     = "https://example.com/terraform/"
      include = ["${HOSTNAME_}/${NAMESPACE}/${TYPE}"]
    }
    direct {
      exclude = ["${HOSTNAME_}/${NAMESPACE}/${TYPE}"]
    }
  }

Then pin the version in your configuration:

  terraform {
    required_providers {
      illumio-core = {
        source  = "${NAMESPACE}/${TYPE}"
        version = "${VERSION}"
      }
    }
  }

The include/exclude pair matters: it routes ONLY this provider to the mirror and
leaves every other provider coming from the registry as normal.
EOF
