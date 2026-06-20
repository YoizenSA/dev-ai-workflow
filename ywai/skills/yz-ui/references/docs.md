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
| Directiva de modal | `assets/yz-modal.directive.ts` | A11y de diálogo: role/aria, focus-trap, scroll-lock, Escape, restaurar foco |
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
- **Modal / overlay** (glass dialog: scrim, card head/body/foot, animaciones — a11y en `yz-modal.directive.ts`)
- **Sidebar rail** (opcional: footer tools colapsados como icon-buttons contenidos, un paso por debajo del avatar)
- **Icon button** (botón circular solo-icono)
- **Spinner / loading inline**
- **Pill badges semánticos**
- **Sistema de toasts completo** (stack, animaciones enter/exit/progress, variantes semánticas, responsive, `prefers-reduced-motion`)

### Anti-patrones comunes

- **No deshabilitar controles durante la carga de datos.** Atar el `disabled` de un control —o el `disable()`/`enable()` de un reactive form— al flag de _loading_ hace que, al resolver la carga en cada navegación, la transición disabled→enabled (140–150ms) se reproduzca y los filtros/selects/botones parpadeen. Los controles de lectura quedan interactivos mientras carga (el loading se muestra en el área de contenido: spinner/skeleton, `aria-busy`); el `disabled` se reserva para estados de **acción** reales (guardar, validar, cancelar, `!canExport`). Ver patrón #7 del `SKILL.md`.
- **Nunca `transition: all`.** Listar las propiedades a animar explícitamente; `all` anima cambios no buscados (flips de disabled/tema, reflows de layout) y cuesta performance. Misma familia que el parpadeo de arriba.
- **`backdrop-filter` siempre con prefijo `-webkit-backdrop-filter`.** Sin él, el glass (patrón estrella del tema oscuro) no renderiza en Safari ni en ningún navegador de iOS — degrada a un panel plano.
- **Tema oscuro: declarar `color-scheme: dark`** en `:root`. Sin eso, los `<select>`, date/time pickers y scrollbars nativos se pintan con chrome claro del SO y se ven rotos sobre fondo oscuro.
- **Toda animación con keyframes lleva guard `@media (prefers-reduced-motion: reduce)`** (entradas, pulse, etc.), no solo los toasts. Los spinners funcionales pueden seguir girando (comunican estado de carga).
- **Lifts en hover con `@media (hover: hover)`.** En dispositivos táctiles el `:hover` queda "pegado" tras el tap; el guard evita que el botón se quede levantado.

### Cómo aplicar el skill a un proyecto

1. Verificar Angular en el último major estable (`ng version`) — actualizar si no
2. Detectar el enfoque de estilos (CSS puro con tokens vs Tailwind 4)
3. Proyecto nuevo: copiar los tokens del tema elegido (claro u oscuro glass) del `SKILL.md`
4. Proyecto existente: correr el **Visual Correction Checklist** del `SKILL.md`, corrigiendo primero a nivel tokens y después por componente
