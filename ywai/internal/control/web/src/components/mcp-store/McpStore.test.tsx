// McpStore.test.tsx — RED TDD specs for slice 7 of the "Real MCP Install"
// feature. These tests pin:
//
//   1. The McpServer interface grows `requiredEnv: EnvSpec[]` (and
//      `installCmd?: string`, used in slice 8 — not pinned here).
//   2. A CredentialsForm is rendered when a not-yet-installed server has
//      requiredEnv, and hidden otherwise.
//   3. The install POST body is `{id, target_agent, credentials}` with
//      `target_agent: "opencode"` (hardcoded in slice 7; the server
//      defaults to "opencode" if empty, but the contract here pins the
//      explicit value — see "Ambiguities" below).
//   4. The install endpoint replies 202 with `{install_id, status_url,
//      ws_channel, entry_id, target_agent}` and the component must poll
//      the status URL and show progress / success / failure accordingly.
//   5. The 400 / 404 / 409 / 422 / 500 error branches are surfaced in
//      the UI (aria-invalid for missing creds, "in progress" for 409,
//      generic error for 500).
//
// These tests are expected to FAIL against the current McpStore.tsx
// because the production `McpServer` interface does not declare
// `requiredEnv`, `handleInstall` does not accept credentials, and
// the component has no concept of a 202 + polling flow. The dev's
// slice-7 work makes them pass without modifying the tests.
//
// Ambiguities documented (read before changing a test):
//   - target_agent: brief pins `"opencode"` literally in the body.
//     If the dev decides to send an empty string and rely on the
//     server default, this test fails — change the assertion to
//     `body.target_agent === undefined || body.target_agent === "opencode"`.
//   - Polling interval: not pinned. Tests use real timers and the
//     default 1s-ish interval; long enough to avoid flakes on CI.
//   - "Progress" wording: brief says "Installing..." or "Probing...".
//     Tests accept either via `/installing|probing/i`.
//   - Success wording: brief is silent. Tests accept `/installed|success|done|✓/i`.
//   - Error wording for 422/500/409: brief is silent. Tests accept
//     broad patterns (`/failed|error|.../i`, `/in progress|already|conflict/i`).
//   - The form's *visibility model*: tests pin "always visible for
//     not-installed servers with requiredEnv" (not "expand-to-reveal").
//     If the dev goes with expand-to-reveal, the visibility tests
//     will fail and need to be adjusted to expand the card first.

import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { McpStore } from './McpStore';

// ----- Test fixtures ---------------------------------------------------------

type TestEnvSpec = {
	name: string;
	description: string;
	required: boolean;
	secret: boolean;
};

// A wider test-only type that includes the slice-7 fields. The production
// `McpServer` interface is private (not exported from McpStore.tsx) and
// does not yet declare `requiredEnv`, so we cannot import it. We declare
// the slice-7 shape here and cast at the fetch-mock boundary.
type TestMcpServer = {
	id: string;
	name: string;
	description: string;
	category: string;
	icon: string;
	installed: boolean;
	enabled: boolean;
	popular: boolean;
	type: string;
	source: 'custom' | 'registry';
	tools: string[];
	url: string;
	docs?: string;
	requiredEnv?: TestEnvSpec[];
};

function makeServer(overrides: Partial<TestMcpServer> = {}): TestMcpServer {
	return {
		id: 'github',
		name: 'GitHub',
		description: 'GitHub MCP server',
		category: 'Integration',
		icon: 'GH',
		installed: false,
		enabled: false,
		popular: true,
		type: 'local',
		source: 'registry',
		tools: ['create_issue', 'list_repos'],
		url: 'https://github.com',
		...overrides,
	};
}

// ----- fetch mock helpers ----------------------------------------------------

type MockResponse = {
	ok: boolean;
	status: number;
	json: () => Promise<unknown>;
};

function jsonResponse(status: number, body: unknown): MockResponse {
	return {
		ok: status >= 200 && status < 300,
		status,
		json: async () => body,
	};
}

type FetchMock = ReturnType<typeof vi.fn>;
function getFetchMock(): FetchMock {
	return globalThis.fetch as unknown as FetchMock;
}

/**
 * Configure the global fetch mock to return the given responses in order.
 * Each call pops the next response; once exhausted, the LAST response is
 * reused so polling tests don't hang.
 */
function mockFetchSequence(responses: MockResponse[]): void {
	if (responses.length === 0) {
		throw new Error('mockFetchSequence requires at least one response');
	}
	let i = 0;
	const fallback = responses[responses.length - 1];
	getFetchMock().mockImplementation(async () => {
		const r = i < responses.length ? responses[i] : fallback;
		if (i < responses.length) i++;
		return r;
	});
}

function mockFetchResponse(response: MockResponse): void {
	getFetchMock().mockResolvedValue(response);
}

/** Find the JSON-parsed body of the POST /api/mcp/install call. */
function getInstallPostBody(): Record<string, unknown> {
	const calls = getFetchMock().mock.calls;
	for (const [url, init] of calls) {
		if (
			typeof url === 'string' &&
			url.endsWith('/api/mcp/install') &&
			(init as RequestInit | undefined)?.method === 'POST'
		) {
			const body = (init as RequestInit).body;
			if (typeof body === 'string') {
				return JSON.parse(body) as Record<string, unknown>;
			}
		}
	}
	throw new Error('No POST /api/mcp/install call found in fetch mock');
}

// ----- Common setup ----------------------------------------------------------

beforeEach(() => {
	globalThis.fetch = vi.fn() as unknown as typeof globalThis.fetch;
});

afterEach(() => {
	cleanup();
	vi.restoreAllMocks();
});

async function renderWithCatalog(servers: TestMcpServer[]): Promise<void> {
	// The very first fetch is the catalog GET. Anything after that is
	// configured per-test.
	mockFetchResponse(jsonResponse(200, servers));
	render(<McpStore />);
	await waitFor(() => {
		expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
	});
}

function findInput(name: string): HTMLInputElement | null {
	return document.querySelector(`input[name="${name}"]`);
}

// ============================================================================
// 1. McpServer interface — requiredEnv
// ============================================================================

describe('McpServer interface — requiredEnv', () => {
	it('renders a server with requiredEnv without crashing (regression for #slice7-interface)', async () => {
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			requiredEnv: [
				{ name: 'GITHUB_TOKEN', description: 'GitHub PAT', required: true, secret: true },
			],
		});
		await renderWithCatalog([server]);
		// The card must render the server's name. This is the simplest possible
		// pin: if the production type drops `requiredEnv` from the wire format
		// OR the component crashes when it sees the field, this fails.
		expect(screen.getByText('GitHub')).toBeInTheDocument();
	});
});

// ============================================================================
// 2-5. CredentialsForm rendering
// ============================================================================

describe('CredentialsForm — rendering', () => {
	it('shows a password input with name=GITHUB_TOKEN when requiredEnv is present and not installed', async () => {
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			installed: false,
			requiredEnv: [
				{ name: 'GITHUB_TOKEN', description: 'GitHub PAT', required: true, secret: true },
			],
		});
		await renderWithCatalog([server]);

		const input = document.querySelector(
			'input[type="password"][name="GITHUB_TOKEN"]'
		) as HTMLInputElement | null;
		expect(input).not.toBeNull();
	});

	it('shows multiple inputs with correct types (secret → password, non-secret → text)', async () => {
		const server = makeServer({
			id: 'custom-mcp',
			name: 'Custom MCP',
			installed: false,
			requiredEnv: [
				{ name: 'API_TOKEN', description: 'API token', required: true, secret: true },
				{ name: 'API_URL', description: 'API endpoint', required: true, secret: false },
			],
		});
		await renderWithCatalog([server]);

		const secretInput = document.querySelector(
			'input[type="password"][name="API_TOKEN"]'
		);
		const textInput = document.querySelector(
			'input[type="text"][name="API_URL"]'
		);
		expect(secretInput).not.toBeNull();
		expect(textInput).not.toBeNull();
	});

	it('does not show the credentials form when the server is installed', async () => {
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			installed: true,
			requiredEnv: [
				{ name: 'GITHUB_TOKEN', description: 'GitHub PAT', required: true, secret: true },
			],
		});
		await renderWithCatalog([server]);

		const input = document.querySelector(
			'input[type="password"][name="GITHUB_TOKEN"]'
		);
		expect(input).toBeNull();
	});

	it('does not show the credentials form when requiredEnv is absent', async () => {
		const server = makeServer({
			id: 'playwright',
			name: 'Playwright',
			installed: false,
			// no requiredEnv
		});
		await renderWithCatalog([server]);

		// No form inputs at all in the MCP card surface.
		const allInputs = document.querySelectorAll('.mcp-store-card input');
		expect(allInputs.length).toBe(0);
	});
});

// ============================================================================
// 6-7. handleInstall — request body
// ============================================================================

describe('handleInstall — request body shape', () => {
	it('sends {id, target_agent, credentials} when the user fills the form', async () => {
		const user = userEvent.setup();
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			installed: false,
			requiredEnv: [
				{ name: 'GITHUB_TOKEN', description: 'GitHub PAT', required: true, secret: true },
			],
		});
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			// Long-running poll reply so the component does not hang.
			jsonResponse(200, { install_id: 'mcp-job-1', state: 'installing' }),
		]);

		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});

		const input = findInput('GITHUB_TOKEN');
		expect(input).not.toBeNull();
		await user.type(input as HTMLInputElement, 'ghp_test');

		const installButton = screen.getByRole('button', { name: /^install$/i });
		await user.click(installButton);

		const body = getInstallPostBody();
		expect(body).toEqual({
			id: 'github',
			target_agent: 'opencode',
			credentials: { GITHUB_TOKEN: 'ghp_test' },
		});
	});

	it('omits the credentials field (or sends {}) when requiredEnv is absent', async () => {
		const user = userEvent.setup();
		const server = makeServer({
			id: 'playwright',
			name: 'Playwright',
			installed: false,
			// no requiredEnv
		});
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-2',
				status_url: '/api/mcp/install/mcp-job-2',
				ws_channel: 'mcp-install',
				entry_id: 'playwright',
				target_agent: 'opencode',
			}),
			jsonResponse(200, { install_id: 'mcp-job-2', state: 'installing' }),
		]);

		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});

		const installButton = screen.getByRole('button', { name: /^install$/i });
		await user.click(installButton);

		const body = getInstallPostBody();
		// Either the field is absent or it's an empty object — both are valid
		// per the brief. Both are rejected by the server with 422 if requiredEnv
		// existed, so the test pins "no creds leak for non-requiredEnv servers".
		const creds = body.credentials;
		const credsIsEmpty =
			creds === undefined ||
			(typeof creds === 'object' &&
				creds !== null &&
				Object.keys(creds as object).length === 0);
		expect(credsIsEmpty).toBe(true);
		expect(body.id).toBe('playwright');
		expect(body.target_agent).toBe('opencode');
	});
});

// ============================================================================
// 8-12. handleInstall — 202 response + polling + terminal states
// ============================================================================

describe('handleInstall — 202 response and polling', () => {
	it('stores install_id and shows progress text after a 202 response', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			// Keep returning "installing" so the progress UI stays visible.
			jsonResponse(200, { install_id: 'mcp-job-1', state: 'installing' }),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		// Progress text must appear after the 202. We don't pin the exact
		// wording — "Installing", "Installing…", "installing github" all pass.
		await waitFor(
			() => {
				const txt = document.body.textContent ?? '';
				expect(/installing|probing/i.test(txt)).toBe(true);
			},
			{ timeout: 3000 }
		);
	});

	it('polls the status URL returned in the 202', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			jsonResponse(200, { install_id: 'mcp-job-1', state: 'installing' }),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const calls = getFetchMock().mock.calls;
				const polled = calls.some(([url]) => {
					if (typeof url !== 'string') return false;
					// String form (polling) OR Request object (less common).
					return url.includes('/api/mcp/install/mcp-job-1');
				});
				expect(polled).toBe(true);
			},
			{ timeout: 3000 }
		);
	});

	it('shows "installing"/"probing" text while the job is in a non-terminal state', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			jsonResponse(200, { install_id: 'mcp-job-1', state: 'installing' }),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		expect(
			await screen.findByText(/installing|probing/i, {}, { timeout: 3000 })
		).toBeInTheDocument();
	});

	it('shows a success message when the polled job reaches state=done', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			jsonResponse(200, {
				install_id: 'mcp-job-1',
				state: 'done',
				result: { tools: ['create_issue', 'list_repos'] },
			}),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const txt = document.body.textContent ?? '';
				expect(/installed|success|done|✓/i.test(txt)).toBe(true);
			},
			{ timeout: 3000 }
		);
	});

	it('shows the error message when the polled job reaches state=failed', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(202, {
				install_id: 'mcp-job-1',
				status_url: '/api/mcp/install/mcp-job-1',
				ws_channel: 'mcp-install',
				entry_id: 'github',
				target_agent: 'opencode',
			}),
			jsonResponse(200, {
				install_id: 'mcp-job-1',
				state: 'failed',
				error: { code: 'install_failed', message: 'spawn failed: ENOENT' },
			}),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const txt = document.body.textContent ?? '';
				expect(/failed|error|spawn failed/i.test(txt)).toBe(true);
			},
			{ timeout: 3000 }
		);
	});
});

// ============================================================================
// 13-15. HTTP error responses
// ============================================================================

describe('handleInstall — error responses', () => {
	it('shows a generic error message on a 500 response', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(500, { error: 'internal server error' }),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const txt = document.body.textContent ?? '';
				expect(/error|failed|install/i.test(txt)).toBe(true);
			},
			{ timeout: 3000 }
		);
	});

	it('marks the GITHUB_TOKEN input aria-invalid on a 422 missing_credentials response', async () => {
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			requiredEnv: [
				{ name: 'GITHUB_TOKEN', description: 'GitHub PAT', required: true, secret: true },
			],
		});
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(422, {
				error: 'missing_credentials',
				code: 'missing_credentials',
				required: ['GITHUB_TOKEN'],
			}),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		// User clicks Install without filling the form. The form must be in
		// the DOM (pinned by test 2) so the user could have filled it.
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const input = document.querySelector(
					'input[name="GITHUB_TOKEN"]'
				) as HTMLInputElement | null;
				expect(input).not.toBeNull();
				expect(input!.getAttribute('aria-invalid')).toBe('true');
			},
			{ timeout: 3000 }
		);
	});

	it('shows an "in progress" message on a 409 install_in_progress response', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(409, {
				error: 'install_in_progress',
				code: 'install_in_progress',
				existing_id: 'mcp-job-existing',
			}),
		]);

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});
		await user.click(screen.getByRole('button', { name: /^install$/i }));

		await waitFor(
			() => {
				const txt = document.body.textContent ?? '';
			expect(/in progress|already|conflict/i.test(txt)).toBe(true);
		},
		{ timeout: 3000 }
	);
});

// ============================================================================
// 16. handleInstall — network failure (fetch itself throws, not HTTP error)
// ============================================================================

describe('handleInstall — network failures', () => {
	it('shows a failed state with the thrown error message when the install fetch throws', async () => {
		const server = makeServer({ id: 'github', name: 'GitHub' });

		// Catalog succeeds; the install POST itself throws (network down / CORS / abort).
		getFetchMock().mockImplementation(async (url: unknown) => {
			if (typeof url === 'string' && url.endsWith('/api/mcp/catalog')) {
				return jsonResponse(200, [server]);
			}
			throw new TypeError('Failed to fetch');
		});

		const user = userEvent.setup();
		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});

		await user.click(screen.getByRole('button', { name: /^install$/i }));

		// The catch block (McpStore.tsx:331-339) stores the error via String(err)
		// and the card renders it under .mcp-store-install-error. String(TypeError)
		// yields "TypeError: Failed to fetch" — pin that exact string.
		await waitFor(
			() => {
				const errEl = document.querySelector('.mcp-store-install-error');
				expect(errEl).not.toBeNull();
				expect(errEl!.textContent ?? '').toMatch(/TypeError: Failed to fetch/i);
			},
			{ timeout: 3000 }
		);

		// And the card must NOT still be in the installing/probing state.
		expect(document.querySelector('.mcp-store-install-progress')).toBeNull();
	});
});

// ============================================================================
// 17. handleUninstall — happy path (entire function was untested)
// ============================================================================

describe('handleUninstall', () => {
	it('posts { id } to /api/mcp/uninstall and shows the success action message', async () => {
		const user = userEvent.setup();
		const server = makeServer({
			id: 'github',
			name: 'GitHub',
			installed: true,
		});
		mockFetchSequence([
			jsonResponse(200, [server]),
			jsonResponse(200, { name: 'GitHub', uninstalled: true }),
		]);

		render(<McpStore />);
		await waitFor(() => {
			expect(screen.queryByText(/Loading MCP catalog/i)).not.toBeInTheDocument();
		});

		// The button text is "Uninstall" when installed (McpStore.tsx:179).
		await user.click(screen.getByRole('button', { name: /^uninstall$/i }));

		// 1. POST /api/mcp/uninstall was called with { id: 'github' } in the body.
		const calls = getFetchMock().mock.calls;
		const uninstallCall = calls.find(([url, init]) => {
			return (
				typeof url === 'string' &&
				url.endsWith('/api/mcp/uninstall') &&
				(init as RequestInit | undefined)?.method === 'POST'
			);
		});
		expect(uninstallCall).toBeDefined();
		const body = JSON.parse((uninstallCall![1] as RequestInit).body as string);
		expect(body).toEqual({ id: 'github' });

		// 2. The success action message appears (McpStore.tsx:354) — auto-dismisses
		//    after 3s so query quickly, before the timer fires.
		await waitFor(
			() => {
				const msg = document.querySelector('.mcp-store-message');
				expect(msg).not.toBeNull();
				expect(msg!.textContent ?? '').toMatch(/uninstalled/i);
			},
			{ timeout: 1000 }
		);
	});
});
});
