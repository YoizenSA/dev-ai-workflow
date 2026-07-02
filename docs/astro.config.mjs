import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

export default defineConfig({
  site: 'https://YoizenSA.github.io',
  base: '/dev-ai-workflow/',
  output: 'static',
  locales: {
    root: { label: 'Español', lang: 'es' },
  },
  integrations: [
    starlight({
      title: 'ywai — Documentación',
      sidebar: [
        { label: 'Inicio', slug: '' },
        { label: 'Primeros pasos', slug: 'getting-started' },
        { label: 'Comandos', slug: 'commands' },
        {
          label: 'Agentes',
          items: [
            { label: 'Visión general', slug: 'agents' },
            { label: 'Dev', slug: 'agents/dev' },
            { label: 'QA', slug: 'agents/qa' },
            { label: 'Architect', slug: 'agents/architect' },
            { label: 'Reviewer', slug: 'agents/reviewer' },
            { label: 'DevOps', slug: 'agents/devops' },
            { label: 'Ask', slug: 'agents/ask' },
          ],
        },
        {
          label: 'Herramientas',
          items: [
            { label: 'Kanban Board', slug: 'kanban' },
            { label: 'Skills', slug: 'skills' },
            { label: 'Workflow Studio', slug: 'workflows' },
          ],
        },
        {
          label: 'Configuración',
          items: [
            { label: 'Configuración', slug: 'configuration' },
          ],
        },
        { label: 'Guías', slug: 'guides' },
      ],
    }),
  ],
});
