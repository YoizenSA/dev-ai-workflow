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
            { label: 'Agentes', slug: 'agents' },
          ],
        },
        {
          label: 'Workflow Studio',
          items: [
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
