# Code Review Rules — Python

## Tipado
- Type hints obligatorios en todas las funciones.
- No usar `Any` sin justificación explícita.
- Usar `pydantic` para validar datos externos.

## Calidad
- No `bare except:` — siempre especificar la excepción.
- No `print()` en código de producción — usar `logging`.
- No variables globales mutables.
- Máximo 3 niveles de anidamiento.

## Seguridad
- ❌ No hardcodear credenciales.
- Sanitizar toda entrada externa con `pydantic`.
- No `eval()` ni `exec()` con input del usuario.

## Dependencias
- Todas las deps en `pyproject.toml` con versiones pinneadas.
- No importar módulos no usados.

## Testing
- `pytest` para todos los tests.
- Mockear todas las llamadas externas.
- Coverage mínimo: 80% en lógica de negocio.
