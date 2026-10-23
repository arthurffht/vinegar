#!/usr/bin/env python3
"""Generate Flatpak sources for vendoring Go modules offline.

Runs 'go mod vendor' into a temporary directory to obtain the exact
modules.txt, then emits an archive source for every vendored module,
fetched from the Go module proxy and verified by checksum.

Usage: build-aux/flatpak/update-go-sources.py
"""

import hashlib
import json
import os
import shutil
import subprocess
import tempfile
import urllib.request

ROOT = os.path.dirname(os.path.dirname(os.path.dirname(os.path.abspath(__file__))))
OUT = os.path.join(ROOT, "build-aux", "flatpak")
PROXY = "https://proxy.golang.org"


def escape(path):
    return "".join("!" + c.lower() if c.isupper() else c for c in path)


def sha256(url):
    cache = subprocess.check_output(["go", "env", "GOMODCACHE"], text=True).strip()
    mod, ver = url[len(PROXY) + 1 :].split("/@v/")
    local = os.path.join(cache, "cache", "download", mod, "@v", ver)
    h = hashlib.sha256()
    if os.path.exists(local):
        with open(local, "rb") as f:
            h.update(f.read())
    else:
        with urllib.request.urlopen(url) as r:
            h.update(r.read())
    return h.hexdigest()


def main():
    with tempfile.TemporaryDirectory() as tmp:
        vendor = os.path.join(tmp, "vendor")
        subprocess.check_call(["go", "mod", "vendor", "-o", vendor], cwd=ROOT)
        modules_txt = os.path.join(vendor, "modules.txt")
        shutil.copy(modules_txt, os.path.join(OUT, "modules.txt"))
        with open(modules_txt) as f:
            lines = f.read().splitlines()

    sources = [{"type": "file", "path": "modules.txt", "dest": "vendor"}]
    for line in lines:
        if not line.startswith("# "):
            continue
        fields = line[2:].split()
        # Replacement directives are listed again, without a version.
        if len(fields) < 2 or fields[1] == "=>":
            continue
        path, version = fields[0], fields[1]
        src_path, src_version = path, version
        if "=>" in fields:
            i = fields.index("=>")
            src_path, src_version = fields[i + 1], fields[i + 2]
        url = f"{PROXY}/{escape(src_path)}/@v/{src_version}.zip"
        sources.append(
            {
                "type": "archive",
                "url": url,
                "sha256": sha256(url),
                "strip-components": len(src_path.split("/")),
                "dest": f"vendor/{path}",
            }
        )

    with open(os.path.join(OUT, "go-sources.json"), "w") as f:
        json.dump(sources, f, indent=2)
        f.write("\n")
    print(f"Wrote {len(sources) - 1} modules")


if __name__ == "__main__":
    main()
