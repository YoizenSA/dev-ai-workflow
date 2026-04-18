#!/usr/bin/env bash
# Setup AI Skills for yFlow development
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
#   ./setup.sh --global-only --project-type=nest --opencode --copilot

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if git rev-parse --show-toplevel >/dev/null 2>&1; then
    REPO_ROOT="$(git rev-parse --show-toplevel)"
else
    # Fallback to current directory or one level up based on where script is located
    if [[ "$SCRIPT_DIR" == *"skills"* ]]; then
        REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
    else
        REPO_ROOT="$(pwd)"
    fi
fi

resolve_skills_source() {
    local candidate
    for candidate in \
        "$REPO_ROOT/skills" \
        "$REPO_ROOT/ywai/skills" \
        "$SCRIPT_DIR" \
        "$SCRIPT_DIR/../skills"; do
        [[ -d "$candidate" ]] && { echo "$candidate"; return 0; }
    done
    echo "$REPO_ROOT/skills"
}

SKILLS_SOURCE="$(resolve_skills_source)"

# Define minimal colors when run standalone
RED='\033[0;31m' GREEN='\033[0;32m' YELLOW='\033[1;33m'
BLUE='\033[0;34m' CYAN='\033[0;36m' BOLD='\033[1m' NC='\033[0m'

# Selection flags
SETUP_CLAUDE=false
SETUP_CURSOR=false
SETUP_OPENCODE=false
SETUP_GEMINI=false
SETUP_CODEX=false
SETUP_COPILOT=false
GLOBAL_ONLY=false
PROJECT_TYPE="generic"
GLOBAL_AGENTS_CONFIGURED=false
SKIP_HOOKS=false

resolve_repo_doc_case_insensitive() {
    local expected_name="$1"
    local expected_lc
    expected_lc="$(printf '%s' "$expected_name" | tr '[:upper:]' '[:lower:]')"

    local candidate base
    for candidate in "$REPO_ROOT"/*; do
        [[ -f "$candidate" ]] || continue
        base="$(basename "$candidate")"
        if [[ "$(printf '%s' "$base" | tr '[:upper:]' '[:lower:]')" == "$expected_lc" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    echo "$REPO_ROOT/$expected_name"
}

# =============================================================================
# HELPER FUNCTIONS
# =============================================================================

show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Configure AI coding assistants for yFlow development."
    echo ""
    echo "Options:"
    echo "  --all        Configure all AI assistants"
    echo "  --claude     Configure Claude Code"
    echo "  --cursor     Configure Cursor"
    echo "  --opencode   Configure OpenCode"
    echo "  --gemini     Configure Gemini CLI"
    echo "  --codex      Configure Codex (OpenAI)"
    echo "  --copilot    Configure GitHub Copilot"
    echo "  --global-only Configure only global user-profile agents (no repo files)"
    echo "  --skip-hooks Skip hooks installation (opencode-command-hooks, biome, etc.)"
    echo "  --project-type=<type> Project type for global agent generation"
    echo "  --help       Show this help message"
    echo ""
    echo "If no options provided, runs in interactive mode."
}

normalize_project_type() {
    local project_type="${1:-$PROJECT_TYPE}"
    case "$project_type" in
        nest|nest-angular|nest-react|python|dotnet|qa-playwright|devops|generic) echo "$project_type" ;;
        *) echo "generic" ;;
    esac
}

types_json_for_global_agents() {
    local candidate
    for candidate in \
        "$REPO_ROOT/ywai/types/types.json" \
        "$REPO_ROOT/types/types.json" \
        "$SCRIPT_DIR/../types/types.json"; do
        [[ -f "$candidate" ]] && { echo "$candidate"; return 0; }
    done
    echo ""
}

global_agent_bundles_json() {
    local candidate
    for candidate in \
        "$REPO_ROOT/ywai/extensions/install-steps/global-agents/bundles.json" \
        "$REPO_ROOT/extensions/install-steps/global-agents/bundles.json" \
        "$SCRIPT_DIR/../extensions/install-steps/global-agents/bundles.json" \
        "$SCRIPT_DIR/../../extensions/install-steps/global-agents/bundles.json"; do
        [[ -f "$candidate" ]] && { echo "$candidate"; return 0; }
    done
    echo ""
}

default_bundle_for_agent() {
    local agent_name="$1"
    case "$agent_name" in
        sdd-orchestator)
            echo "sdd-init sdd-explore sdd-propose sdd-spec sdd-design sdd-tasks sdd-apply sdd-verify sdd-archive"
            ;;
        fe-engineer)
            echo "react-19 tailwind-4 typescript biome"
            ;;
        nest-engineer)
            echo "typescript biome"
            ;;
        dotnet-engineer)
            echo "dotnet"
            ;;
        qa-playwright)
            echo "playwright"
            ;;
        devops)
            echo "devops"
            ;;
        *)
            echo ""
            ;;
    esac
}

global_agent_skills_bundle() {
    local agent_name="$1"
    local project_type
    project_type="$(normalize_project_type "${2:-$PROJECT_TYPE}")"

    local bundles_json
    bundles_json="$(global_agent_bundles_json)"
    if [[ -f "$bundles_json" ]] && command -v python3 >/dev/null 2>&1; then
        local configured
        configured=$(python3 -c "
import json
try:
  data=json.load(open('$bundles_json'))
  defaults=data.get('defaults',{}).get('$agent_name',[])
  overrides=data.get('by_project_type',{}).get('$project_type',{}).get('$agent_name',[])
  values=overrides if isinstance(overrides,list) and overrides else defaults
  if isinstance(values,list):
    values=[str(v).strip() for v in values if str(v).strip()]
    print(' '.join(values))
except: pass
" 2>/dev/null)
        [[ -n "$configured" ]] && { echo "$configured"; return 0; }
    fi

    default_bundle_for_agent "$agent_name"
}

skill_auto_invoke_patterns() {
    local skill_name="$1"
    local skill_file="$SKILLS_SOURCE/$skill_name/SKILL.md"
    [[ -f "$skill_file" ]] || { echo ""; return 0; }
    command -v python3 >/dev/null 2>&1 || { echo ""; return 0; }

    python3 - "$skill_file" << 'PY' 2>/dev/null
import re
import sys
from pathlib import Path

path = Path(sys.argv[1])
text = path.read_text(encoding="utf-8", errors="ignore")
front = text

if text.startswith("---"):
    markers = list(re.finditer(r"^---\s*$", text, flags=re.M))
    if len(markers) >= 2:
        front = text[markers[0].end():markers[1].start()]
else:
    marker = re.search(r"^---\s*$", text, flags=re.M)
    if marker:
        front = text[:marker.start()]

lines = front.splitlines()
patterns = []

for i, line in enumerate(lines):
    if re.match(r"^\s*auto_invoke\s*:", line):
        value = line.split(":", 1)[1].strip()
        if value.startswith("[") and value.endswith("]"):
            inner = value[1:-1]
            items = [x.strip().strip('"\'') for x in inner.split(",") if x.strip()]
            patterns.extend(items)
            break
        if value:
            patterns.append(value.strip('"\''))
            break

        j = i + 1
        while j < len(lines):
            cur = lines[j]
            if re.match(r"^\s*-\s+", cur):
                patterns.append(re.sub(r"^\s*-\s+", "", cur).strip().strip('"\''))
                j += 1
                continue
            if re.match(r"^\s*$", cur):
                j += 1
                continue
            break
        break

clean = []
for p in patterns:
    p = str(p).strip()
    if p and p not in clean:
        clean.append(p)

print(" | ".join(clean[:3]))
PY
}

append_global_agent_skills_bundle() {
    local file_path="$1"
    local agent_name="$2"
    local project_type="$3"

    local bundle_skills
    bundle_skills="$(global_agent_skills_bundle "$agent_name" "$project_type")"
    [[ -n "$bundle_skills" ]] || return 0

    cat >> "$file_path" << 'EOF'

## Skills bundle (global)
EOF

    local skill_name
    for skill_name in $bundle_skills; do
        echo "- \`$skill_name\`" >> "$file_path"
    done

    cat >> "$file_path" << 'EOF'

## Skills invoke
EOF

    for skill_name in $bundle_skills; do
        local triggers
        triggers="$(skill_auto_invoke_patterns "$skill_name")"
        if [[ -n "$triggers" ]]; then
            echo "- Use \`$skill_name\` when tasks match: $triggers" >> "$file_path"
        else
            echo "- Use \`$skill_name\` when its domain is required." >> "$file_path"
        fi
    done
}

setup_global_opencode_skills_link() {
    local opencode_dir="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
    local opencode_skills="$opencode_dir/skills"

    mkdir -p "$opencode_dir"

    if [ -L "$opencode_skills" ]; then
        rm "$opencode_skills"
    elif [ -d "$opencode_skills" ]; then
        mv "$opencode_skills" "$opencode_dir/skills.backup.$(date +%s)"
    elif [ -e "$opencode_skills" ]; then
        mv "$opencode_skills" "$opencode_dir/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$opencode_skills"
    echo -e "${GREEN}  ✓ $opencode_dir/skills -> skills/ (OpenCode global)${NC}"
}

global_agent_names_for_type() {
    local project_type
    project_type="$(normalize_project_type "$1")"

    local types_json
    types_json="$(types_json_for_global_agents)"
    if [[ -f "$types_json" ]] && command -v python3 >/dev/null 2>&1; then
        local configured
        configured=$(python3 -c "
import json
try:
  data=json.load(open('$types_json'))
  values=data.get('types',{}).get('$project_type',{}).get('global_agents',[])
  if isinstance(values,list):
    values=[str(v).strip() for v in values if str(v).strip()]
    print(' '.join(values))
except: pass
" 2>/dev/null)
        [[ -n "$configured" ]] && { echo "$configured"; return 0; }
    fi

    case "$project_type" in
        nest)
            echo "sdd-orchestator nest-engineer devops"
            ;;
        nest-angular|nest-react)
            echo "sdd-orchestator fe-engineer devops"
            ;;
        dotnet)
            echo "sdd-orchestator dotnet-engineer devops"
            ;;
        qa-playwright)
            echo "sdd-orchestator qa-playwright devops"
            ;;
        devops|python|generic|*)
            echo "sdd-orchestator devops"
            ;;
    esac
}

resolve_global_agent_template_file() {
    local agent_name="$1"
    local project_type
    project_type="$(normalize_project_type "${2:-$PROJECT_TYPE}")"

    local candidate
    for candidate in \
        "$REPO_ROOT/ywai/extensions/install-steps/global-agents/templates/$project_type/$agent_name.md" \
        "$REPO_ROOT/ywai/extensions/install-steps/global-agents/templates/$agent_name.md" \
        "$REPO_ROOT/extensions/install-steps/global-agents/templates/$project_type/$agent_name.md" \
        "$REPO_ROOT/extensions/install-steps/global-agents/templates/$agent_name.md" \
        "$SCRIPT_DIR/../extensions/install-steps/global-agents/templates/$project_type/$agent_name.md" \
        "$SCRIPT_DIR/../extensions/install-steps/global-agents/templates/$agent_name.md" \
        "$SCRIPT_DIR/../../extensions/install-steps/global-agents/templates/$project_type/$agent_name.md" \
        "$SCRIPT_DIR/../../extensions/install-steps/global-agents/templates/$agent_name.md"; do
        if [[ -f "$candidate" ]]; then
            echo "$candidate"
            return 0
        fi
    done

    echo ""
}

resolve_template_file() {
    local template_name="$1"
    local candidate
    for candidate in \
        "$REPO_ROOT/ywai/templates/$template_name" \
        "$REPO_ROOT/templates/$template_name" \
        "$SCRIPT_DIR/../templates/$template_name"; do
        [[ -f "$candidate" ]] && { echo "$candidate"; return 0; }
    done
    echo ""
}

append_sdd_devops_guidance() {
    local file_path="$1"
    local agent_name="$2"

    cat >> "$file_path" << EOF

## How to use Skills (SDD + DevOps)

### SDD quick commands
- \/sdd:new <change-name>
- \/sdd:ff <change-name>
- \/sdd:apply
- \/sdd:verify
- \/sdd:archive

### DevOps trigger keywords
- pipeline
- azure pipelines
- helm
- docker
- devops
- kubernetes
- k8s
- deploy
- ci/cd

### Agent focus: ${agent_name}
EOF

    case "$agent_name" in
        sdd-orchestator)
            cat >> "$file_path" << 'EOF'
- Orchestrate SDD phases and keep implementation aligned with specs.
- Prefer `/sdd:new` and `/sdd:ff` for multi-file features.
EOF
            ;;
        fe-engineer)
            cat >> "$file_path" << 'EOF'
- Focus on frontend architecture, components, styling, and UX quality.
- Use SDD for cross-cutting UI changes and workflows.
EOF
            ;;
        nest-engineer)
            cat >> "$file_path" << 'EOF'
- Focus on NestJS backend architecture, modules, services, and APIs.
- Coordinate SDD artifacts for backend features before implementation.
EOF
            ;;
        dotnet-engineer)
            cat >> "$file_path" << 'EOF'
- Focus on .NET architecture, clean boundaries, and robust service design.
- Use SDD flow for feature planning and delivery control.
EOF
            ;;
        qa-playwright)
            cat >> "$file_path" << 'EOF'
- Focus on Playwright E2E coverage, flake reduction, and release confidence.
- Use SDD flow when test architecture or critical flows need explicit planning.
EOF
            ;;
        devops)
            cat >> "$file_path" << 'EOF'
- Focus on CI/CD, Docker, Helm, deployment, and environment contracts.
- Align infrastructure tasks with feature specs and delivery phases.
EOF
            ;;
    esac
}

write_global_agent_file() {
    local target_file="$1"
    local agent_name="$2"
    local project_type="$3"
    local mode="$4" # opencode | copilot_prompt | copilot_agent

    case "$mode" in
        copilot_prompt)
            cat > "$target_file" << EOF
---
name: ${agent_name}
description: Global ${agent_name} instructions for ${project_type} projects
applyTo: "**"
---

EOF
            ;;
        opencode)
            cat > "$target_file" << EOF
---
description: ${agent_name} global agent for ${project_type} projects
---

EOF
            ;;
        gemini)
            cat > "$target_file" << EOF
---
description: ${agent_name} global agent for ${project_type} projects
---

EOF
            ;;
        cursor)
            cat > "$target_file" << EOF
---
description: ${agent_name} global agent for ${project_type} projects
---

EOF
            ;;
        copilot_agent|*)
            cat > "$target_file" << EOF
---
name: ${agent_name}
description: Global ${agent_name} agent for ${project_type} projects
---

EOF
            ;;
    esac

    cat >> "$target_file" << EOF
# ${agent_name}

Project type scope: ${project_type}
EOF

    local agent_template_file
    agent_template_file="$(resolve_global_agent_template_file "$agent_name" "$project_type")"
    if [[ -f "$agent_template_file" ]]; then
        cat >> "$target_file" << EOF

## Base directives (from extensions)
Source: ${agent_template_file}
EOF
        cat "$agent_template_file" >> "$target_file"
    else
        cat >> "$target_file" << 'EOF'

## Base directives (from extensions)
Template not found for this agent. Use focused, minimal, and safe defaults.
EOF
        echo -e "${YELLOW}  ! Missing template for global agent '${agent_name}' (project type '${project_type}')${NC}"
    fi

    append_global_agent_skills_bundle "$target_file" "$agent_name" "$project_type"

    append_sdd_devops_guidance "$target_file" "$agent_name"

    if [[ "$agent_name" == "sdd-orchestator" ]]; then
        local orchestrator_tpl
        orchestrator_tpl="$(resolve_template_file "sdd-orchestrator.md")"
        if [[ -f "$orchestrator_tpl" ]]; then
            echo "" >> "$target_file"
            cat "$orchestrator_tpl" >> "$target_file"
        fi
    fi
}

setup_global_profile_agents() {
    [[ "$GLOBAL_AGENTS_CONFIGURED" == "true" ]] && return 0

    local project_type
    project_type="$(normalize_project_type "$PROJECT_TYPE")"

    # Prefer the Go generator when available: single source of truth shared
    # with the wizard and Windows path. Falls back to the bash implementation
    # below if the binary is missing or the call fails.
    if command -v ywai >/dev/null 2>&1; then
        if ywai --update-global-agents --type="$project_type" --silent; then
            echo -e "${GREEN}  ✓ Global agents configured via ywai (type '$project_type')${NC}"
            GLOBAL_AGENTS_CONFIGURED=true
            return 0
        fi
        echo -e "${YELLOW}  ! ywai delegation failed, using bash fallback${NC}"
    fi

    local agent_names
    agent_names="$(global_agent_names_for_type "$project_type")"

    local opencode_dir="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
    local opencode_agents_dir="$opencode_dir/agent"
    local opencode_agents_alt_dir="$opencode_dir/agents"
    local copilot_agents_dir="$HOME/.copilot/agents"
    local gemini_agents_dir="$HOME/.gemini/agents"
    local cursor_agents_dir="$HOME/.cursor/agents"

    local vscode_user_dir
    if [[ "$(uname)" == "Darwin" ]]; then
      vscode_user_dir="$HOME/Library/Application Support/Code/User"
    else
      vscode_user_dir="${XDG_CONFIG_HOME:-$HOME/.config}/Code/User"
    fi
    local vscode_prompts_dir="$vscode_user_dir/prompts"

    rm -rf "$opencode_agents_dir" "$opencode_agents_alt_dir" "$copilot_agents_dir" "$vscode_prompts_dir" "$gemini_agents_dir" "$cursor_agents_dir"
    local legacy_prompt="$vscode_prompts_dir/enterprise-persona.instructions.md"

    mkdir -p "$opencode_agents_dir" "$opencode_agents_alt_dir" "$copilot_agents_dir" "$vscode_prompts_dir" "$gemini_agents_dir" "$cursor_agents_dir"

    local first_prompt=""
    local agent_name
    for agent_name in $agent_names; do
        write_global_agent_file "$opencode_agents_dir/${agent_name}.md" "$agent_name" "$project_type" "opencode"
        write_global_agent_file "$opencode_agents_alt_dir/${agent_name}.md" "$agent_name" "$project_type" "opencode"
        write_global_agent_file "$copilot_agents_dir/${agent_name}.md" "$agent_name" "$project_type" "copilot_agent"
        write_global_agent_file "$gemini_agents_dir/${agent_name}.md" "$agent_name" "$project_type" "gemini"
        write_global_agent_file "$cursor_agents_dir/${agent_name}.md" "$agent_name" "$project_type" "cursor"
        write_global_agent_file "$vscode_prompts_dir/${agent_name}.instructions.md" "$agent_name" "$project_type" "copilot_prompt"
        [[ -z "$first_prompt" ]] && first_prompt="$vscode_prompts_dir/${agent_name}.instructions.md"
    done

    if [[ -n "$first_prompt" && -f "$first_prompt" ]]; then
        cp "$first_prompt" "$legacy_prompt"
    fi

    echo -e "${GREEN}  ✓ Configured global agents for type '$project_type': $agent_names${NC}"
    GLOBAL_AGENTS_CONFIGURED=true
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

    # Note: CLAUDE.md is not copied to repo (it's in .gitignore)
    # Each developer generates it locally if needed
    
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

    # Note: CURSOR.md and .cursorrules are not copied to repo (they're in .gitignore)
    # Each developer generates them locally if needed
}

setup_opencode() {
    if [[ "$GLOBAL_ONLY" == "true" ]]; then
        setup_global_opencode_skills_link
        setup_global_profile_agents
        return 0
    fi

    local opencode_dir="${XDG_CONFIG_HOME:-$HOME/.config}/opencode"
    local opencode_skills="$opencode_dir/skills"

    mkdir -p "$opencode_dir/commands"

    if [ -L "$opencode_skills" ]; then
        rm "$opencode_skills"
    elif [ -d "$opencode_skills" ]; then
        mv "$opencode_skills" "$opencode_dir/skills.backup.$(date +%s)"
    elif [ -e "$opencode_skills" ]; then
        mv "$opencode_skills" "$opencode_dir/skills.backup.$(date +%s)"
    fi

    ln -s "$SKILLS_SOURCE" "$opencode_skills"
    echo -e "${GREEN}  ✓ $opencode_dir/skills -> skills/ (OpenCode)${NC}"

    if [ -f "$REPO_ROOT/ywai/types/generic/AGENTS.md" ]; then
        cp "$REPO_ROOT/ywai/types/generic/AGENTS.md" "$opencode_dir/AGENTS.md"
        
        # Append SDD Orchestrator template
        local orchestrator_tpl="$REPO_ROOT/ywai/templates/sdd-orchestrator.md"
        if [ -f "$orchestrator_tpl" ]; then
            echo "" >> "$opencode_dir/AGENTS.md"
            cat "$orchestrator_tpl" >> "$opencode_dir/AGENTS.md"
        fi
        
        # Append Engram template  
        local engram_tpl="$REPO_ROOT/ywai/templates/engram-protocol.md"
        if [ -f "$engram_tpl" ]; then
            echo "" >> "$opencode_dir/AGENTS.md"
            cat "$engram_tpl" >> "$opencode_dir/AGENTS.md"
        fi

        echo -e "${GREEN}  ✓ Built ywai/types/generic/AGENTS.md to $opencode_dir/AGENTS.md${NC}"
    fi

    if [ -f "$REPO_ROOT/ywai/config/opencode.json" ]; then
        cp "$REPO_ROOT/ywai/config/opencode.json" "$opencode_dir/opencode.json"
        echo -e "${GREEN}  ✓ Copied ywai/config/opencode.json to $opencode_dir${NC}"
    fi

    if [ -d "$REPO_ROOT/ywai/extensions/install-steps/slash-commands" ]; then
        cp -R "$REPO_ROOT/ywai/extensions/install-steps/slash-commands/"*.md "$opencode_dir/commands/"
        echo -e "${GREEN}  ✓ Copied commands from ywai/extensions/install-steps/slash-commands to $opencode_dir/commands${NC}"
    fi
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

    # Note: GEMINI.md is not copied to repo (it's in .gitignore)
    # Each developer generates it locally if needed
    
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
    if [[ "$GLOBAL_ONLY" == "true" ]]; then
        setup_global_profile_agents
        return 0
    fi

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
    
    # OS-dependent paths for VS Code User Profile
    local vscode_user_dir
    if [[ "$(uname)" == "Darwin" ]]; then
      vscode_user_dir="$HOME/Library/Application Support/Code/User"
    else
      vscode_user_dir="${XDG_CONFIG_HOME:-$HOME/.config}/Code/User"
    fi
    local vscode_prompts_dir="$vscode_user_dir/prompts"
    local vscode_mcp_file="$vscode_user_dir/mcp.json"

    mkdir -p "$vscode_dir"
    mkdir -p "$vscode_prompts_dir"

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

    echo -e "${BLUE}  Configuring VS Code Copilot custom instructions...${NC}"
    cat > "$vscode_prompts_dir/enterprise-persona.instructions.md" << 'EOF'
---
name: Enterprise Persona
description: Enterprise AI coding persona with SDD orchestration and Engram memory
applyTo: "**"
---

EOF
    if [ -f "$REPO_ROOT/ywai/types/generic/AGENTS.md" ]; then
        cat "$REPO_ROOT/ywai/types/generic/AGENTS.md" >> "$vscode_prompts_dir/enterprise-persona.instructions.md"
        
        # Append SDD Orchestrator template
        local orchestrator_tpl="$REPO_ROOT/ywai/templates/sdd-orchestrator.md"
        if [ -f "$orchestrator_tpl" ]; then
            echo "" >> "$vscode_prompts_dir/enterprise-persona.instructions.md"
            cat "$orchestrator_tpl" >> "$vscode_prompts_dir/enterprise-persona.instructions.md"
        fi
        
        # Append Engram template
        local engram_tpl="$REPO_ROOT/ywai/templates/engram-protocol.md"
        if [ -f "$engram_tpl" ]; then
            echo "" >> "$vscode_prompts_dir/enterprise-persona.instructions.md"
            cat "$engram_tpl" >> "$vscode_prompts_dir/enterprise-persona.instructions.md"
        fi
    fi
    echo -e "${GREEN}  ✓ Created enterprise-persona.instructions.md in User profile${NC}"

    echo -e "${BLUE}  Configuring VS Code Copilot MCP servers...${NC}"
    local VSCODE_MCP_OVERLAY='{
  "servers": {
    "engram": {
      "command": "engram",
      "args": ["mcp"]
    },
    "context7": {
      "type": "http",
      "url": "https://mcp.context7.com/mcp"
    }
  }
}'

    if command -v jq >/dev/null 2>&1 && [ -f "$vscode_mcp_file" ]; then
        if jq -s '.[0] * .[1]' "$vscode_mcp_file" <(echo "$VSCODE_MCP_OVERLAY") > "${vscode_mcp_file}.tmp"; then
            mv "${vscode_mcp_file}.tmp" "$vscode_mcp_file"
        else
            rm -f "${vscode_mcp_file}.tmp"
            echo "$VSCODE_MCP_OVERLAY" > "$vscode_mcp_file"
        fi
    else
        echo "$VSCODE_MCP_OVERLAY" > "$vscode_mcp_file"
    fi
    echo -e "${GREEN}  ✓ Updated mcp.json in User profile${NC}"
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
            
            # Copy base file
            cp "$agents_file" "$agents_dir/$target_name"
            
            # Append SDD Orchestrator template if exists
            local orchestrator_tpl="$REPO_ROOT/ywai/templates/sdd-orchestrator.md"
            if [ -f "$orchestrator_tpl" ]; then
                echo "" >> "$agents_dir/$target_name"
                cat "$orchestrator_tpl" >> "$agents_dir/$target_name"
            fi
            
            # Append Engram Protocol template if exists
            local engram_tpl="$REPO_ROOT/ywai/templates/engram-protocol.md"
            if [ -f "$engram_tpl" ]; then
                echo "" >> "$agents_dir/$target_name"
                cat "$engram_tpl" >> "$agents_dir/$target_name"
            fi

            count=$((count + 1))
        fi
    done < <(find "$REPO_ROOT" \
        -type d \( -name "node_modules" -o -name ".git" -o -name "bin" -o -name "obj" -o -name ".next" -o -name "dist" \) -prune \
        -o \( -iname "AGENTS.md" \) -print 2>/dev/null)

    echo -e "${GREEN}  ✓ Copied and built $count instruction files -> $target_name${NC}"
}

# =============================================================================
# MAIN
# =============================================================================

while [[ $# -gt 0 ]]; do
    case $1 in
        --all)
            # --all intentionally targets only the officially supported AI
            # assistants: OpenCode, Claude, and GitHub Copilot. Cursor, Gemini
            # and Codex are opt-in via their own --cursor / --gemini / --codex
            # flags (they are not wired into the happy path and will not be
            # written to the repo by default).
            SETUP_CLAUDE=true; SETUP_OPENCODE=true; SETUP_COPILOT=true; shift ;;
        --claude) SETUP_CLAUDE=true; shift ;;
        --cursor) SETUP_CURSOR=true; shift ;;
        --opencode) SETUP_OPENCODE=true; shift ;;
        --gemini) SETUP_GEMINI=true; shift ;;
        --codex) SETUP_CODEX=true; shift ;;
        --copilot) SETUP_COPILOT=true; shift ;;
        --skip-hooks) SKIP_HOOKS=true; shift ;;
        --global-only) GLOBAL_ONLY=true; shift ;;
        --project-type=*) PROJECT_TYPE="${1#*=}"; shift ;;
        --help|-h) show_help; exit 0 ;;
        *) echo -e "${RED}Unknown option: $1${NC}"; exit 1 ;;
    esac
done

if [[ "$GLOBAL_ONLY" == "true" ]] \
    && [[ "$SETUP_CLAUDE" == "false" && "$SETUP_CURSOR" == "false" && "$SETUP_OPENCODE" == "false" \
      && "$SETUP_GEMINI" == "false" && "$SETUP_CODEX" == "false" && "$SETUP_COPILOT" == "false" ]]; then
    SETUP_OPENCODE=true
    SETUP_COPILOT=true
fi

if [[ "$GLOBAL_ONLY" == "true" ]]; then
    SETUP_CLAUDE=false
    SETUP_CURSOR=false
    SETUP_GEMINI=false
    SETUP_CODEX=false
fi

echo -e "${BLUE}🤖 yFlow AI Skills Setup${NC}"
echo "========================="
echo ""

SKILL_COUNT=0
if [[ "$GLOBAL_ONLY" == "false" ]]; then
    SKILL_COUNT=$(find "$SKILLS_SOURCE" -maxdepth 2 -name "SKILL.md" | wc -l | tr -d ' ')

    if [ "$SKILL_COUNT" -eq 0 ]; then
        echo -e "${RED}No skills found in $SKILLS_SOURCE${NC}"
        exit 1
    fi

    echo -e "${BLUE}Found $SKILL_COUNT skills to configure${NC}"
    echo ""
else
    echo -e "${BLUE}Global-only mode enabled (user-profile agents).${NC}"
    echo ""
fi

if [[ "$GLOBAL_ONLY" == "false" ]] && [ "$SETUP_CLAUDE" = false ] && [ "$SETUP_CURSOR" = false ] && [ "$SETUP_OPENCODE" = false ] && [ "$SETUP_GEMINI" = false ] && [ "$SETUP_CODEX" = false ] && [ "$SETUP_COPILOT" = false ]; then
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
if [[ "$GLOBAL_ONLY" == "true" ]]; then
    echo -e "${GREEN}✅ Successfully configured global OpenCode/Copilot/Gemini/Cursor agents!${NC}"
else
    echo -e "${GREEN}✅ Successfully configured $SKILL_COUNT AI skills!${NC}"
fi
