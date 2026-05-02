## Referencias a Recursos Yoizen UI

Esta carpeta contiene links a la documentación y recursos reales del proyecto Yoizen UI.

### Archivos de Configuración

| Recurso | Ubicación | Descripción |
|---------|-----------|-------------|
| Tailwind Config | `services/yoizen-ui/tailwind.config.js` | Configuración completa de tema |
| Estilos Base | `services/yoizen-ui/src/index.css` | CSS variables y utilidades |
| PostCSS Config | `services/yoizen-ui/postcss.config.js` | Configuración PostCSS |
| Vite Config | `services/yoizen-ui/vite.config.ts` | Configuración bundler |

### Componentes de Referencia

| Componente | Ubicación | Patrones Demostrados |
|------------|-----------|---------------------|
| Layout | `services/yoizen-ui/src/components/layout/Layout.tsx` | Estructura sidebar + main |
| Sidebar | `services/yoizen-ui/src/components/layout/Sidebar.tsx` | Navegación, estados activos |
| Button | `services/yoizen-ui/src/components/common/Button.tsx` | Variantes, iconos, loading |
| Card | `services/yoizen-ui/src/components/common/Card.tsx` | Bordes, sombras, padding |
| Input | `services/yoizen-ui/src/components/common/Input.tsx` | Estados, validación, focus |
| Modal | `services/yoizen-ui/src/components/common/Modal.tsx` | Overlays, animaciones |
| HealthStatus | `services/yoizen-ui/src/components/dashboard/HealthStatus.tsx` | Badges, colores estado |
| StatsCards | `services/yoizen-ui/src/components/dashboard/StatsCards.tsx` | Grids, métricas |

### Assets Visuales

| Asset | Ubicación | Uso |
|-------|-----------|-----|
| Logo Principal | `services/yoizen-ui/public/logo.svg` | Header, branding |
| Logo Negativo | `services/yoizen-ui/public/logo-negativo.svg` | Dark backgrounds |
| Logo con Slogan | `services/yoizen-ui/public/logo-sec-slogan.svg` | Landing pages |
| Icono | `services/yoizen-ui/public/icon.svg` | Favicon, avatares |
| Logo Footer | `services/yoizen-ui/public/logo-footer.svg` | Optimizado footer |

### Hooks y Utilidades

| Utilidad | Ubicación | Propósito |
|----------|-----------|-----------|
| useJobs | `services/yoizen-ui/src/hooks/useJobs.ts` | Data fetching pattern |
| useAiAssist | `services/yoizen-ui/src/hooks/useAiAssist.ts` | AI drawer state |
| api.ts | `services/yoizen-ui/src/services/api.ts` | API client configuration |

### Tipos TypeScript

| Tipo | Ubicación | Entidades |
|------|-----------|-----------|
| Agent | `services/yoizen-ui/src/types/agent.ts` | Estructura agente |
| Job | `services/yoizen-ui/src/types/jobs.ts` | Tipos de jobs |
| Health | `services/yoizen-ui/src/types/health.ts` | Estados de salud |
| Webchat | `services/yoizen-ui/src/types/webchat.ts` | Mensajes conversación |

### Paginas Completas (Ejemplos de Layout)

| Pagina | Ubicacion | Features |
|--------|-----------|----------|
| Dashboard | `services/yoizen-ui/src/pages/Dashboard.tsx` | Grid layout, widgets |
| Agents | `services/yoizen-ui/src/pages/Agents.tsx` | Listas, filtros, acciones |
| Agent Detail | `services/yoizen-ui/src/pages/AgentDetail.tsx` | Formularios, tabs |
| Jobs | `services/yoizen-ui/src/pages/Jobs.tsx` | Tablas, paginación |
| Settings | `services/yoizen-ui/src/pages/Settings.tsx` | Paneles, configuración |
| Demo | `services/yoizen-ui/src/pages/Demo.tsx` | Component showcase |

---

**Nota**: Todas las rutas son relativas desde la raiz del workspace.
