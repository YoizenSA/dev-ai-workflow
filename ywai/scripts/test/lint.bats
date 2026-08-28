# Requires: bats (https://github.com/bats-core/bats-core)
#
# lint.sh must fail closed when golangci-lint is missing (same gate as CI),
# and skip the binary when --staged has no Go files.

setup() {
    SCRIPT="$BATS_TEST_DIRNAME/../lint.sh"
}

@test "full lint fails when golangci-lint is missing" {
    run env PATH="/usr/bin:/bin" GOLANGCI_LINT="/no/such/golangci-lint" bash "$SCRIPT"
    [ "$status" -ne 0 ]
    [[ "$output" == *"golangci-lint/v2/cmd/golangci-lint@"* ]]
}

@test "staged lint with no Go files succeeds without golangci-lint" {
    run env PATH="/usr/bin:/bin" GOLANGCI_LINT="/no/such/golangci-lint" STAGED_GO_FILES="" bash "$SCRIPT" --staged
    [ "$status" -eq 0 ]
}
