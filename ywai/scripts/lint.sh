#!/usr/bin/env bash
# Same Go lint gate as .github/workflows/ci.yml (vet + golangci-lint).
# Used by git hooks and `scripts/dev.sh lint`.
#
#   bash scripts/lint.sh           # full tree (pre-push / CI parity)
#   bash scripts/lint.sh --staged  # staged ywai/*.go only (pre-commit)
set -euo pipefail

GOLANGCI_VERSION="v2.12.2"
TIMEOUT="3m"

find_ywai_root() {
    local dir
    dir="$(cd "$(dirname "$0")/.." && pwd)"
    echo "$dir"
}

find_golangci() {
    if [[ -n "${GOLANGCI_LINT:-}" ]]; then
        if [[ -x "${GOLANGCI_LINT}" ]]; then
            echo "${GOLANGCI_LINT}"
            return 0
        fi
        return 1
    fi
    if command -v golangci-lint >/dev/null 2>&1; then
        command -v golangci-lint
        return 0
    fi
    local gopath
    gopath="$(go env GOPATH 2>/dev/null || true)"
    if [[ -n "$gopath" && -x "$gopath/bin/golangci-lint" ]]; then
        echo "$gopath/bin/golangci-lint"
        return 0
    fi
    return 1
}

die_missing() {
    echo "golangci-lint not found. Install the same version CI uses:" >&2
    echo "  go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@${GOLANGCI_VERSION}" >&2
    exit 1
}

repo_staged_go() {
    if [[ -n "${STAGED_GO_FILES+x}" ]]; then
        if [[ -n "${STAGED_GO_FILES}" ]]; then
            # shellcheck disable=SC2086
            printf '%s\n' ${STAGED_GO_FILES}
        fi
        return 0
    fi
    git -C "$(git rev-parse --show-toplevel)" diff --cached --name-only --diff-filter=ACMR \
        | grep -E '^ywai/.*\.go$' || true
}

YWAI_ROOT="$(find_ywai_root)"
cd "$YWAI_ROOT"

if [[ "${1:-}" == "--staged" ]]; then
    mapfile -t staged < <(repo_staged_go)
    if [[ ${#staged[@]} -eq 0 ]]; then
        echo "lint: no staged Go files"
        exit 0
    fi

    rels=()
    for f in "${staged[@]}"; do
        rel="${f#ywai/}"
        rels+=("./${rel}")
    done

    unformatted="$(gofmt -l "${rels[@]}")"
    if [[ -n "$unformatted" ]]; then
        echo "gofmt needed on:" >&2
        echo "$unformatted" >&2
        echo "Fix: (cd ywai && gofmt -w ${rels[*]})" >&2
        exit 1
    fi

    bin="$(find_golangci)" || die_missing
    echo "lint (staged): ${rels[*]}"
    "$bin" run --timeout="$TIMEOUT" "${rels[@]}"
    echo "lint: ok"
    exit 0
fi

bin="$(find_golangci)" || die_missing
echo "go vet ./..."
go vet ./...
echo "golangci-lint run --timeout=${TIMEOUT}"
"$bin" run --timeout="$TIMEOUT"
echo "lint: ok"
