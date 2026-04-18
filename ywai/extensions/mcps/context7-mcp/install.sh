#!/usr/bin/env bash
# Context7 MCP Installer - Enhanced YWAI Extension
# Uses npx ctx7 setup for proper MCP installation across configured providers

# Removed set -e to handle errors gracefully for missing providers

TARGET_DIR="${1:-.}"
PROVIDERS="${2:-opencode,claude}"

echo "Installing Context7 MCP for providers: $PROVIDERS"

# Convert comma-separated to array
IFS=',' read -ra PROVIDER_ARRAY <<< "$PROVIDERS"

# Install for each specified provider
SUCCESS_COUNT=0
TOTAL_COUNT=${#PROVIDER_ARRAY[@]}

for provider in "${PROVIDER_ARRAY[@]}"; do
    echo "Installing Context7 for $provider..."
    
    if npx ctx7 setup --$provider 2>/dev/null; then
        echo "✅ Context7 installed successfully for $provider"
        ((SUCCESS_COUNT++))
    else
        echo "⚠️  Could not install Context7 for $provider (provider may not be installed)"
    fi
done

echo ""
echo "Context7 MCP installation complete!"
echo "Successful: $SUCCESS_COUNT/$TOTAL_COUNT providers"

# Also create the example file for backward compatibility
TARGET_MCP_DIR="$TARGET_DIR/.ywai/mcp"
TARGET_FILE="$TARGET_MCP_DIR/context7-mcp.example.json"

mkdir -p "$TARGET_MCP_DIR"

cat > "$TARGET_FILE" << 'EOF'
{
  "context7": {
    "type": "remote",
    "url": "https://mcp.context7.com/mcp",
    "enabled": true
  }
}
EOF

echo ""
echo "Created example Context7 MCP config at $TARGET_FILE"
echo "Note: Real MCP servers have been configured globally using npx ctx7 setup"
