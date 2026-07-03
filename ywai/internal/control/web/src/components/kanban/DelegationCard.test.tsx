import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { DelegationCard } from './DelegationCard';
import type { Delegation } from '../../api/types';

// Mock the kanban store — DelegationCard reads activities and calls
// fetchActivities/resolveActivity/moveDelegation from it.
vi.mock('../../stores/kanbanStore', () => ({
	useKanbanStore: vi.fn((selector?: (s: Record<string, unknown>) => unknown) => {
		const state = {
			activities: {} as Record<string, unknown>,
			fetchActivities: vi.fn(),
			resolveActivity: vi.fn(),
			moveDelegation: vi.fn(),
		};
		return selector ? selector(state) : state;
	}),
}));

const delegation: Delegation = {
	id: 'deleg-001',
	session_id: 'session-abc',
	agent: 'architect',
	task_summary: 'Design auth module',
	status: 'running',
	column: 'in_progress',
	dependencies: [],
	created_at: '2025-01-01T00:00:00Z',
	handoff:
		'Architecture plan: implement OAuth2 with refresh tokens. ' +
		'Use PKCE flow for SPA clients and issue short-lived access tokens (15 min). ' +
		'Store refresh tokens in httpOnly secure cookies.',
	handoff_preview:
		'Architecture plan: implement OAuth2 with refresh tokens. Use PKCE flow…',
};

describe('DelegationCard', () => {
	beforeEach(() => cleanup());

	it('opens DelegationDetailModal when the expand/details button is clicked', async () => {
		const user = userEvent.setup();
		render(<DelegationCard delegation={delegation} />);

		// Find the accessible button that opens the detail modal.
		// The card is compact (not expanded), so this tests the
		// expand-from-compact behavior.
		const expandBtn = screen.getByRole('button', {
			name: /view details|details|expand|open detail/i,
		});
		await user.click(expandBtn);

		// DelegationDetailModal renders the full handoff inside a portal.
		expect(screen.getByText('Handoff / Plan')).toBeInTheDocument();
		expect(screen.getByText(delegation.handoff!)).toBeInTheDocument();
	});
});
