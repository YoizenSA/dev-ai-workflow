#!/usr/bin/env bash
# ---------------------------------------------------------------------------
# validate-workflows.sh — YAML syntax check for GitHub Actions workflow files.
#
# Tries, in order:
#   1. python3 + PyYAML (import yaml)
#   2. node + js-yaml   (require('js-yaml'))
#   3. yamllint
#
# Exits 0 if ALL files are valid YAML, 1 otherwise.
# ---------------------------------------------------------------------------
set -euo pipefail

# Locate the workflow directory relative to this script.
WORKFLOW_DIR="$(cd "$(dirname "$0")/../../../.github/workflows" && pwd)"
FILES=(
    "$WORKFLOW_DIR/ci.yml"
    "$WORKFLOW_DIR/release.yml"
)

errors=0

# ---------------------------------------------------------------------------
# check_python3  <file>
# ---------------------------------------------------------------------------
check_python3() {
    local file="$1"
    python3 -c "
import sys
try:
    import yaml
    yaml.safe_load(open('$file'))
    sys.exit(0)
except ImportError:
    sys.exit(2)   # fall through to next checker
except yaml.YAMLError as e:
    print(f'INVALID (python3+yaml): $file — {e}', file=sys.stderr)
    sys.exit(1)
except Exception as e:
    print(f'ERROR (python3+yaml): $file — {e}', file=sys.stderr)
    sys.exit(1)
"
}

# ---------------------------------------------------------------------------
# check_node  <file>
# ---------------------------------------------------------------------------
check_node() {
    local file="$1"
    node -e "
try {
    require('js-yaml').load(require('fs').readFileSync('$file', 'utf8'));
} catch (e) {
    console.error('INVALID (node+js-yaml): $file — ' + e.message);
    process.exit(1);
}
" 2>/dev/null
}

# ---------------------------------------------------------------------------
# check_yamllint  <file>
# ---------------------------------------------------------------------------
check_yamllint() {
    local file="$1"
    if command -v yamllint &>/dev/null; then
        yamllint "$file"
    else
        return 2   # not available
    fi
}

# ---------------------------------------------------------------------------
# validate  <file>  — try each checker in order
# ---------------------------------------------------------------------------
validate() {
    local file="$1"
    echo "Checking: $file"

    if check_python3 "$file"; then
        return 0
    fi
    local rc=$?
    [ $rc -eq 2 ] || return $rc   # real error → abort this file

    if check_node "$file"; then
        return 0
    fi
    rc=$?
    [ $rc -eq 2 ] || return $rc

    if check_yamllint "$file"; then
        return 0
    fi
    rc=$?

    # None of the checkers succeeded or were available.
    echo "WARNING: $file — no YAML validator available (tried python3+yaml, node+js-yaml, yamllint)." >&2
    return 1
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------
for f in "${FILES[@]}"; do
    if [ ! -f "$f" ]; then
        echo "ERROR: $f not found" >&2
        errors=1
        continue
    fi
    if validate "$f"; then
        echo "  OK"
    else
        errors=1
    fi
done

exit $errors
