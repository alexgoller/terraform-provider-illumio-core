#!/usr/bin/env python3
"""Write the Terraform network mirror index files for a set of provider archives.

The network mirror protocol needs two JSON documents alongside the .zip files:

    index.json        every version this mirror offers
    <version>.json    the archives for one version, with hashes

See https://developer.hashicorp.com/terraform/internals/provider-network-mirror-protocol

`hashes` uses the .terraform.lock.hcl format. A `zh:` hash is the SHA-256 of the
.zip, so it is produced here. An `h1:` hash is a dirhash over the *extracted*
contents and is deliberately not emitted: it cannot be computed portably without
unpacking, and clients accept a mirror that publishes `zh:` alone.

Usage:
    scripts/mirror-index.py dist 2.0.6
    scripts/mirror-index.py dist 2.0.6 --also-list 2.0.5 2.0.4

`--also-list` adds versions to index.json without needing their archives present,
so a release job can publish an index covering every tag rather than only the one
it just built.
"""

import argparse
import hashlib
import json
import os
import re
import sys

PROVIDER = "terraform-provider-illumio-core"


def zh(path):
    """The zh: hash of an archive — plain SHA-256 of the file."""
    digest = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(1 << 20), b""):
            digest.update(chunk)
    return "zh:" + digest.hexdigest()


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("directory", help="directory holding the provider .zip files")
    ap.add_argument("version", help="version being released, without a leading v")
    ap.add_argument("--also-list", nargs="*", default=[],
                    help="further versions to name in index.json")
    args = ap.parse_args()

    version = args.version.lstrip("v")
    pattern = re.compile(
        rf"^{re.escape(PROVIDER)}_{re.escape(version)}_([a-z0-9]+)_([a-z0-9]+)\.zip$"
    )

    archives = {}
    for name in sorted(os.listdir(args.directory)):
        m = pattern.match(name)
        if m:
            platform = f"{m.group(1)}_{m.group(2)}"
            archives[platform] = {
                "url": name,
                "hashes": [zh(os.path.join(args.directory, name))],
            }

    if not archives:
        sys.exit(
            f"no {PROVIDER}_{version}_<os>_<arch>.zip files in {args.directory!r} — "
            "nothing to index"
        )

    versions = {v.lstrip("v"): {} for v in args.also_list}
    versions[version] = {}

    index_path = os.path.join(args.directory, "index.json")
    with open(index_path, "w") as fh:
        json.dump({"versions": versions}, fh, indent=2, sort_keys=True)
        fh.write("\n")

    version_path = os.path.join(args.directory, f"{version}.json")
    with open(version_path, "w") as fh:
        json.dump({"archives": archives}, fh, indent=2, sort_keys=True)
        fh.write("\n")

    print(f"  {index_path}   {len(versions)} version(s)")
    print(f"  {version_path}   {len(archives)} platform(s): "
          f"{' '.join(sorted(archives))}")


if __name__ == "__main__":
    main()
