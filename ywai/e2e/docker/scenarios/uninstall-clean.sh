#!/usr/bin/env bash
# =============================================================================
# Cell: clean uninstall.
#
# Contract: install for every target, then `ywai uninstall --yes` removes the
# managed footprint: agent profiles, ywai skills and plugin entries. ywai
# keeps ~/.ywai on purpose (config + credentials) unless --purge is passed,
# so that directory staying behind is not a failure.
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"

cell_init uninstall-clean

echo "-- install for every target"
for t in $TARGETS; do
    install_target "$t"
done
assert_targets_installed

echo "-- uninstall --yes"
run_ywai uninstall --yes
rc=$?
assert_exit_zero "$rc" "uninstall --yes"
assert_not_contains "$RUN_OUT" "  ? " "uninstall reports no failed items"

echo "-- managed footprint must be gone"
for t in $TARGETS; do
    case "$t" in
    opencode)
        assert_absent_or_empty "$HOME/.config/opencode/agents"
        assert_absent_or_empty "$HOME/.config/opencode/skills"
        # install vendors bundles under ywai-plugins/ (see
        # internal/plugins/background_agents.go), so that is the dir the
        # uninstall plan targets (uninstall.go).
        assert_absent_or_empty "$HOME/.config/opencode/ywai-plugins"
        ;;
    claude-code)
        assert_absent_or_empty "$HOME/.claude/agents"
        assert_absent_or_empty "$HOME/.claude/skills"
        ;;
    esac
done

# The agent keys ywai manages must leave the OpenCode config. jq reads JSON
# only, so the key check runs for opencode.json; a .jsonc file only gets the
# existence check.
cfg="$(opencode_config_file)"
if [ -n "$cfg" ]; then
    case "$cfg" in
    *.json)
        assert_json_field "$cfg" 'has("agents")' 'false'
        assert_json_field "$cfg" 'has("agent")' 'false'
        ;;
    *)
        pass "config is $cfg; key check skipped (jsonc)"
        ;;
    esac
else
    pass "opencode config removed entirely"
fi
finish
