# Requires: bats (https://github.com/bats-core/bats-core)
#
# TDD RED phase — these tests validate that prepare-embedded.sh exits with
# code 1 when bun is unavailable AND no prebuilt background-agents bundle
# exists. They will fail against the OLD code (which only warned silently
# and continued to exit 0) and pass once the fix is applied.

setup() {
    export TEST_TMPDIR=$(mktemp -d)

    # Create the full mock repo structure so the OLD code (without exit 1)
    # runs cleanly to completion and exits 0.  If we only created a minimal
    # tree, the subsequent cp -a / ls / find commands would fail under
    # set -euo pipefail, masking the difference between old and new behavior.

    # Script location — REPO_ROOT resolves to TEST_TMPDIR
    mkdir -p "$TEST_TMPDIR/scripts"

    # Source dirs copied into the embedded data tree after the bun check
    mkdir -p "$TEST_TMPDIR/skills/example-skill"
    mkdir -p "$TEST_TMPDIR/agents"
    touch  "$TEST_TMPDIR/agents/AGENT.md"

    # WEB_DIR (= REPO_ROOT/internal/control/web) — npm build is skipped
    # (npm not in PATH), but cp -a "$WEB_DIR/dist/." still requires the
    # source directory to exist.
    mkdir -p "$TEST_TMPDIR/internal/control/web/dist"

    # Background-agents source tree — the bundle dist/ DOES NOT exist,
    # which is the scenario under test.
    mkdir -p "$TEST_TMPDIR/plugins/background-agents/src/plugin"
    touch "$TEST_TMPDIR/plugins/background-agents/src/plugin/background-agents.ts"

    # Copy the real script-under-test into the mock tree so $0 resolves
    # inside TEST_TMPDIR and REPO_ROOT / BA_DIR / BA_BUNDLE follow suit.
    cp "$BATS_TEST_DIRNAME/../prepare-embedded.sh" "$TEST_TMPDIR/scripts/"
}

teardown() {
    rm -rf "$TEST_TMPDIR"
}

# ---------------------------------------------------------------------------
# Helper — produce a colon-separated PATH that excludes every directory
# containing a `bun` or `npm` executable.  Without this, `command -v bun`
# could find a system-installed bun and the script would attempt to build
# the bundle rather than hitting the else branch.
# ---------------------------------------------------------------------------
filter_path() {
    local result="" dir
    while IFS= read -r -d ':' dir || [ -n "$dir" ]; do
        [ -z "$dir" ] && continue
        [ ! -d "$dir" ] && continue
        [ -x "$dir/bun" ] && continue
        [ -x "$dir/npm" ] && continue
        result="${result:+${result}:}${dir}"
    done <<< "${PATH}:"
    printf '%s' "$result"
}

@test "exits with code 1 when bun is unavailable and no prebuilt bundle exists" {
    local cleaned_path
    cleaned_path="$(filter_path)"

    # Ensure we still have basic utilities (dirname, …) reachable.
    # If filtering somehow emptied the path, add a minimal fallback.
    if [ -z "$cleaned_path" ]; then
        cleaned_path="/usr/bin:/bin"
    fi

    # Run the script with a cleansed environment.  HOME is set to a temp
    # subdirectory so the candidate check $HOME/.bun/bin/bun also misses.
    run env PATH="$cleaned_path" HOME="$TEST_TMPDIR/home" \
        bash "$TEST_TMPDIR/scripts/prepare-embedded.sh"

    echo "=== bats captured output ===" >&2
    echo "status: $status" >&2
    echo "output: $output" >&2

    [ "$status" -eq 1 ]
}

@test "prints an error to stderr mentioning 'ERROR' or 'bun'" {
    local cleaned_path
    cleaned_path="$(filter_path)"
    if [ -z "$cleaned_path" ]; then
        cleaned_path="/usr/bin:/bin"
    fi

    # Merge stderr → stdout so bats captures the error message in $output.
    run bash -c \
        "export PATH='$cleaned_path'; export HOME='$TEST_TMPDIR/home'; \
         bash '$TEST_TMPDIR/scripts/prepare-embedded.sh' 2>&1"

    echo "=== bats captured output ===" >&2
    echo "status: $status" >&2
    echo "output: $output" >&2

    [ "$status" -eq 1 ] || { echo "FAIL: expected exit code 1, got $status" >&2; false; }
}
