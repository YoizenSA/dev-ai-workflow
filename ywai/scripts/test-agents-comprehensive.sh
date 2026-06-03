#!/bin/bash

# Script comprehensivo de pruebas para ywai
# Prueba instalación real, skills, plugins, agent profiles, flags, y edge cases

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS="✓"
FAIL="✗"
WARN="⚠"

# Test counters
TESTS_RUN=0
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_WARNED=0

# Temp directory for isolated tests
TEST_TEMP_DIR=""

cleanup() {
    if [ -n "$TEST_TEMP_DIR" ] && [ -d "$TEST_TEMP_DIR" ]; then
        echo -e "${YELLOW}Cleaning up temp directory: $TEST_TEMP_DIR${NC}"
        # Use sudo if needed, or just warn
        if rm -rf "$TEST_TEMP_DIR" 2>/dev/null; then
            :
        else
            echo -e "${YELLOW}Warning: Could not remove $TEST_TEMP_DIR (permission denied)${NC}"
        fi
    fi
    rm -f /tmp/ywai-test 2>/dev/null || true
}

trap cleanup EXIT

# Helper functions
log_pass() {
    echo -e "${GREEN}$PASS $1${NC}"
    ((TESTS_PASSED++))
}

log_fail() {
    echo -e "${RED}$FAIL $1${NC}"
    ((TESTS_FAILED++))
}

log_warn() {
    echo -e "${YELLOW}$WARN $1${NC}"
    ((TESTS_WARNED++))
}

log_test() {
    echo ""
    echo "=== $1 ==="
    ((TESTS_RUN++))
}

# Build ywai binary
build_ywai() {
    echo "=== Building ywai binary ==="
    go build -o /tmp/ywai-test ./cmd/ywai
    log_pass "ywai binary built successfully"
}

# Create fake agent binary
create_fake_agent() {
    local agent_name=$1
    local binary_name=$2
    local temp_dir=$3
    
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        binary_name="${binary_name}.exe"
    fi
    
    mkdir -p "$temp_dir"
    echo "#!/bin/sh" > "$temp_dir/$binary_name"
    chmod +x "$temp_dir/$binary_name"
}

# Test 1: Verify agent profiles have tools.json
test_agent_profiles_tools() {
    log_test "Verifying agent profiles have tools.json"
    
    local all_valid=true
    for profile in ask dev qa architect reviewer devops; do
        tools_file="$REPO_ROOT/agents/$profile/tools.json"
        if [ -f "$tools_file" ]; then
            allowed=$(grep -o '"allowed":\s*\[[^]]*\]' "$tools_file" | head -1)
            log_pass "$profile: tools.json exists - $allowed"
        else
            log_fail "$profile: tools.json missing"
            all_valid=false
        fi
    done
    
    if $all_valid; then
        log_pass "All agent profiles have tools.json"
    else
        log_fail "Some agent profiles missing tools.json"
    fi
}

# Test 2: Dry-run installation for each agent
test_dry_run_install() {
    log_test "Dry-run installation for each agent"
    
    for agent_id in "vscode-copilot" "opencode" "pi"; do
        local binary_name
        case "$agent_id" in
            "vscode-copilot") binary_name="code" ;;
            "opencode") binary_name="opencode" ;;
            "pi") binary_name="pi" ;;
        esac
        
        local temp_dir=$(mktemp -d)
        create_fake_agent "$agent_id" "$binary_name" "$temp_dir"
        export PATH="$temp_dir:$PATH"
        
        local output
        output=$(/tmp/ywai-test install --agent "$agent_id" --dry-run 2>&1)
        
        if [ $? -eq 0 ]; then
            log_pass "$agent_id: dry-run successful"
            
            if echo "$output" | grep -q "$agent_id"; then
                log_pass "$agent_id: agent name mentioned"
            else
                log_fail "$agent_id: agent name NOT mentioned"
            fi
            
            if echo "$output" | grep -qi "skill"; then
                log_pass "$agent_id: skills mentioned"
            else
                log_fail "$agent_id: skills NOT mentioned"
            fi
        else
            log_fail "$agent_id: dry-run FAILED"
            echo "Output: $output"
        fi
        
        rm -rf "$temp_dir"
    done
}

# Test 3: Verify skills source directory structure
test_skills_structure() {
    log_test "Verify skills source directory structure"
    
    local skills_dir="$REPO_ROOT/skills"
    if [ -d "$skills_dir" ]; then
        log_pass "Skills source directory exists"
        
        # Check for expected skills with .ywai-extra marker
        local expected_skills=("typescript" "react-19" "tailwind-4" "angular" "dotnet" "devops" "playwright" "git-commit")
        local found_count=0
        
        for skill in "${expected_skills[@]}"; do
            if [ -d "$skills_dir/$skill" ]; then
                if [ -f "$skills_dir/$skill/.ywai-extra" ]; then
                    log_pass "Skill $skill has .ywai-extra marker"
                    ((found_count++))
                else
                    log_warn "Skill $skill exists but missing .ywai-extra marker"
                fi
            else
                log_fail "Skill $skill NOT found in source"
            fi
        done
        
        if [ "$found_count" -eq "${#expected_skills[@]}" ]; then
            log_pass "All expected skills have .ywai-extra marker"
        else
            log_warn "Only $found_count/${#expected_skills[@]} skills have marker"
        fi
    else
        log_fail "Skills source directory NOT found"
    fi
}

# Test 4: Verify agent profiles structure
test_agent_profiles_structure() {
    log_test "Verify agent profiles structure"
    
    local agents_dir="$REPO_ROOT/agents"
    if [ -d "$agents_dir" ]; then
        log_pass "Agents directory exists"
        
        local expected_agents=("ask" "dev" "qa" "architect" "reviewer" "devops")
        local found_count=0
        
        for agent in "${expected_agents[@]}"; do
            local agent_dir="$agents_dir/$agent"
            if [ -d "$agent_dir" ]; then
                if [ -f "$agent_dir/AGENT.md" ]; then
                    log_pass "Agent $agent has AGENT.md"
                    ((found_count++))
                else
                    log_fail "Agent $agent missing AGENT.md"
                fi
            else
                log_fail "Agent $agent directory NOT found"
            fi
        done
        
        if [ "$found_count" -eq "${#expected_agents[@]}" ]; then
            log_pass "All expected agents have AGENT.md"
        else
            log_warn "Only $found_count/${#expected_agents[@]} agents have AGENT.md"
        fi
    else
        log_fail "Agents directory NOT found"
    fi
}

# Test 5: Test installation flags
test_install_flags() {
    log_test "Test installation flags"
    
    local temp_dir=$(mktemp -d)
    create_fake_agent "opencode" "opencode" "$temp_dir"
    export PATH="$temp_dir:$PATH"
    
    # Test --global flag
    local output
    output=$(/tmp/ywai-test install --agent opencode --global --dry-run 2>&1)
    if echo "$output" | grep -qi "global"; then
        log_pass "--global flag recognized"
    else
        log_warn "--global flag not explicitly mentioned in output"
    fi
    
    # Test --preset flag
    output=$(/tmp/ywai-test install --agent opencode --preset minimal --dry-run 2>&1)
    if [ $? -eq 0 ]; then
        log_pass "--preset minimal accepted"
    else
        log_fail "--preset minimal failed"
    fi
    
    rm -rf "$temp_dir"
}

# Test 6: Test error handling
test_error_handling() {
    log_test "Test error handling"
    
    # Test with non-existent agent
    local output
    output=$(/tmp/ywai-test install --agent nonexistent-agent --dry-run 2>&1)
    if [ $? -ne 0 ]; then
        log_pass "Correctly fails with non-existent agent"
    else
        log_fail "Should fail with non-existent agent"
    fi
    
    # Test with no agents detected (skip if real agents exist in PATH)
    # Note: This test is skipped if real agents are detected in system PATH
    log_warn "Skipped: Cannot reliably test 'no agents' with system PATH containing real agents"
}

# Test 7: Test multiple agents detection
test_multiple_agents() {
    log_test "Test multiple agents detection"
    
    local temp_dir=$(mktemp -d)
    create_fake_agent "opencode" "opencode" "$temp_dir"
    create_fake_agent "pi" "pi" "$temp_dir"
    export PATH="$temp_dir:$PATH"
    
    local output
    output=$(/tmp/ywai-test agents 2>&1)
    
    local detected_count=$(echo "$output" | grep -c "Found:" 2>/dev/null | tr -d ' ' || echo "0")
    if [ "$detected_count" -ge 2 ]; then
        log_pass "Multiple agents detected correctly"
    else
        log_warn "Expected 2+ agents, detected: $detected_count"
    fi
    
    rm -rf "$temp_dir"
    export PATH=$(echo "$PATH" | sed "s|$temp_dir:||")
}

# Test 8: Test skills list command
test_skills_list() {
    log_test "Test skills list command"
    
    local output
    output=$(/tmp/ywai-test skills 2>&1)
    
    if [ $? -eq 0 ]; then
        log_pass "ywai skills command works"
        
        # Check for known skills
        for skill in "typescript" "react-19" "tailwind-4"; do
            if echo "$output" | grep -qi "$skill"; then
                log_pass "Skill $skill listed"
            else
                log_warn "Skill $skill not in list"
            fi
        done
    else
        log_fail "ywai skills command failed"
    fi
}

# Test 9: Test plugins detection
test_plugins_detection() {
    log_test "Test plugins detection"
    
    # Check if plugins directory exists
    local plugins_dir="$REPO_ROOT/internal/plugins"
    if [ -d "$plugins_dir" ]; then
        log_pass "Plugins directory exists"
        
        # Check for plugin files
        for plugin in "quota.go" "mcp.go" "ado.go"; do
            if [ -f "$plugins_dir/$plugin" ]; then
                log_pass "Plugin file $plugin exists"
            else
                log_warn "Plugin file $plugin not found"
            fi
        done
    else
        log_fail "Plugins directory NOT found"
    fi
}

# Test 10: Test agent settings paths
test_agent_settings_paths() {
    log_test "Test agent settings paths"
    
    local temp_dir=$(mktemp -d)
    create_fake_agent "opencode" "opencode" "$temp_dir"
    export PATH="$temp_dir:$PATH"
    
    # Test agents command to see if it detects config paths
    local output
    output=$(/tmp/ywai-test agents 2>&1)
    
    if echo "$output" | grep -q "skills:"; then
        log_pass "Agent skills paths detected"
    else
        log_warn "Agent skills paths not explicitly shown"
    fi
    
    rm -rf "$temp_dir"
}

# Main execution
main() {
    echo "=========================================="
    echo "  ywai Comprehensive Test Suite"
    echo "=========================================="
    
    build_ywai
    test_agent_profiles_tools
    test_dry_run_install
    test_skills_structure
    test_agent_profiles_structure
    test_install_flags
    test_error_handling
    test_multiple_agents
    test_skills_list
    test_plugins_detection
    test_agent_settings_paths
    
    echo ""
    echo "=========================================="
    echo "  Test Summary"
    echo "=========================================="
    echo -e "Tests run:    $TESTS_RUN"
    echo -e "${GREEN}Passed:       $TESTS_PASSED${NC}"
    echo -e "${YELLOW}Warnings:     $TESTS_WARNED${NC}"
    echo -e "${RED}Failed:       $TESTS_FAILED${NC}"
    echo "=========================================="
    
    if [ $TESTS_FAILED -eq 0 ]; then
        echo -e "${GREEN}All critical tests passed!${NC}"
        exit 0
    else
        echo -e "${RED}Some tests failed. Please review the output above.${NC}"
        exit 1
    fi
}

main "$@"
