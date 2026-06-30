import type { DriveStep } from 'driver.js'

// Tour steps for the Workflow Studio onboarding. Anchored to the real UI via
// data-tour attributes (set in WorkflowEditor) with CSS-class fallbacks so the
// tour still resolves if a data attribute is missing.
//
// Steps cover the primary loop: pick a workflow → design on the canvas → edit a
// node → run it → export it.
export function workflowTourSteps(): DriveStep[] {
	return [
		{
			element: '[data-tour="workflow-select"]',
			popover: {
				title: 'Pick or create a workflow',
				description:
					'Choose an existing workflow from here, or use New to start one from scratch. Your workflows live in ~/.ywai/workflows.',
				side: 'bottom',
				align: 'start',
			},
		},
		{
			element: '[data-tour="palette"]',
			popover: {
				title: 'Drag nodes onto the canvas',
				description:
					'Drag node types from the palette — SubAgent, Ask User, If/Else, Skill, MCP, Group — onto the canvas, or double-click to drop one.',
				side: 'right',
				align: 'start',
			},
		},
		{
			element: '[data-tour="canvas"]',
			popover: {
				title: 'Connect and arrange',
				description:
					'Connect nodes by dragging from an output handle to an input. Use Auto-layout to arrange the graph into a clean left-to-right flow.',
				side: 'left',
				align: 'center',
			},
		},
		{
			element: '[data-tour="node-detail"]',
			popover: {
				title: 'Edit the selected node',
				description:
					'Set the node\'s fields here: system prompt, task, tools, model. Double-click a node for the Monaco focus editor.',
				side: 'left',
				align: 'center',
			},
		},
		{
			element: '[data-tour="ai-refine-button"]',
			popover: {
				title: 'Edit with AI',
				description:
					'Describe a change in plain language and let the AI rewrite the workflow. The result loads into the editor as a single undo step.',
				side: 'bottom',
				align: 'end',
			},
		},
		{
			element: '[data-tour="run-button"]',
			popover: {
				title: 'Run it',
				description:
					'Exports the workflow and spawns its orchestrator via opencode. Output streams live to the panel below.',
				side: 'bottom',
				align: 'end',
			},
		},
		{
			element: '[data-tour="export"]',
			popover: {
				title: 'Export to your agent',
				description:
					'Preview the generated slash command + agents, then Apply to write them under ~/.config/opencode (or ~/.claude for Claude Code).',
				side: 'bottom',
				align: 'center',
			},
		},
	]
}

// Storage key tracking whether the user has seen the tour. Cleared when the
// user dismisses or completes the tour; also re-runnable from a help button.
export const TOUR_SEEN_KEY = 'ywai:workflow-tour-seen'
