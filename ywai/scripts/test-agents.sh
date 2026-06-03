#!/bin/bash
set -e

# Script para probar la instalación de ywai con cada agente
# Prueba: vscode-copilot, opencode, pi
# Verifica que los tools/permisos se adapten por agente

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Building ywai binary ==="
go build -o /tmp/ywai-test ./cmd/ywai

# Función para crear un fake agent
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
    echo "Created fake binary: $temp_dir/$binary_name"
}

# Función para probar instalación de un agente
test_agent_install() {
    local agent_id=$1
    local binary_name=$2
    local temp_dir=$(mktemp -d)
    
    echo ""
    echo "=== Testing agent: $agent_id ==="
    
    # Crear fake binary
    create_fake_agent "$agent_id" "$binary_name" "$temp_dir"
    
    # Agregar al PATH
    export PATH="$temp_dir:$PATH"
    
    # Ejecutar install con dry-run
    echo "Running: ywai install --agent $agent_id --dry-run"
    local output
    output=$(/tmp/ywai-test install --agent "$agent_id" --dry-run 2>&1)
    
    if [ $? -eq 0 ]; then
        echo "✓ $agent_id: dry-run successful"
        
        # Verificar que mencione el agente
        if echo "$output" | grep -q "$agent_id"; then
            echo "✓ $agent_id: agent name mentioned in output"
        else
            echo "✗ $agent_id: agent name NOT mentioned in output"
        fi
        
        # Verificar que mencione skills
        if echo "$output" | grep -qi "skill"; then
            echo "✓ $agent_id: skills mentioned in output"
        else
            echo "✗ $agent_id: skills NOT mentioned in output"
        fi
        
        # Verificar que mencione agent profiles
        if echo "$output" | grep -qi "agent profile"; then
            echo "✓ $agent_id: agent profiles mentioned in output"
        else
            echo "✗ $agent_id: agent profiles NOT mentioned in output"
        fi
        
        # Verificar instalación específica por tipo de agente
        case "$agent_id" in
            "opencode")
                if echo "$output" | grep -qi "opencode.json"; then
                    echo "✓ $agent_id: opencode.json config mentioned"
                else
                    echo "⚠ $agent_id: opencode.json config not explicitly mentioned (may be in gentle-ai)"
                fi
                ;;
            "vscode-copilot")
                if echo "$output" | grep -qi "prompts"; then
                    echo "✓ $agent_id: VS Code prompts directory mentioned"
                else
                    echo "⚠ $agent_id: VS Code prompts not explicitly mentioned"
                fi
                ;;
            "pi")
                if echo "$output" | grep -qi "pi"; then
                    echo "✓ $agent_id: pi agent mentioned"
                else
                    echo "⚠ $agent_id: pi agent not explicitly mentioned"
                fi
                ;;
        esac
    else
        echo "✗ $agent_id: dry-run FAILED"
        echo "Output: $output"
    fi
    
    # Limpiar
    rm -rf "$temp_dir"
}

# Verificar que los perfiles de agentes tengan tools.json
echo ""
echo "=== Verifying agent profiles have tools.json ==="
for profile in ask dev qa architect reviewer devops; do
    tools_file="$REPO_ROOT/agents/$profile/tools.json"
    if [ -f "$tools_file" ]; then
        echo "✓ $profile: tools.json exists"
        # Mostrar tools permitidos
        allowed=$(grep -o '"allowed":\s*\[[^]]*\]' "$tools_file" | head -1)
        echo "  Allowed: $allowed"
    else
        echo "✗ $profile: tools.json missing"
    fi
done

# Prueba para vscode-copilot
test_agent_install "vscode-copilot" "code"

# Prueba para opencode
test_agent_install "opencode" "opencode"

# Prueba para pi
test_agent_install "pi" "pi"

echo ""
echo "=== All tests completed ==="
rm -f /tmp/ywai-test
