#!/usr/bin/env python3
"""Compare an object's fields between two archived PCE API schema versions.

The provider has to know which fields a given PCE supports. Sending one that
does not exist yet is rejected, which is how deny rules broke on 25.2: the
provider sent all_ips_except_for_in_consumers, added in 26.x.

Usage:
    scripts/diff-api-schema.py 25.2.20 26.3.0 deny_rules_get
    scripts/diff-api-schema.py 25.2.20 26.3.0            # every shared schema
    scripts/diff-api-schema.py --list 26.3.0 deny        # find a schema by name
"""

import json
import os
import sys

ROOT = os.path.join(os.path.dirname(os.path.abspath(__file__)), os.pardir, "api-schemas")


def schema_paths(version):
    """Map schema name -> file path for a version."""
    out = {}
    base = os.path.join(ROOT, version)
    if not os.path.isdir(base):
        sys.exit(f"no archived schemas for {version}; try: ls api-schemas")
    for sub in ("v2", "common"):
        d = os.path.join(base, sub)
        if not os.path.isdir(d):
            continue
        for f in sorted(os.listdir(d)):
            if f.endswith(".schema.json"):
                out.setdefault(f[: -len(".schema.json")], os.path.join(d, f))
    return out


def properties(path):
    """Top-level property names, following into array items."""
    with open(path) as fh:
        try:
            doc = json.load(fh)
        except json.JSONDecodeError:
            return set()
    if doc.get("type") == "array":
        doc = doc.get("items", {})
    return set((doc.get("properties") or {}).keys())


def compare(old_version, new_version, name, old, new, verbose=True):
    before, after = properties(old[name]), properties(new[name])
    added, removed = sorted(after - before), sorted(before - after)
    if not added and not removed:
        if verbose:
            print(f"  {name}: identical ({len(before)} fields)")
        return False

    print(f"  {name}")
    for f in added:
        print(f"    + {f}   (only in {new_version} — do not send to {old_version})")
    for f in removed:
        print(f"    - {f}   (only in {old_version})")
    return True


def main():
    args = [a for a in sys.argv[1:] if a != "--list"]

    if "--list" in sys.argv:
        if len(args) < 1:
            sys.exit(__doc__)
        version, needle = args[0], (args[1] if len(args) > 1 else "")
        for n in sorted(schema_paths(version)):
            if needle in n:
                print(f"  {n}")
        return

    if len(args) < 2:
        sys.exit(__doc__)

    old_version, new_version = args[0], args[1]
    old, new = schema_paths(old_version), schema_paths(new_version)

    print(f"comparing {old_version} -> {new_version}")

    if len(args) >= 3:
        name = args[2]
        for v, table in ((old_version, old), (new_version, new)):
            if name not in table:
                sys.exit(f"  {name} does not exist in {v} "
                         f"(use --list {v} {name} to search)")
        compare(old_version, new_version, name, old, new)
        return

    shared = sorted(set(old) & set(new))
    changed = sum(compare(old_version, new_version, n, old, new, verbose=False)
                  for n in shared)
    print()
    print(f"  {changed} of {len(shared)} shared schemas differ")
    only_new = sorted(set(new) - set(old))
    if only_new:
        print(f"  {len(only_new)} schemas exist only in {new_version}: "
              f"{', '.join(only_new[:8])}{' ...' if len(only_new) > 8 else ''}")


if __name__ == "__main__":
    main()
