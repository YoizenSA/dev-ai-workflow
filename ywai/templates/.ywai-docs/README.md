# `.ywai/` — Configuración de tu workflow AI

Esta carpeta es la **sala de control de ywai** en tu proyecto. Se generó
cuando corriste el instalador. Commiteá lo que quieras versionar (las guías)
y dejá el resto gitignored.

## Qué hay adentro

| Archivo / carpeta | Para qué sirve |
|:---|:---|
| `config.json` | Provider + modelo por defecto + overrides por fase |
| `sdd-models.json` | Catálogo canónico de modelos por fase (lo leen las skills SDD y `ga`) |
| `skill-registry.md` | Compact rules auto-generadas para cada skill instalada |
| `docs/` | **Guías para humanos sobre cómo usar todo** |

## Quick start

```bash
# Arrancar un cambio nuevo (exploración + propuesta)
/sdd-new feature-name

# Planificación completa (proposal + spec + design + tasks)
/sdd-ff feature-name

# Implementar las tareas
/sdd-apply

# Verificar que lo implementado coincida con specs y design
/sdd-verify

# Archivar el cambio cuando todo está verde
/sdd-archive
```

## Guías

- [Quickstart de SDD](docs/sdd-quickstart.md) — el flujo end-to-end.
- [Referencia de slash commands](docs/slash-commands.md) — cada `/sdd-*`.
- [Skills y auto-invoke](docs/skills.md) — cómo se resuelven las skills en los prompts.
- [Protocolo de persistencia Engram](docs/engram.md) — STEP A/B de `mem_search`, `mem_save`.
- [TDD en `sdd-apply`](docs/tdd.md) — modos Standard, Light y Strict.

## Regenerar el registry

Después de instalar o remover skills, regenerá las compact rules:

```bash
bash skills/skill-sync/assets/sync.sh --registry
```

Esto reescribe `.ywai/skill-registry.md` (y lo guarda en Engram si está disponible).

## Ayuda

- Repo del proyecto: <https://github.com/Yoizen/dev-ai-workflow>
- Issues: <https://github.com/Yoizen/dev-ai-workflow/issues>
