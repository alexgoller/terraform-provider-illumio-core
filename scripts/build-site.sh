#!/usr/bin/env bash
#
# Sync the provider documentation into the Jekyll site under website/.
#
# docs/ is the single source of truth: it is what the Terraform Registry
# publishes and what tfplugindocs generates. This script copies it into
# website/ and rewrites the front matter for Jekyll, so the GitHub Pages site
# and the Registry never drift apart.
#
# Synced content is NOT committed - it is regenerated on every build.
#
# Usage:
#   scripts/build-site.sh          # sync only
#   scripts/build-site.sh --serve  # sync, then serve locally (needs jekyll)
#
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC="${ROOT}/docs"
DEST="${ROOT}/website"

[ -d "$SRC" ] || { echo "error: ${SRC} not found" >&2; exit 1; }

# Remove previously synced content, leaving the site's own files in place.
rm -rf "${DEST}/resources" "${DEST}/data-sources" "${DEST}/guides"

python3 - "$SRC" "$DEST" <<'PY'
import os, re, sys

src, dest = sys.argv[1], sys.argv[2]

# Only these subtrees are published. docs/superpowers/ holds internal design
# specs, which are deliberately excluded from the public site.
SUBDIRS = ("resources", "data-sources", "guides")

def split_front_matter(text):
    if not text.startswith("---"):
        return {}, text
    end = text.find("\n---", 3)
    if end == -1:
        return {}, text
    raw, body = text[3:end], text[end + 4:]

    meta, key = {}, None
    for line in raw.splitlines():
        if not line.strip() or line.lstrip().startswith("#"):
            continue
        m = re.match(r'^(\w[\w-]*):\s*(.*)$', line)
        if m:
            key = m.group(1)
            value = m.group(2).strip()
            # A block scalar indicator is not content; the value is on the
            # following indented lines.
            meta[key] = "" if value in ("|", "|-", ">", ">-") else value
        elif key:
            meta[key] = (meta[key] + " " + line.strip()).strip()
    return meta, body.lstrip("\n")

def yaml_quote(value):
    return '"' + value.replace('\\', '\\\\').replace('"', '\\"') + '"'

count = 0
for subdir in SUBDIRS:
    srcdir = os.path.join(src, subdir)
    if not os.path.isdir(srcdir):
        continue
    outdir = os.path.join(dest, subdir)
    os.makedirs(outdir, exist_ok=True)

    for name in sorted(os.listdir(srcdir)):
        if not name.endswith(".md"):
            continue
        with open(os.path.join(srcdir, name)) as fh:
            meta, body = split_front_matter(fh.read())

        # tfplugindocs writes page_title; Jekyll wants title.
        title = meta.get("page_title", "").strip('"')
        if not title:
            title = name[:-3]
        else:
            # "illumio-core_deny_rule Resource - terraform-provider-illumio-core"
            title = title.split(" - ")[0].strip()

        desc = meta.get("description", "").strip()

        front = ["---", "layout: default", f"title: {yaml_quote(title)}"]
        if desc:
            front.append(f"description: {yaml_quote(desc)}")
        front.append("---")

        # Provider docs contain literal ${...} and can contain {{ }} inside HCL
        # examples. raw stops Liquid from trying to interpret any of it;
        # Markdown still renders normally.
        out = "\n".join(front) + "\n\n{% raw %}\n" + body.rstrip() + "\n{% endraw %}\n"

        with open(os.path.join(outdir, name), "w") as fh:
            fh.write(out)
        count += 1

print(f"synced {count} pages into website/")
PY

# The generated provider index (provider configuration reference) becomes its
# own page. website/index.md is the hand-written landing page and is committed.
if [ -f "${SRC}/index.md" ]; then
  python3 - "${SRC}/index.md" "${DEST}/guides/provider-configuration.md" <<'PY'
import sys
src, dest = sys.argv[1], sys.argv[2]
text = open(src).read()
body = text
if text.startswith("---"):
    end = text.find("\n---", 3)
    if end != -1:
        body = text[end + 4:].lstrip("\n")
front = '---\nlayout: default\ntitle: "Provider Configuration"\n---\n\n'
open(dest, "w").write(front + "{% raw %}\n" + body.rstrip() + "\n{% endraw %}\n")
print("synced provider configuration page")
PY
fi

if [ "${1:-}" = "--serve" ]; then
  command -v jekyll >/dev/null 2>&1 || { echo "error: jekyll not installed" >&2; exit 1; }
  cd "$DEST" && exec jekyll serve
fi
