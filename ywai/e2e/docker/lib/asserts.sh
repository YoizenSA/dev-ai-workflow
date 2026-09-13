#!/usr/bin/env bash
# =============================================================================
# Shared assertion helpers for the Docker lifecycle matrix cells.
#
# A cell sources this file, runs assertions, and ends with `finish`.
# Failures are recorded and printed; the cell exits 1 when any failed.
#
# Environment contract (set by the images / compose services):
#   CELL_AGENT     opencode2 | claude | both           (matrix axis)
#   YWAI_BIN       binary under test (default /usr/local/bin/ywai)
#   YWAI_FAKE_LOG  where the fake agents log their argv
#   HOME           /home/ywai (UID 10001) for every cell
# =============================================================================

ASSERT_FAILURES=0
CELL_NAME="${CELL_NAME:-unknown-cell}"
# ywai agent names this cell installs for (space separated).
TARGETS=""
# Fixed flags for every non-interactive real install:
# no control-server autostart and no ponytail marketplace.
INSTALL_FLAGS=(--autostart=false --ponytail=false)

cell_init() {
    local scenario="$1"
    CELL_NAME="${CELL_AGENT:-unset}-${scenario}"
    case "${CELL_AGENT:-}" in
    opencode2) TARGETS="opencode" ;;
    claude) TARGETS="claude-code" ;;
    both) TARGETS="opencode claude-code" ;;
    *)
        TARGETS=""
        fail "CELL_AGENT must be one of: opencode2, claude, both (got '${CELL_AGENT:-}')"
        ;;
    esac
    YWAI_BIN="${YWAI_BIN:-/usr/local/bin/ywai}"
    YWAI_FAKE_LOG="${YWAI_FAKE_LOG:-$HOME/fake-agent.log}"
    # MODE=plain binaries read skills/agents from the repo checkout at /src
    # (data resolution walks up to go.mod). Embedded binaries must not rely
    # on it, so they run from HOME.
    if [ -f /src/go.mod ]; then
        YWAI_WORKDIR=/src
    else
        YWAI_WORKDIR="$HOME"
    fi
    export YWAI_BIN YWAI_FAKE_LOG YWAI_WORKDIR
}

fail() {
    ASSERT_FAILURES=$((ASSERT_FAILURES + 1))
    echo "FAIL [$CELL_NAME] $*"
}

pass() {
    echo "ok   [$CELL_NAME] $*"
}

# run_ywai runs the binary under test with stdin from /dev/null (no TTY).
# Output is captured to a file, not a pipe: install spawns a detached
# control-server child that inherits stdio, and a pipe never sees EOF, so
# a command substitution would hang forever after a successful install.
# Output lands in RUN_OUT; the ywai exit code is returned.
RUN_OUT=""
run_ywai() {
    local out="/tmp/ywai-run-out.$$"
    (cd "$YWAI_WORKDIR" && "$YWAI_BIN" "$@" </dev/null >"$out" 2>&1)
    local rc=$?
    RUN_OUT="$(cat "$out")"
    rm -f "$out"
    return $rc
}

# run_binary works like run_ywai but for an explicit binary path (used by
# update-swap to install with the previous build first).
run_binary() {
    local bin="$1"
    shift
    local out="/tmp/ywai-run-out.$$"
    (cd "$YWAI_WORKDIR" && "$bin" "$@" </dev/null >"$out" 2>&1)
    local rc=$?
    RUN_OUT="$(cat "$out")"
    rm -f "$out"
    return $rc
}

# stop_serve kills a detached control server left behind by install.
# Cells that snapshot HOME afterwards need this: a live serve keeps
# writing state files, which would show up as snapshot churn. Not an
# error when nothing is running.
stop_serve() {
    pkill -f "ywai serve" 2>/dev/null || true
    sleep 1
}

assert_exit_zero() {
    local rc="$1" what="$2"
    if [ "$rc" -eq 0 ]; then
        pass "$what exited 0"
    else
        fail "$what exited $rc (want 0). Last output lines:"
        printf '%s\n' "$RUN_OUT" | tail -n 12 | sed 's/^/       /'
    fi
}

assert_contains() {
    local haystack="$1" needle="$2" label="$3"
    case "$haystack" in
    *"$needle"*) pass "$label" ;;
    *) fail "$label: output has no '$needle'" ;;
    esac
}

assert_not_contains() {
    local haystack="$1" needle="$2" label="$3"
    case "$haystack" in
    *"$needle"*) fail "$label: output must not contain '$needle'" ;;
    *) pass "$label" ;;
    esac
}

assert_file() {
    if [ -f "$1" ]; then
        pass "file exists: $1"
    else
        fail "missing file: $1"
    fi
}

assert_dir_populated() {
    if [ -d "$1" ] && [ -n "$(ls -A "$1" 2>/dev/null)" ]; then
        pass "dir populated: $1"
    else
        fail "dir missing or empty: $1"
    fi
}

assert_absent_or_empty() {
    if [ ! -e "$1" ] || [ -z "$(ls -A "$1" 2>/dev/null)" ]; then
        pass "absent or empty: $1"
    else
        fail "not cleaned: $1 still holds: $(ls -A "$1" 2>/dev/null | head -n 5 | tr '\n' ' ')"
    fi
}

# assert_json_field FILE JQ_FILTER EXPECTED
# Needs jq, which only the tier B (install) image ships.
assert_json_field() {
    local file="$1" filter="$2" expected="$3" got
    if [ ! -f "$file" ]; then
        fail "missing JSON file: $file"
        return
    fi
    got="$(jq -r "$filter" "$file" 2>&1)"
    if [ "$got" = "$expected" ]; then
        pass "$file: $filter == $expected"
    else
        fail "$file: $filter = '$got', want '$expected'"
    fi
}

# opencode_config_file prints the OpenCode config ywai wrote, if any.
opencode_config_file() {
    if [ -f "$HOME/.config/opencode/opencode.json" ]; then
        echo "$HOME/.config/opencode/opencode.json"
    elif [ -f "$HOME/.config/opencode/opencode.jsonc" ]; then
        echo "$HOME/.config/opencode/opencode.jsonc"
    fi
}

version_file() {
    echo "$HOME/.ywai/version.json"
}

# assert_target_installed checks the on-disk footprint of one installed agent.
assert_target_installed() {
    local t="$1" cfg
    case "$t" in
    opencode)
        assert_dir_populated "$HOME/.config/opencode/skills"
        assert_dir_populated "$HOME/.config/opencode/agents"
        cfg="$(opencode_config_file)"
        if [ -n "$cfg" ]; then
            assert_file "$cfg"
        else
            fail "missing opencode.json(c) under $HOME/.config/opencode"
        fi
        ;;
    claude-code)
        assert_dir_populated "$HOME/.claude/skills"
        assert_dir_populated "$HOME/.claude/agents"
        ;;
    esac
}

# assert_targets_installed checks every target plus the version file.
assert_targets_installed() {
    local t
    for t in $TARGETS; do
        assert_target_installed "$t"
    done
    assert_file "$(version_file)"
}

# install_target runs one non-interactive real install for target $1 and
# asserts the stable contract: exit 0, Done marker, no fatal errors.
install_target() {
    local t="$1" rc
    run_ywai install --agent "$t" "${INSTALL_FLAGS[@]}"
    rc=$?
    assert_exit_zero "$rc" "install --agent $t"
    assert_contains "$RUN_OUT" "=== Done! ===" "install --agent $t prints the stable marker"
    assert_not_contains "$RUN_OUT" "=== Failed" "install --agent $t has no failed footer"
}

# swap_binary SRC DST [STAGE_DIR] replaces the binary at DST with SRC even
# while a process still runs from it: cp onto a busy executable fails with
# ETXTBSY (install leaves the control server mapped from the inode), but
# rename(2) over it is always legal. Same technique selfupdate uses. The
# stage dir must be writable by the cell user and on DST's filesystem;
# the default is the image-owned staging dir next to the spare binaries.
swap_binary() {
    local src="$1" dst="$2"
    local stage="${3:-/opt/ywai-e2e/bin}" tmp
    tmp="$stage/.swap-tmp"
    cp "$src" "$tmp" && chmod 0755 "$tmp" && mv -f "$tmp" "$dst"
}

finish() {
    echo "---- [$CELL_NAME] failures: $ASSERT_FAILURES ----"
    if [ "$ASSERT_FAILURES" -gt 0 ]; then
        echo "RESULT [$CELL_NAME]: FAIL"
        exit 1
    fi
    echo "RESULT [$CELL_NAME]: PASS"
    exit 0
}
