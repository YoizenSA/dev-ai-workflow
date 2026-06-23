import { useState, useEffect, useMemo } from 'react';
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
	type: string;
	source: 'custom' | 'registry';
	tools: string[];
	url: string;
	docs?: string;
}

const categories = ['All', 'Documentation', 'Browser', 'Testing', 'Memory', 'Code Analysis', 'Core', 'Integration', 'Database', 'DevOps'];

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

	// Auto-dismiss action messages after 3s
	useEffect(() => {
		if (!actionMessage) return;
		const timer = setTimeout(() => setActionMessage(null), 3000);
		return () => clearTimeout(timer);
	}, [actionMessage]);

	const handleInstall = async (id: string) => {
		try {
			const res = await fetch('/api/mcp/install', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ id }),
			});
			if (!res.ok) throw new Error('Install failed');
			const result = await res.json();
			setServers((prev) =>
				prev.map((s) => (s.id === id ? { ...s, installed: true, enabled: true } : s))
			);
			setActionMessage(`[OK] ${result.name} installed`);
		} catch {
			setActionMessage(`[ERROR] Failed to install ${id}`);
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

	const customMcps = filteredServers.filter(s => s.source === 'custom');
	const registryMcps = filteredServers.filter(s => s.source === 'registry');

	if (loading) {
		return (
			<div className="mcp-store">
				<div className="mcp-store-loading">Loading MCP catalog...</div>
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

	return (
		<div className="mcp-store">
			{actionMessage && (
				<div className="mcp-store-message">
					{actionMessage}
				</div>
			)}

			<div className="mcp-store-header">
				<div className="mcp-store-title-row">
					<h2 className="mcp-store-title">MCP Store</h2>
					<span className="mcp-store-count">{installedCount} installed</span>
				</div>
				<p className="mcp-store-subtitle">
					Browse and install Model Context Protocol servers
				</p>
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
								<div className="mcp-store-section-grid">
									{customMcps.map((server) => (
										<div
											key={server.id}
											className={`mcp-store-card ${server.installed ? 'installed' : ''} ${expandedServer === server.id ? 'expanded' : ''}`}
											onClick={() =>
												setExpandedServer(expandedServer === server.id ? null : server.id)
											}
										>
											<div className="mcp-store-card-header">
												<span className="mcp-store-card-icon">{server.icon}</span>
												<div className="mcp-store-card-info">
													<div className="mcp-store-card-name-row">
														<h3 className="mcp-store-card-name">{server.name}</h3>
														{server.installed && (
															<span className="mcp-store-card-installed-badge">
																Installed
															</span>
														)}
													</div>
													<span className="mcp-store-card-category">{server.category}</span>
												</div>
											</div>

											<p className="mcp-store-card-description">{server.description}</p>

											{expandedServer === server.id && (
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

											<div className="mcp-store-card-actions">
												<button
													className={`mcp-store-install-btn ${server.installed ? 'uninstall' : ''}`}
													onClick={(e) => {
														e.stopPropagation();
														if (server.installed) {
															handleUninstall(server.id);
														} else {
															handleInstall(server.id);
														}
													}}
												>
													{server.installed ? 'Uninstall' : 'Install'}
												</button>
											</div>
										</div>
									))}
								</div>
							</div>
						)}

						{registryMcps.length > 0 && (
							<div className="mcp-store-section">
								<h2 className="mcp-store-section-title">
									<Globe size={16} />
									Community
								</h2>
								<div className="mcp-store-section-grid">
									{registryMcps.map((server) => (
										<div
											key={server.id}
											className={`mcp-store-card ${server.installed ? 'installed' : ''} ${expandedServer === server.id ? 'expanded' : ''}`}
											onClick={() =>
												setExpandedServer(expandedServer === server.id ? null : server.id)
											}
										>
											<div className="mcp-store-card-header">
												<span className="mcp-store-card-icon">{server.icon}</span>
												<div className="mcp-store-card-info">
													<div className="mcp-store-card-name-row">
														<h3 className="mcp-store-card-name">{server.name}</h3>
														{server.installed && (
															<span className="mcp-store-card-installed-badge">
																Installed
															</span>
														)}
													</div>
													<span className="mcp-store-card-category">{server.category}</span>
												</div>
											</div>

											<p className="mcp-store-card-description">{server.description}</p>

											{expandedServer === server.id && (
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

											<div className="mcp-store-card-actions">
												<button
													className={`mcp-store-install-btn ${server.installed ? 'uninstall' : ''}`}
													onClick={(e) => {
														e.stopPropagation();
														if (server.installed) {
															handleUninstall(server.id);
														} else {
															handleInstall(server.id);
														}
													}}
												>
													{server.installed ? 'Uninstall' : 'Install'}
												</button>
											</div>
										</div>
									))}
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
