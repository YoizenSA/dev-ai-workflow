#!/usr/bin/env bash
# One-shot syntax check for the docker matrix scripts (dev convenience).
set -e
cd "$(dirname "$0")/../../.."   # repo root
status=0
for f in \
    ywai/e2e/docker/run-matrix.sh \
    ywai/e2e/docker/lib/asserts.sh \
    ywai/e2e/docker/lib/snapshot.sh \
    ywai/e2e/docker/fakes/opencode2.sh \
    ywai/e2e/docker/fakes/claude.sh \
    ywai/e2e/docker/scenarios/dryrun.sh \
    ywai/e2e/docker/scenarios/fresh.sh \
    ywai/e2e/docker/scenarios/reinstall.sh \
    ywai/e2e/docker/scenarios/update-swap.sh \
    ywai/e2e/docker/scenarios/update-release.sh \
    ywai/e2e/docker/scenarios/uninstall-clean.sh \
    ywai/scripts/dev.sh; do
    if bash -n "$f"; then
        echo "OK   $f"
    else
        echo "FAIL $f"
        status=1
    fi
done
exit $status
