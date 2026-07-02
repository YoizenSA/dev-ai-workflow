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
          label: 'Guías',
          items: [
            { label: 'Visión general', slug: 'guides' },
            { label: 'Implementar Feature', slug: 'guides/feature' },
            { label: 'Arreglar Bug', slug: 'guides/bugfix' },
            { label: 'Code Review', slug: 'guides/review' },
            { label: 'Migración', slug: 'guides/migration' },
            { label: 'Testing', slug: 'guides/testing' },
            { label: 'CI/CD', slug: 'guides/cicd' },
            { label: 'Refactoring', slug: 'guides/refactoring' },
            {
              label: 'Personalizar',
              items: [
                { label: 'System Prompts', slug: 'guides/system-prompts' },
                { label: 'Escribir Skills', slug: 'guides/writing-skills' },
                { label: 'Custom Agents', slug: 'guides/custom-agents' },
              ],
            },
          ],
        },
        {
          label: 'Agentes',
          items: [
            { label: 'Visión general', slug: 'agents' },
            { label: 'Orchestrator', slug: 'agents/orchestrator' },
            { label: 'Architect', slug: 'agents/architect' },
            { label: 'Dev', slug: 'agents/dev' },
            { label: 'QA', slug: 'agents/qa' },
            { label: 'Reviewer', slug: 'agents/reviewer' },
            { label: 'DevOps', slug: 'agents/devops' },
            { label: 'Ask', slug: 'agents/ask' },
            { label: 'Finder', slug: 'agents/finder' },
            { label: 'Planning', slug: 'agents/planning' },
            { label: 'Memory', slug: 'agents/memory' },
            {
              label: 'Social Refactor',
              items: [
                { label: 'Migration Orchestrator', slug: 'agents/migration-orchestrator' },
                { label: 'Migration Planner', slug: 'agents/migration-planner' },
                { label: 'Migration Scope', slug: 'agents/migration-scope' },
                { label: 'Migration Validator', slug: 'agents/migration-validator' },
                { label: 'Validator Focused', slug: 'agents/migration-validator-focused' },
              ],
            },
            {
              label: 'QA Automation',
              items: [
                { label: 'QA Orchestrator', slug: 'agents/qa-orchestrator' },
                { label: 'QA Analyst', slug: 'agents/qa-analyst' },
                { label: 'QA Dev', slug: 'agents/qa-dev' },
                { label: 'QA Finder', slug: 'agents/qa-finder' },
                { label: 'QA Reviewer', slug: 'agents/qa-reviewer' },
                { label: 'QA Ask', slug: 'agents/qa-ask' },
              ],
            },
          ],
        },
        {
          label: 'Workflows',
          items: [
            { label: 'Visión general', slug: 'workflows' },
            { label: 'Workflow Studio', slug: 'workflows/studio' },
            { label: 'Commands', slug: 'workflows/commands' },
            { label: 'Agent Groups', slug: 'workflows/groups' },
          ],
        },
        {
          label: 'Herramientas',
          items: [
            { label: 'Kanban Board', slug: 'kanban' },
            { label: 'Skills', slug: 'skills' },
            { label: 'Skills Reference', slug: 'skills/reference' },
            { label: 'Settings UI', slug: 'settings' },
          ],
        },
        {
          label: 'Configuración',
          items: [
            { label: 'Configuración', slug: 'configuration' },
          ],
        },
      ],
    }),
  ],
});
