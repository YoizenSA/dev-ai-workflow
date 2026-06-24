import { useState, useEffect, useMemo, useRef } from 'react';
import { Globe, Search, Star } from 'lucide-react';
import './McpStore.css';

interface McpServer {
	id: string;
	name: string;
	description: string;
	category: string;
	icon: string;
	installed: boolean;
	enabled: boolean;
	popular: boolean;
	type: 'local' | 'remote';
	source: 'custom' | 'registry';
	status?: 'available' | 'connected' | 'disabled' | 'missing_executable' | 'connection_error';
	statusLabel?: string;
	statusMessage?: string;
	fixAction?: string;
	tools: string[];
	url?: string;
	docs?: string;
	requiredEnv?: Array<{
		name: string;
		description: string;
		required: boolean;
		secret: boolean;
	}>;
	installCmd?: string;
}

type InstallState = {
	state: 'idle' | 'pending' | 'installing' | 'probing' | 'done' | 'failed';
	installId?: string;
	errorCode?: string;
	errorMessage?: string;
	tools?: string[];
};

const categories = ['All', 'Documentation', 'Browser', 'Testing', 'Memory', 'Code Analysis', 'Core', 'Integration', 'Database', 'DevOps'];

type TargetAgent = 'opencode' | 'pi' | 'claude-code';

function CredentialsForm({
	server,
	values,
	onChange,
	error,
}: {
	server: McpServer;
	values: Record<string, string>;
	onChange: (name: string, value: string) => void;
	error?: string;
}) {
	if (!server.requiredEnv || server.requiredEnv.length === 0) return null;
	return (
		<div className="mcp-store-creds-form">
			{server.requiredEnv.map((env) => (
				<div key={env.name} className="mcp-store-creds-field">
					<label htmlFor={`cred-${server.id}-${env.name}`}>
						{env.name}
						{env.required && <span className="required">*</span>}
					</label>
					<input
						id={`cred-${server.id}-${env.name}`}
						name={env.name}
						type={env.secret ? 'password' : 'text'}
						value={values[env.name] || ''}
						onChange={(e) => onChange(env.name, e.target.value)}
						placeholder={env.description}
						aria-invalid={error ? 'true' : undefined}
						data-env={env.name}
					/>
				</div>
			))}
		</div>
	);
}

function McpCard({
	server,
	expanded,
	onToggleExpand,
	onInstall,
	onUninstall,
	credentials,
	onCredentialChange,
	installState,
}: {
	server: McpServer;
	expanded: boolean;
	onToggleExpand: () => void;
	onInstall: () => void;
	onUninstall: () => void;
	credentials: Record<string, string>;
	onCredentialChange: (name: string, value: string) => void;
	installState?: InstallState;
}) {
	const isInstalled = server.installed;
	const statusClass = server.status ? ` status-${server.status}` : '';
	const credsError =
		installState?.errorCode === 'missing_credentials' ? 'missing_credentials' : undefined;

	return (
		<div
			className={`mcp-store-card ${isInstalled ? 'installed' : ''} ${expanded ? 'expanded' : ''}`}
			onClick={onToggleExpand}
		>
			<div className="mcp-store-card-header">
				<span className="mcp-store-card-icon">{server.icon}</span>
				<div className="mcp-store-card-info">
					<div className="mcp-store-card-name-row">
						<h3 className="mcp-store-card-name">{server.name}</h3>
						{server.statusLabel && (
							<span className={`mcp-store-card-status-badge${statusClass}`}>
								{server.statusLabel}
							</span>
						)}
						{isInstalled && !server.statusLabel && (
							<span className="mcp-store-card-installed-badge">Installed</span>
						)}
					</div>
					<span className="mcp-store-card-category">{server.category}</span>
				</div>
			</div>

			<p className="mcp-store-card-description">{server.description}</p>

			{server.statusMessage && (
				<div className={`mcp-store-card-status-message${statusClass}`}>
					<span>{server.statusMessage}</span>
					{server.fixAction && <strong>{server.fixAction.replace(/_/g, ' ')}</strong>}
				</div>
			)}

			{!isInstalled && (
				<CredentialsForm
					server={server}
					values={credentials}
					onChange={onCredentialChange}
					error={credsError}
				/>
			)}

			{expanded && (
				<div className="mcp-store-card-details">
					{server.tools && server.tools.length > 0 && (
						<div className="mcp-store-card-tools">
							<span className="mcp-store-card-tools-label">Tools:</span>
							{server.tools.map((tool) => (
								<span key={tool} className="mcp-store-card-tool">
									{tool}
								</span>
							))}
						</div>
					)}
					{server.url && (
						<a
							className="mcp-store-card-link"
							href={server.url}
							target="_blank"
							rel="noopener noreferrer"
							onClick={(e) => e.stopPropagation()}
						>
							Documentation
						</a>
					)}
				</div>
			)}

			{installState?.state === 'installing' && (
				<div className="mcp-store-install-progress" aria-busy="true">Installing...</div>
			)}
			{installState?.state === 'probing' && (
				<div className="mcp-store-install-progress" aria-busy="true">Probing...</div>
			)}
			{installState?.state === 'done' && (
				<div className="mcp-store-install-success">
					✓ Installed ({installState.tools?.length || 0} tools)
				</div>
			)}
			{installState?.state === 'failed' && (
				<div className="mcp-store-install-error">✗ {installState.errorMessage}</div>
			)}

			<div className="mcp-store-card-actions">
				<button
					className={`mcp-store-install-btn ${isInstalled ? 'uninstall' : ''}`}
					onClick={(e) => {
						e.stopPropagation();
						if (isInstalled) {
							onUninstall();
						} else {
							onInstall();
						}
					}}
				>
					{isInstalled ? 'Uninstall' : 'Install'}
				</button>
			</div>
		</div>
	);
}

export function McpStore() {
	const [servers, setServers] = useState<McpServer[]>([]);
	const [search, setSearch] = useState('');
	const [selectedCategory, setSelectedCategory] = useState('All');
	const [showInstalledOnly, setShowInstalledOnly] = useState(false);
	const [showPopularOnly, setShowPopularOnly] = useState(false);
	const [expandedServer, setExpandedServer] = useState<string | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [actionMessage, setActionMessage] = useState<string | null>(null);
	const [installStates, setInstallStates] = useState<Record<string, InstallState>>({});
	const [credentials, setCredentials] = useState<Record<string, Record<string, string>>>({});
	const [selectedTarget, setSelectedTarget] = useState<TargetAgent>('opencode');
	const intervalsRef = useRef<Set<ReturnType<typeof setInterval>>>(new Set());

	useEffect(() => {
		fetch('/api/mcp/catalog')
			.then((res) => res.json())
			.then((data) => {
				setServers(data);
				setLoading(false);
			})
			.catch(() => {
				setError('Failed to load MCP catalog');
				setLoading(false);
			});
	}, []);

	// Clear any poll intervals when the component unmounts so timers don't
	// fire against a torn-down React tree.
	useEffect(() => {
		return () => {
			intervalsRef.current.forEach(clearInterval);
			intervalsRef.current.clear();
		};
	}, []);

	// Auto-dismiss action messages after 3s
	useEffect(() => {
		if (!actionMessage) return;
		const timer = setTimeout(() => setActionMessage(null), 3000);
		return () => clearTimeout(timer);
	}, [actionMessage]);

	const stopPolling = (interval: ReturnType<typeof setInterval>) => {
		clearInterval(interval);
		intervalsRef.current.delete(interval);
	};

	const pollStatus = (serverId: string, installId: string) => {
		const interval = setInterval(async () => {
			try {
				const res = await fetch(`/api/mcp/install/${installId}`);
				if (!res.ok) {
					stopPolling(interval);
					return;
				}
				const job = (await res.json()) as {
					state?: 'pending' | 'installing' | 'probing' | 'done' | 'failed';
					result?: { tools?: string[] };
					error?: { code?: string; message?: string };
				};
				if (job.state === 'done') {
					stopPolling(interval);
					setInstallStates((prev) => ({
						...prev,
						[serverId]: { state: 'done', tools: job.result?.tools },
					}));
					setServers((prev) =>
						prev.map((s) => (s.id === serverId ? { ...s, installed: true } : s))
					);
				} else if (job.state === 'failed') {
					stopPolling(interval);
					setInstallStates((prev) => ({
						...prev,
						[serverId]: {
							state: 'failed',
							errorCode: job.error?.code,
							errorMessage: job.error?.message,
						},
					}));
				} else {
					setInstallStates((prev) => ({
						...prev,
						[serverId]: { state: job.state ?? 'installing', installId },
					}));
				}
			} catch {
				stopPolling(interval);
			}
		}, 1000);
		intervalsRef.current.add(interval);
	};

	const handleInstall = async (id: string) => {
		const creds = credentials[id] || {};

		setInstallStates((prev) => ({ ...prev, [id]: { state: 'pending' } }));

		try {
			const res = await fetch('/api/mcp/install', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					id,
					target_agent: selectedTarget,
					credentials: creds,
				}),
			});

			if (res.status === 202) {
				const body = (await res.json()) as { install_id?: string };
				setInstallStates((prev) => ({
					...prev,
					[id]: { state: 'installing', installId: body.install_id },
				}));
				if (body.install_id) {
					pollStatus(id, body.install_id);
				}
			} else if (res.status === 422) {
				setInstallStates((prev) => ({
					...prev,
					[id]: {
						state: 'failed',
						errorCode: 'missing_credentials',
						errorMessage: 'Missing required credentials',
					},
				}));
			} else if (res.status === 409) {
				setInstallStates((prev) => ({
					...prev,
					[id]: {
						state: 'failed',
						errorCode: 'install_in_progress',
						errorMessage: 'Another install is in progress',
					},
				}));
			} else {
				setInstallStates((prev) => ({
					...prev,
					[id]: {
						state: 'failed',
						errorMessage: `Server error: ${res.status}`,
					},
				}));
			}
		} catch (err) {
			setInstallStates((prev) => ({
				...prev,
				[id]: {
					state: 'failed',
					errorMessage: String(err),
				},
			}));
		}
	};

	const handleUninstall = async (id: string) => {
		try {
			const res = await fetch('/api/mcp/uninstall', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ id }),
			});
			if (!res.ok) throw new Error('Uninstall failed');
			const result = await res.json();
			setServers((prev) =>
				prev.map((s) => (s.id === id ? { ...s, installed: false, enabled: false } : s))
			);
			setActionMessage(`[OK] ${result.name} uninstalled`);
		} catch {
			setActionMessage(`[ERROR] Failed to uninstall ${id}`);
		}
	};

	const filteredServers = useMemo(() => {
		return servers.filter((server) => {
			const matchesSearch =
				search === '' ||
				server.name.toLowerCase().includes(search.toLowerCase()) ||
				server.description.toLowerCase().includes(search.toLowerCase());

			const matchesCategory =
				selectedCategory === 'All' || server.category === selectedCategory;

			const matchesInstalled = !showInstalledOnly || server.installed;
			const matchesPopular = !showPopularOnly || server.popular;

			return matchesSearch && matchesCategory && matchesInstalled && matchesPopular;
		});
	}, [servers, search, selectedCategory, showInstalledOnly, showPopularOnly]);

	const installedCount = servers.filter((s) => s.installed).length;

	const customMcps = filteredServers.filter((s) => s.source === 'custom');
	const registryMcps = filteredServers.filter((s) => s.source === 'registry');

	if (loading) {
		return (
			<div className="mcp-store" aria-busy="true">
				<div className="mcp-store-header">
					<div className="mcp-store-title-row">
						<h2 className="mcp-store-title">MCP Store</h2>
					</div>
				</div>
				<div className="mcp-store-section-grid" aria-busy="true">
					{[...Array(6)].map((_, i) => (
						<div key={i} className="skeleton skel-card">
							<div className="skel-avatar" />
							<div className="skel-line title" />
							<div className="skel-line desc" />
							<div className="skel-line desc sm" />
							<div className="skel-line tag" />
						</div>
					))}
				</div>
			</div>
		);
	}

	if (error) {
		return (
			<div className="mcp-store">
				<div className="mcp-store-error">{error}</div>
			</div>
		);
	}

	const renderCard = (server: McpServer) => (
		<McpCard
			key={server.id}
			server={server}
			expanded={expandedServer === server.id}
			onToggleExpand={() =>
				setExpandedServer(expandedServer === server.id ? null : server.id)
			}
			onInstall={() => handleInstall(server.id)}
			onUninstall={() => handleUninstall(server.id)}
			credentials={credentials[server.id] || {}}
			onCredentialChange={(name, value) =>
				setCredentials((prev) => ({
					...prev,
					[server.id]: { ...(prev[server.id] || {}), [name]: value },
				}))
			}
			installState={installStates[server.id]}
		/>
	);

	return (
		<div className="mcp-store">
			{actionMessage && <div className="mcp-store-message">{actionMessage}</div>}

			<div className="mcp-store-header">
				<div className="mcp-store-title-row">
					<h2 className="mcp-store-title">MCP Store</h2>
					<span className="mcp-store-count">{installedCount} installed</span>
				</div>
				<p className="mcp-store-subtitle">
					Browse and install Model Context Protocol servers
				</p>
			</div>

			<div className="mcp-store-target-selector">
				<label htmlFor="mcp-target">Target agent:</label>
				<select
					id="mcp-target"
					name="mcp-target-agent"
					value={selectedTarget}
					onChange={(e) => setSelectedTarget(e.target.value as TargetAgent)}
					aria-label="Target agent"
				>
					<option value="opencode">OpenCode</option>
					<option value="pi">Pi</option>
					<option value="claude-code">Claude Code</option>
				</select>
			</div>

			<div className="mcp-store-controls">
				<div className="mcp-store-search-wrapper">
					<Search className="mcp-store-search-icon" size={16} />
					<input
						className="mcp-store-search-input"
						type="text"
						placeholder="Search MCP servers..."
						value={search}
						onChange={(e) => setSearch(e.target.value)}
					/>
				</div>

				<div className="mcp-store-filters">
					<button
						className={`mcp-store-filter-pill ${showInstalledOnly ? 'active' : ''}`}
						onClick={() => setShowInstalledOnly(!showInstalledOnly)}
						type="button"
					>
						Installed
					</button>
					<button
						className={`mcp-store-filter-pill ${showPopularOnly ? 'active' : ''}`}
						onClick={() => setShowPopularOnly(!showPopularOnly)}
						type="button"
					>
						Popular
					</button>
				</div>
			</div>

			<div className="mcp-store-categories">
				{categories.map((cat) => (
					<button
						key={cat}
						className={`mcp-store-category-btn ${selectedCategory === cat ? 'active' : ''}`}
						onClick={() => setSelectedCategory(cat)}
					>
						{cat}
					</button>
				))}
			</div>

			<div className="mcp-store-grid">
				{filteredServers.length === 0 ? (
					<div className="mcp-store-empty">
						No servers found. Try adjusting your search or filters.
					</div>
				) : (
					<>
						{customMcps.length > 0 && (
							<div className="mcp-store-section">
								<h2 className="mcp-store-section-title">
									<Star size={16} fill="currentColor" />
									Recommended
								</h2>
								<div className="mcp-store-section-grid">{customMcps.map(renderCard)}</div>
							</div>
						)}

						{registryMcps.length > 0 && (
							<div className="mcp-store-section">
								<h2 className="mcp-store-section-title">
									<Globe size={16} />
									Community
								</h2>
								<div className="mcp-store-section-grid">
									{registryMcps.map(renderCard)}
								</div>
							</div>
						)}
					</>
				)}
			</div>
		</div>
	);
}

export default McpStore;
