import { render, screen, renderHook, waitFor } from '@testing-library/react';
import { HealthDashboard, HealthStatusCard } from './HealthDashboard';
import { useHealth } from './useHealth';

// ----- Types mirroring the backend API -------------------------------------

interface HealthStatus {
	daemon_ok: boolean;
	db_ok: boolean;
	repo_count: number;
	last_check: string;
}

// ----- Test fixtures -------------------------------------------------------

function healthyStatus(overrides?: Partial<HealthStatus>): HealthStatus {
	return {
		daemon_ok: true,
		db_ok: true,
		repo_count: 5,
		last_check: '2025-07-03T10:00:00Z',
		...overrides,
	};
}

function mockFetchResponse(status: HealthStatus) {
	globalThis.fetch = vi.fn().mockResolvedValue({
		ok: true,
		json: () => Promise.resolve(status),
	});
}

// ----- Setup / teardown ----------------------------------------------------

beforeEach(() => {
	globalThis.fetch = vi.fn();
});

afterEach(() => {
	vi.restoreAllMocks();
});

// ---------------------------------------------------------------------------
// useHealth hook
// ---------------------------------------------------------------------------

describe('useHealth', () => {
	it('fetches health status from /api/health', async () => {
		const status = healthyStatus();
		mockFetchResponse(status);

		const { result } = renderHook(() => useHealth());

		await waitFor(() => {
			expect(result.current.data).toEqual(status);
		});
		expect(fetch).toHaveBeenCalledWith('/api/health', expect.anything());
	});

	it('returns loading state while fetching', () => {
		// Never resolve the fetch so loading stays true
		globalThis.fetch = vi.fn().mockReturnValue(new Promise(() => {}));

		const { result } = renderHook(() => useHealth());

		expect(result.current.loading).toBe(true);
		expect(result.current.data).toBeNull();
		expect(result.current.error).toBeNull();
	});

	it('handles fetch error', async () => {
		globalThis.fetch = vi.fn().mockRejectedValue(new Error('Network error'));

		const { result } = renderHook(() => useHealth());

		await waitFor(() => {
			expect(result.current.error).toBeDefined();
		});
		expect(result.current.data).toBeNull();
		expect(result.current.loading).toBe(false);
	});
});

// ---------------------------------------------------------------------------
// HealthDashboard component
// ---------------------------------------------------------------------------

describe('HealthDashboard', () => {
	it('renders "Healthy" when all checks pass', async () => {
		mockFetchResponse(healthyStatus());

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/healthy/i)).toBeInTheDocument();
		});
	});

	it('renders "Unhealthy" when daemon is down', async () => {
		mockFetchResponse(healthyStatus({ daemon_ok: false }));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
		});
	});

	it('renders "Unhealthy" when database is down', async () => {
		mockFetchResponse(healthyStatus({ db_ok: false }));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
		});
	});

	it('renders repo count badge', async () => {
		mockFetchResponse(healthyStatus({ repo_count: 3 }));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/3/)).toBeInTheDocument();
		});
	});

	it('renders last check timestamp', async () => {
		mockFetchResponse(healthyStatus());

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/2025|jul|03/i)).toBeInTheDocument();
		});
	});

	it('shows loading indicator while fetching', () => {
		globalThis.fetch = vi.fn().mockReturnValue(new Promise(() => {}));

		render(<HealthDashboard />);

		expect(screen.getByLabelText(/loading health status/i)).toBeInTheDocument();
	});

	it('shows error message on fetch failure', async () => {
		globalThis.fetch = vi.fn().mockRejectedValue(new Error('Failed'));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/error|failed/i)).toBeInTheDocument();
		});
	});

	it('renders "Unhealthy" when both daemon and db are down', async () => {
		mockFetchResponse(healthyStatus({ daemon_ok: false, db_ok: false }));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
		});
	});

	it('renders correct card states in mixed scenario (daemon down, db up)', async () => {
		mockFetchResponse(healthyStatus({ daemon_ok: false, db_ok: true }));

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/unhealthy/i)).toBeInTheDocument();
		});

		// Daemon card should show error icon
		const daemonCard = screen.getByText(/daemon/i).closest('div')!;
		expect(daemonCard.querySelector('[data-status="error"]')).toBeInTheDocument();

		// Database card should show ok icon
		const dbCard = screen.getByText(/database/i).closest('div')!;
		expect(dbCard.querySelector('[data-status="ok"]')).toBeInTheDocument();
	});
});

// ---------------------------------------------------------------------------
// HealthStatusCard sub-component
// ---------------------------------------------------------------------------

describe('HealthStatusCard', () => {
	it('renders check name', () => {
		render(<HealthStatusCard name="Daemon" ok={true} />);

		expect(screen.getByText(/daemon/i)).toBeInTheDocument();
	});

	it('shows a check icon for OK status', () => {
		render(<HealthStatusCard name="Database" ok={true} />);

		// Look for a green/check status indicator
		const card = screen.getByText(/database/i).closest('div');
		expect(card?.querySelector('.ok-icon, [data-status="ok"], .check-icon')).toBeInTheDocument();
	});

	it('shows a cross icon for down status', () => {
		render(<HealthStatusCard name="Database" ok={false} />);

		const card = screen.getByText(/database/i).closest('div');
		expect(card?.querySelector('.error-icon, [data-status="error"], .cross-icon')).toBeInTheDocument();
	});

	it('renders check name and both daemon/db cards in dashboard', async () => {
		mockFetchResponse(healthyStatus());

		render(<HealthDashboard />);

		await waitFor(() => {
			expect(screen.getByText(/daemon/i)).toBeInTheDocument();
		});
		expect(screen.getByText(/database/i)).toBeInTheDocument();
	});
});
