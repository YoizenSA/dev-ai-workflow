#!/usr/bin/env bash
# Setup AI Skills for GA (Guardian Agent) projects
# Configures AI coding assistants that follow agentskills.io standard:
#   - Claude Code: .claude/skills/ symlink + CLAUDE.md copies
#   - Gemini CLI: .gemini/skills/ symlink + GEMINI.md copies
#   - Codex (OpenAI): .codex/skills/ symlink + AGENTS.md (native)
#   - GitHub Copilot: .github/skills/ symlink
#   - Cursor: .cursor/skills/ symlink + CURSOR.md copies + .cursorrules
#
# Usage:
#   ./setup.sh              # Interactive mode (select AI assistants)
#   ./setup.sh --all        # Configure all AI assistants
#   ./setup.sh --claude     # Configure Claude Code
#   ./setup.sh --cursor     # Configure Cursor
#   ./setup.sh --opencode   # Configure OpenCode
#   ./setup.sh --gemini     # Configure Gemini CLI

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(pwd)"
SKILLS_SOURCE="$SCRIPT_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Selection flags
SETUP_CLAUDE=false
SETUP_CURSOR=false
SETUP_OPENCODE=false
SETUP_GEMINI=false
SETUP_CODEX=false
SETUP_COPILOT=false

# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Configure AI coding assistants for your project."
    echo ""
    echo "Options:"
    echo "  --all        Configure all AI assistants"
    echo "  --claude     Configure Claude Code"
    echo "  --cursor     Configure Cursor"
    echo "  --opencode   Configure OpenCode"
    echo "  --gemini     Configure Gemini CLI"
    echo "  --codex      Configure Codex (OpenAI)"
    echo "  --copilot    Configure GitHub Copilot"
    echo "  --help       Show this help message"
    echo ""
    echo "If no options provided, runs in interactive mode."
}

show_menu() {
    echo -e "${BOLD}Which AI assistants do you use?${NC}"
    echo -e "${CYAN}(Use numbers to toggle, Enter to confirm)${NC}"
    echo ""

    local options=("Claude Code" "Cursor" "OpenCode" "Gemini CLI" "Codex (OpenAI)" "GitHub Copilot")
    local selected=(true false false false false false)  # Claude selected by default

    while true; do
        for i in "${!options[@]}"; do
            if [ "${selected[$i]}" = true ]; then
                echo -e "  ${GREEN}[x]${NC} $((i+1)). ${options[$i]}"
            else
                echo -e "  [ ] $((i+1)). ${options[$i]}"
            fi
        done
        echo ""
        echo -e "  ${YELLOW}a${NC}. Select all"
        echo -e "  ${YELLOW}n${NC}. Select none"
        echo ""
        echo -n "Toggle (1-6, a, n) or Enter to confirm: "

        read -r choice

        case $choice in
            1) selected[0]=$([ "${selected[0]}" = true ] && echo false || echo true) ;;
            2) selected[1]=$([ "${selected[1]}" = true ] && echo false || echo true) ;;
            3) selected[2]=$([ "${selected[2]}" = true ] && echo false || echo true) ;;
            4) selected[3]=$([ "${selected[3]}" = true ] && echo false || echo true) ;;
            5) selected[4]=$([ "${selected[4]}" = true ] && echo false || echo true) ;;
            6) selected[5]=$([ "${selected[5]}" = true ] && echo false || echo true) ;;
            a|A) selected=(true true true true true true) ;;
            n|N) selected=(false false false false false false) ;;
            "") break ;;
            *) echo -e "${RED}Invalid option${NC}" ;;
        esac

        # Move cursor up to redraw menu
        echo -en "\033[12A\033[J"
    done

    SETUP_CLAUDE=${selected[0]}
    SETUP_CURSOR=${selected[1]}
    SETUP_OPENCODE=${selected[2]}
    SETUP_GEMINI=${selected[3]}
    SETUP_CODEX=${selected[4]}
    SETUP_COPILOT=${selected[5]}
}

setup_claude() {
    local target="$REPO_ROOT/.claude/skills"
    mkdir -p "$REPO_ROOT/.claude"

    if [ -L "$target" ]; then
        rm "$target"
    elif [ -d "$target" ]; then
        mv "$target" "$REPO_ROOT/.claude/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$target"
    echo -e "${GREEN}  ✓ .claude/skills -> skills/ (Claude Code/OpenCode)${NC}"

    copy_agents_md "CLAUDE.md"
    
    # Copy .gitignore to .claude directory
    if [ -f "$REPO_ROOT/.gitignore" ]; then
        cp "$REPO_ROOT/.gitignore" "$REPO_ROOT/.claude/.gitignore"
        echo -e "${GREEN}  ✓ Copied .gitignore to .claude/${NC}"
    fi
    
    # Create Claude.md with Claude-specific instructions
    create_claude_md
}

setup_cursor() {
    local target="$REPO_ROOT/.cursor/skills"
    mkdir -p "$REPO_ROOT/.cursor"

    if [ -L "$target" ]; then
        rm "$target"
    elif [ -d "$target" ]; then
        mv "$target" "$REPO_ROOT/.cursor/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$target"
    echo -e "${GREEN}  ✓ .cursor/skills -> skills/ (Cursor)${NC}"

    copy_agents_md "CURSOR.md"
    
    if [ -f "$REPO_ROOT/AGENTS.MD" ]; then
        cp "$REPO_ROOT/AGENTS.MD" "$REPO_ROOT/.cursorrules"
        echo -e "${GREEN}  ✓ AGENTS.MD -> .cursorrules${NC}"
    fi
}

setup_opencode() {
    setup_claude
}

setup_gemini() {
    local target="$REPO_ROOT/.gemini/skills"
    mkdir -p "$REPO_ROOT/.gemini"

    if [ -L "$target" ]; then
        rm "$target"
    elif [ -d "$target" ]; then
        mv "$target" "$REPO_ROOT/.gemini/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$target"
    echo -e "${GREEN}  ✓ .gemini/skills -> skills/${NC}"

    copy_agents_md "GEMINI.md"
    
    # Copy .gitignore to .gemini directory
    if [ -f "$REPO_ROOT/.gitignore" ]; then
        cp "$REPO_ROOT/.gitignore" "$REPO_ROOT/.gemini/.gitignore"
        echo -e "${GREEN}  ✓ Copied .gitignore to .gemini/${NC}"
    fi
    
    # Create gemini.md with Gemini-specific instructions
    create_gemini_md
}

setup_codex() {
    local target="$REPO_ROOT/.codex/skills"
    mkdir -p "$REPO_ROOT/.codex"

    if [ -L "$target" ]; then
        rm "$target"
    elif [ -d "$target" ]; then
        mv "$target" "$REPO_ROOT/.codex/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$target"
    echo -e "${GREEN}  ✓ .codex/skills -> skills/${NC}"
}

setup_copilot() {
    local target="$REPO_ROOT/.github/skills"
    mkdir -p "$REPO_ROOT/.github"

    if [ -L "$target" ]; then
        rm "$target"
    elif [ -d "$target" ]; then
        mv "$target" "$REPO_ROOT/.github/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$target"
    echo -e "${GREEN}  ✓ .github/skills -> skills/ (GitHub Copilot)${NC}"

    setup_vscode_settings
}

setup_vscode_settings() {
    local settings_file="$REPO_ROOT/.vscode/settings.json"
    local vscode_dir="$REPO_ROOT/.vscode"

    mkdir -p "$vscode_dir"

    if [ ! -f "$settings_file" ]; then
        echo "  Creating new settings.json..."
        cat > "$settings_file" << 'EOF'
{
    "chat.useAgentsMdFile": true,
    "github.copilot.chat.commitMessage.enabled": true,
    "github.copilot.chat.commitMessageGeneration.instructions": "Generate commit messages following Conventional Commits specification validated by lefthook:\n\nFormat: <type>[optional scope]: <description>\n\nAllowed types (validated by lefthook):\n- feat: New feature\n- fix: Bug fix\n- docs: Documentation changes\n- style: Code style changes (formatting, etc.)\n- refactor: Code refactoring\n- test: Adding or updating tests\n- chore: Maintenance tasks, build process, dependencies\n- perf: Performance improvement\n- ci: CI/CD configuration changes\n- build: Build system changes\n- revert: Revert previous commit\n- merge: Merge commits\n\nCommon scopes: agents, webapi, web-executor, web, interval, domain, infrastructure, migration, analytics\n\nRules enforced by lefthook:\n- Use imperative mood (add, fix, not added, fixed)\n- Format: <type>[optional scope]: <description>\n- Type MUST be one of the allowed types above\n- Scope is optional and should be in parentheses if used\n- Colon and space after type/scope is required\n- Description is required after the colon\n- Don't end description with period\n- Add BREAKING CHANGE: in footer for breaking changes\n- Reference issues with Fixes #123\n\nExamples:\n- feat: add new feature\n- fix(auth): resolve login issue\n- docs: update README"
}
EOF
        echo -e "${GREEN}  ✓ Created .vscode/settings.json with Copilot settings${NC}"
    else
        echo -e "${YELLOW}  ℹ settings.json already exists, skipping update${NC}"
    fi
}

create_claude_md() {
    local claude_md="$REPO_ROOT/.claude/Claude.md"
    
    if [ ! -f "$claude_md" ]; then
        cat > "$claude_md" << 'EOF'
# Claude Code Instructions

You are Claude, an AI coding assistant working with the Guardian Agent (GA) system.

## Core Principles
- Follow the AGENTS.MD guidelines in this repository
- Use the skills/ directory for specialized coding tasks
- Maintain code quality and follow established patterns
- Always validate changes before committing

## Available Skills
Access specialized skills through the skills/ directory symlinked at .claude/skills/

## Workflow
1. Analyze the request and AGENTS.MD requirements
2. Use appropriate skills from the skills/ directory
3. Implement changes following coding standards
4. Run validation checks
5. Commit with proper conventional commit messages

## Key Commands
- Use skills for complex tasks (linting, testing, etc.)
- Follow lefthook pre-commit validation
- Reference REVIEW.md for code review standards
EOF
        echo -e "${GREEN}  ✓ Created .claude/Claude.md${NC}"
    else
        echo -e "${YELLOW}  ℹ .claude/Claude.md already exists${NC}"
    fi
}

create_gemini_md() {
    local gemini_md="$REPO_ROOT/.gemini/gemini.md"
    
    if [ ! -f "$gemini_md" ]; then
        cat > "$gemini_md" << 'EOF'
# Gemini CLI Instructions

You are Gemini, an AI coding assistant working with the Guardian Agent (GA) system.

## Core Principles
- Follow the AGENTS.MD guidelines in this repository
- Use the skills/ directory for specialized coding tasks
- Maintain code quality and follow established patterns
- Always validate changes before committing

## Available Skills
Access specialized skills through the skills/ directory symlinked at .gemini/skills/

## Workflow
1. Analyze the request and AGENTS.MD requirements
2. Use appropriate skills from the skills/ directory
3. Implement changes following coding standards
4. Run validation checks
5. Commit with proper conventional commit messages

## Key Commands
- Use skills for complex tasks (linting, testing, etc.)
- Follow lefthook pre-commit validation
- Reference REVIEW.md for code review standards
EOF
        echo -e "${GREEN}  ✓ Created .gemini/gemini.md${NC}"
    else
        echo -e "${YELLOW}  ℹ .gemini/gemini.md already exists${NC}"
    fi
}

copy_agents_md() {
    local target_name="$1"
    local count=0

    # Optimized find to ignore heavy directories (bin, obj, node_modules, etc.)
    # This is much faster on Windows/WSL
    while IFS= read -r agents_file; do
        if [ -n "$agents_file" ]; then
            local agents_dir
            agents_dir=$(dirname "$agents_file")
            cp "$agents_file" "$agents_dir/$target_name"
            count=$((count + 1))
        fi
    done < <(find "$REPO_ROOT" \
        -type d \( -name "node_modules" -o -name ".git" -o -name "bin" -o -name "obj" -o -name ".next" -o -name "dist" \) -prune \
        -o \( -name "AGENTS.md" -o -name "AGENTS.MD" \) -print 2>/dev/null)

    echo -e "${GREEN}  ✓ Copied $count instruction files -> $target_name${NC}"
}

# =============================================================================
# MAIN
# =============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            SETUP_CLAUDE=true; SETUP_CURSOR=true; SETUP_OPENCODE=true; SETUP_GEMINI=true; SETUP_CODEX=true; SETUP_COPILOT=true; shift ;;
        --claude) SETUP_CLAUDE=true; shift ;;
        --cursor) SETUP_CURSOR=true; shift ;;
        --opencode) SETUP_OPENCODE=true; shift ;;
        --gemini) SETUP_GEMINI=true; shift ;;
        --codex) SETUP_CODEX=true; shift ;;
        --copilot) SETUP_COPILOT=true; shift ;;
        --help|-h) show_help; exit 0 ;;
        *) echo -e "${RED}Unknown option: $1${NC}"; exit 1 ;;
    esac
done

echo -e "${BLUE}🤖 GA AI Skills Setup${NC}"
echo "========================="
echo ""

SKILL_COUNT=$(find "$SKILLS_SOURCE" -maxdepth 2 -name "SKILL.md" | wc -l | tr -d ' ')

if [ "$SKILL_COUNT" -eq 0 ]; then
    echo -e "${RED}No skills found in $SKILLS_SOURCE${NC}"
    exit 1
fi

echo -e "${BLUE}Found $SKILL_COUNT skills to configure${NC}"
echo ""

if [ "$SETUP_CLAUDE" = false ] && [ "$SETUP_CURSOR" = false ] && [ "$SETUP_OPENCODE" = false ] && [ "$SETUP_GEMINI" = false ] && [ "$SETUP_CODEX" = false ] && [ "$SETUP_COPILOT" = false ]; then
    show_menu
fi

STEP=1
TOTAL=0
[ "$SETUP_CLAUDE" = true ] && TOTAL=$((TOTAL + 1))
[ "$SETUP_CURSOR" = true ] && TOTAL=$((TOTAL + 1))
[ "$SETUP_OPENCODE" = true ] && TOTAL=$((TOTAL + 1))
[ "$SETUP_GEMINI" = true ] && TOTAL=$((TOTAL + 1))
[ "$SETUP_CODEX" = true ] && TOTAL=$((TOTAL + 1))
[ "$SETUP_COPILOT" = true ] && TOTAL=$((TOTAL + 1))

if [ "$SETUP_CLAUDE" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up Claude Code...${NC}"
    setup_claude; STEP=$((STEP + 1))
fi

if [ "$SETUP_CURSOR" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up Cursor...${NC}"
    setup_cursor; STEP=$((STEP + 1))
fi

if [ "$SETUP_OPENCODE" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up OpenCode...${NC}"
    setup_opencode; STEP=$((STEP + 1))
fi

if [ "$SETUP_GEMINI" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up Gemini CLI...${NC}"
    setup_gemini; STEP=$((STEP + 1))
fi

if [ "$SETUP_CODEX" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up Codex (OpenAI)...${NC}"
    setup_codex; STEP=$((STEP + 1))
fi

if [ "$SETUP_COPILOT" = true ]; then
    echo -e "${YELLOW}[$STEP/$TOTAL] Setting up GitHub Copilot...${NC}"
    setup_copilot
fi

echo ""
echo -e "${GREEN}✅ Successfully configured $SKILL_COUNT AI skills!${NC}"
