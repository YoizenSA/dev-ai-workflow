#!/usr/bin/env bash
# Fake claude binary for the Docker lifecycle matrix.
#
# ywai detects Claude Code by finding this name on PATH. The stub:
#   - appends its argv to $YWAI_FAKE_LOG so scenarios (and humans) can
#     inspect which agent invocations happened
#   - answers "2.0.0" on `--version` and `version` (detection probes it)
#   - always exits 0
set -u

log="${YWAI_FAKE_LOG:-$HOME/fake-agent.log}"
printf '%s\n' "$*" >>"$log"

case "${1:-}" in
--version | version)
    echo "2.0.0"
    ;;
esac
exit 0
