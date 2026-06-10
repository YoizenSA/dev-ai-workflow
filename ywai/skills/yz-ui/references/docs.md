## Recursos del Skill yz-ui

El skill es autocontenido y agnóstico de proyecto: las normas del `SKILL.md` aplican a **todo frontend Yoizen**, existente o nuevo. No depende de ningún repo de referencia.

### Assets

| Asset | Ubicación | Uso |
|-------|-----------|-----|
| Logo Principal | `assets/logo.svg` | Header, branding |
| Logo Negativo | `assets/logo-negativo.svg` / `assets/logo-negative.svg` | Fondos oscuros |
| Logo con Slogan | `assets/logo-sec-slogan.svg` | Landing pages |
| Icono | `assets/icon.svg` | Favicon, avatares |
| Logo Footer | `assets/logo-footer.svg` | Optimizado footer |
| Logo Dorso | `assets/logo-dorso-maneas.svg` | Variante especial |
| Snippets CSS | `assets/css-snippets.css` | Implementaciones completas listas para copiar |
| Template Angular | `assets/component-template.ts` | Componente standalone con signals y OnPush |
| Schema de tema | `assets/tailwind-theme-schema.json` | Estructura de tokens para Tailwind |

### Contenido de `css-snippets.css`

- Gradientes de marca (fondo y texto) y grid pattern técnico
- Utilidades de color con CSS variables
- Componentes comunes: card, sidebar, input, botón primario
- Alertas semánticas (info/success/warning/error)
- Scrollbar personalizado (Webkit + Firefox)
- Animaciones: fade-in, pulse sutil
- Utilidades de layout: container, grid responsive, flex, truncate
- **Glass panel** (tema oscuro)
- **Icon button** (botón circular solo-icono)
- **Spinner / loading inline**
- **Pill badges semánticos**
- **Sistema de toasts completo** (stack, animaciones enter/exit/progress, variantes semánticas, responsive, `prefers-reduced-motion`)

### Cómo aplicar el skill a un proyecto

1. Verificar Angular en el último major estable (`ng version`) — actualizar si no
2. Detectar el enfoque de estilos (CSS puro con tokens vs Tailwind 4)
3. Proyecto nuevo: copiar los tokens del tema elegido (claro u oscuro glass) del `SKILL.md`
4. Proyecto existente: correr el **Visual Correction Checklist** del `SKILL.md`, corrigiendo primero a nivel tokens y después por componente
