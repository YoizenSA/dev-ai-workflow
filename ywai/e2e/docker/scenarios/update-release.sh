#!/usr/bin/env bash
# =============================================================================
# Cell: real release update N-1 -> N (nightly, needs network).
#
# Downloads the previous stable release, installs it, then runs `ywai update`
# and asserts the self-update path: checksum-verified download of N-1, a
# version bump to N, and managed state still applied. This is the only cell
# that touches real releases; update-swap is the offline twin.
#
# Skips (pass, with a note) while the project has fewer than two stable
# releases: the N-1 -> N transition cannot exist yet.
# =============================================================================
set -uo pipefail
source "$(cd "$(dirname "$0")" && pwd)/../lib/asserts.sh"

cell_init update-release

REPO="YoizenSA/dev-ai-workflow"
if [ "$CELL_AGENT" != "opencode2" ]; then
    echo "skip: update-release runs only for the opencode2 host"
    finish
fi

api_get() {
    curl -fsSL --retry 3 --connect-timeout 20 "https://api.github.com/repos/$REPO/$1"
}

echo "-- resolve latest (N) and previous (N-1) stable releases"
latest_tag="$(api_get releases/latest | jq -r '.tag_name')"
if [ -z "$latest_tag" ] || [ "$latest_tag" = "null" ]; then
    fail "cannot read the latest release from the GitHub API"
    finish
fi
latest="${latest_tag#v}"

# Stable tags only (no pre-release suffix), sorted descending; the first one
# strictly below N is N-1.
prev="$(api_get "releases?per_page=100" \
    | jq -r '.[].tag_name' \
    | grep -E '^v?[0-9]+\.[0-9]+\.[0-9]+$' \
    | sed 's/^v//' \
    | sort -rV \
    | awk -v cur="$latest" '$0 < cur {print; exit}')"
if [ -z "$prev" ]; then
    echo "skip: only one stable release ($latest) exists; N-1 -> N cannot run yet"
    finish
fi
pass "update path: v$prev -> v$latest"

echo "-- download and verify release v$prev"
tmp="/tmp/ywai-release-$prev"
asset="ywai_${prev}_linux_amd64.tar.gz"
mkdir -p "$tmp"
curl -fsSL --retry 3 -o "$tmp/$asset" "https://github.com/$REPO/releases/download/v$prev/$asset"
curl -fsSL --retry 3 -o "$tmp/checksums.txt" "https://github.com/$REPO/releases/download/v$prev/checksums.txt"
want="$(awk -v a="$asset" '$2 == "/"a || $2 == a {print $1}' "$tmp/checksums.txt" | head -n 1)"
got="$(sha256sum "$tmp/$asset" | awk '{print $1}')"
if [ -n "$want" ] && [ "$want" = "$got" ]; then
    pass "checksum matches checksums.txt"
else
    fail "checksum mismatch for $asset (want '$want', got '$got')"
    finish
fi
tar -xzf "$tmp/$asset" -C "$tmp"
assert_file "$tmp/ywai"

echo "-- install release v$prev"
# /usr/local/bin/ywai is chown ywai in the image so a cell can replace it.
cp "$tmp/ywai" /usr/local/bin/ywai
run_ywai install --agent opencode "${INSTALL_FLAGS[@]}"
rc=$?
assert_exit_zero "$rc" "v$prev install"
assert_contains "$RUN_OUT" "=== Done! ===" "v$prev install prints the stable marker"
assert_target_installed opencode
assert_json_field "$(version_file)" .installed "$prev"

echo "-- ywai update: self-update v$prev -> v$latest"
run_ywai update --agent opencode
rc=$?
assert_exit_zero "$rc" "update"
assert_contains "$RUN_OUT" "Updated:" "update reports the version bump"
assert_contains "$RUN_OUT" "=== Done! ===" "update prints the stable marker"

echo "-- the binary on PATH is now N and state matches"
final="$(cd "$YWAI_WORKDIR" && "$YWAI_BIN" --version </dev/null 2>&1)"
assert_contains "$final" "ywai version $latest" "binary reports $latest (got: $final)"
assert_target_installed opencode
assert_json_field "$(version_file)" .installed "$latest"
finish
