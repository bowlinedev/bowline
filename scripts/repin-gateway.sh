#!/usr/bin/env bash
set -euo pipefail

config="${1:?usage: repin-gateway.sh path/to/bowline.gateway.json}"

python3 - "$config" <<'PY'
import json, pathlib, sys

config = pathlib.Path(sys.argv[1])
doc = json.loads(config.read_text())
base = config.parent
changed = []
for name, service in sorted(doc.get("services", {}).items()):
    contract = base / service["contract"]
    got = json.loads(contract.read_text())["hash"]
    if service.get("version") != got:
        changed.append(f"{name} -> {got}")
        service["version"] = got
if changed:
    config.write_text(json.dumps(doc, indent=2) + "\n")
    for line in changed:
        print(f"repinned {line}")
else:
    print("pins already match")
PY
