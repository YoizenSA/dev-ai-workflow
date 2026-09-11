#!/usr/bin/env bash
# Dev convenience: parse the workflow and compose YAMLs with python3+pyyaml.
set -e
cd "$(dirname "$0")/../../.."
python3 - <<'EOF'
import yaml
for f in [
    ".github/workflows/docker-matrix.yml",
    ".github/workflows/release.yml",
    "ywai/e2e/docker/compose.yaml",
]:
    with open(f) as fh:
        data = yaml.safe_load(fh)
    print("OK  ", f, "-", len(data.get("jobs", data.get("services", {}))), "top-level entries")
EOF
