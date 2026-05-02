# Code Review Rules

## Calidad general
- Código limpio y legible.
- Nombres descriptivos de variables y funciones.
- No código muerto — eliminar en vez de comentar.
- Máximo 3 niveles de anidamiento.

## Seguridad
- ❌ No hardcodear credenciales, tokens ni API keys.
- Todas las secrets vienen de variables de entorno.
- Validar y sanitizar todo input externo.
- Solo HTTPS para comunicación externa.

## Funciones y módulos
- Una función = una responsabilidad.
- Máximo 60 líneas por función.
- Máximo 400 líneas por archivo.

## Logging
- No `print`/`console.log` en producción.
- Usar el sistema de logging del proyecto.

## Testing
- Toda feature nueva necesita tests.
- Mockear dependencias externas.
- Mínimo 80% de coverage en lógica de negocio.
