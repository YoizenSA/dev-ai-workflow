# Context7 MCP - Enhanced Installer

Instala Context7 MCP correctamente para OpenCode y Claude por defecto, con soporte opcional para otros providers, usando el método oficial `npx ctx7 setup`.

## ¿Qué hace?

- ✅ **Instalación Real MCP**: Usa `npx ctx7 setup` en lugar de solo crear archivos de ejemplo
- ✅ **Multi-Provider**: Instala para opencode y claude por defecto; cursor y gemini-cli son opcionales
- ✅ **OAuth Authentication**: Maneja generación de API keys automáticamente  
- ✅ **Rules & Skills**: Instala reglas y skills de documentación automáticamente
- ✅ **Error Handling**: Maneja gracefulmente providers que no están instalados

## Instalación

El installer es llamado automáticamente durante el setup de YWAI para proyectos que incluyen Context7 MCP.

### Uso Manual

```bash
# Instalar para providers por defecto (OpenCode + Claude)
./ywai/extensions/mcps/context7-mcp/install.sh

# Instalar para providers específicos
./ywai/extensions/mcps/context7-mcp/install.sh . "opencode,claude"

# Instalar en directorio específico
./ywai/extensions/mcps/context7-mcp/install.sh /path/to/project
```

## ¿Qué se instala?

### Configuración MCP
- **OpenCode**: `~/.config/opencode/opencode.json`
- **Claude**: `~/.claude.json` 
- **Cursor**: `~/.cursor/mcp.json`
- **Gemini CLI**: `~/.gemini/config.json`

### Rules
- **OpenCode**: `~/.config/opencode/rules/context7.md`
- **Claude**: `~/.claude/rules/context7.md`
- **Cursor**: `~/.cursor/rules/context7.mdc`
- **Gemini CLI**: `~/.gemini/rules/context7.md`

### Skills
- Skills de documentation lookup instalados en cada provider

## Uso

Después de la instalación, puedes usar Context7 en tus prompts:

```text
Create a Next.js middleware that checks for a valid JWT in cookies. use context7
```

Context7 automáticamente obtendrá documentación actualizada para las librerías mencionadas.

## Verificación

Verifica que Context7 esté instalado:

```bash
# OpenCode
opencode mcp list

# Claude  
cat ~/.claude.json | jq .mcp

# Cursor
cat ~/.cursor/mcp.json | jq .mcpServers
```

## Cambios vs Original

| Feature | Original | Enhanced |
|---------|----------|----------|
| MCP Registration | ❌ Solo crea archivos ejemplo | ✅ Registra MCP servers realmente |
| API Keys | ❌ Configuración manual requerida | ✅ Generación OAuth/API key automática |
| Multi-Provider | ❌ Solo OpenCode | ✅ Todos los providers principales |
| Rules & Skills | ❌ No incluidos | ✅ Instalados automáticamente |
| Error Handling | ❌ Básico | ✅ Manejo graceful de providers faltantes |

## Backward Compatibility

El installer todavía crea `.ywai/mcp/context7-mcp.example.json` para compatibilidad con sistemas existentes, pero la instalación real se hace globalmente usando `npx ctx7 setup`.
