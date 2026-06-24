import { useState, useEffect, useCallback } from 'react';
import './AdoConfig.css';

interface AdoProfile {
	org: string;
	project: string;
	patEnvVar: string;
	repos: string[];
}

interface AdoPluginConfig {
	enabled: boolean;
	defaultProfile: string;
	profiles: Record<string, AdoProfile>;
}

const emptyProfile: AdoProfile = {
	org: '',
	project: '',
	patEnvVar: 'AZURE_DEVOPS_PAT',
	repos: [],
};

export function AdoConfig() {
	const [config, setConfig] = useState<AdoPluginConfig | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<string | null>(null);
	const [message, setMessage] = useState<{ text: string; type: 'success' | 'error' } | null>(null);

	// Modal state
	const [showModal, setShowModal] = useState(false);
	const [editingName, setEditingName] = useState<string | null>(null);
	const [formName, setFormName] = useState('');
	const [formProfile, setFormProfile] = useState<AdoProfile>({ ...emptyProfile });
	const [repoInput, setRepoInput] = useState('');
	const [showSetupGuide, setShowSetupGuide] = useState(false);

	// Auto-dismiss messages
	useEffect(() => {
		if (!message) return;
		const timer = setTimeout(() => setMessage(null), 3000);
		return () => clearTimeout(timer);
	}, [message]);

	const fetchConfig = useCallback(async () => {
		try {
			setLoading(true);
			const res = await fetch('/api/ado/config');
			if (!res.ok) throw new Error('Failed to fetch config');
			const data = await res.json();
			setConfig(data);
			setError(null);
		} catch (err) {
			setError(err instanceof Error ? err.message : 'Unknown error');
		} finally {
			setLoading(false);
		}
	}, []);

	useEffect(() => {
		fetchConfig();
	}, [fetchConfig]);

	const handleToggle = async () => {
		if (!config) return;
		try {
			const res = await fetch('/api/ado/toggle', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ enabled: !config.enabled }),
			});
			if (!res.ok) throw new Error('Toggle failed');
			const data = await res.json();
			setConfig(data.config);
			setMessage({ text: data.message, type: 'success' });
		} catch (err) {
			setMessage({ text: err instanceof Error ? err.message : 'Toggle failed', type: 'error' });
		}
	};

	const handleDefaultChange = async (profileName: string) => {
		if (!config) return;
		try {
			const updated = { ...config, defaultProfile: profileName };
			const res = await fetch('/api/ado/config', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify(updated),
			});
			if (!res.ok) throw new Error('Failed to update default profile');
			const data = await res.json();
			setConfig(data.config);
			setMessage({ text: 'Default profile updated', type: 'success' });
		} catch (err) {
			setMessage({ text: err instanceof Error ? err.message : 'Update failed', type: 'error' });
		}
	};

	const openAddModal = () => {
		setEditingName(null);
		setFormName('');
		setFormProfile({ ...emptyProfile });
		setRepoInput('');
		setShowModal(true);
	};

	const openEditModal = (name: string) => {
		if (!config) return;
		const profile = config.profiles[name];
		setEditingName(name);
		setFormName(name);
		setFormProfile({
			org: profile.org,
			project: profile.project,
			patEnvVar: profile.patEnvVar || 'AZURE_DEVOPS_PAT',
			repos: [...(profile.repos || [])],
		});
		setRepoInput('');
		setShowModal(true);
	};

	const closeModal = () => {
		setShowModal(false);
		setEditingName(null);
		setFormName('');
		setFormProfile({ ...emptyProfile });
		setRepoInput('');
	};

	const handleSaveProfile = async () => {
		const name = editingName || formName.trim();
		if (!name) {
			setMessage({ text: 'Profile name is required', type: 'error' });
			return;
		}
		if (!formProfile.org.trim()) {
			setMessage({ text: 'Organization is required', type: 'error' });
			return;
		}
		if (!formProfile.project.trim()) {
			setMessage({ text: 'Project is required', type: 'error' });
			return;
		}

		try {
			const res = await fetch('/api/ado/profile', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					name,
					profile: {
						org: formProfile.org.trim(),
						project: formProfile.project.trim(),
						patEnvVar: formProfile.patEnvVar.trim() || 'AZURE_DEVOPS_PAT',
						repos: formProfile.repos,
					},
				}),
			});
			if (!res.ok) {
				const data = await res.json();
				throw new Error(data.error || 'Failed to save profile');
			}
			const data = await res.json();
			setConfig(data.config);
			setMessage({ text: data.message, type: 'success' });
			closeModal();
		} catch (err) {
			setMessage({ text: err instanceof Error ? err.message : 'Save failed', type: 'error' });
		}
	};

	const handleDeleteProfile = async (name: string) => {
		if (!config) return;
		try {
			const res = await fetch('/api/ado/profile', {
				method: 'DELETE',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ name }),
			});
			if (!res.ok) {
				const data = await res.json();
				throw new Error(data.error || 'Failed to delete profile');
			}
			const data = await res.json();
			setConfig(data.config);
			setMessage({ text: data.message, type: 'success' });
		} catch (err) {
			setMessage({ text: err instanceof Error ? err.message : 'Delete failed', type: 'error' });
		}
	};

	const addRepo = () => {
		const repo = repoInput.trim();
		if (!repo) return;
		if (formProfile.repos.includes(repo)) {
			setRepoInput('');
			return;
		}
		setFormProfile({
			...formProfile,
			repos: [...formProfile.repos, repo],
		});
		setRepoInput('');
	};

	const removeRepo = (repo: string) => {
		setFormProfile({
			...formProfile,
			repos: formProfile.repos.filter((r) => r !== repo),
		});
	};

	const handleRepoKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === 'Enter') {
			e.preventDefault();
			addRepo();
		}
	};

	const profileNames = config ? Object.keys(config.profiles) : [];

	if (loading) {
		return (
			<div className="ado-config" aria-busy="true">
				<div className="ado-config-header">
					<div className="ado-config-title-section">
						<h2>Azure DevOps Plugin</h2>
						<p className="text-muted" style={{ fontSize: 'var(--font-size-sm)', margin: 0 }}>Loading configuration...</p>
					</div>
				</div>
				<div className="ado-config-profiles" aria-busy="true">
					{[...Array(2)].map((_, i) => (
						<div key={i} className="skeleton skel-card" style={{ minHeight: 140 }}>
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
			<div className="ado-config">
				<p style={{ color: 'var(--red-500, #ef4444)' }}>Error: {error}</p>
			</div>
		);
	}

	return (
		<div className="ado-config">
			{/* Header */}
			<div className="ado-config-header">
				<div className="ado-config-title-group">
					<h1 className="ado-config-title">Azure DevOps Configuration</h1>
					<span className={`ado-config-status ${config?.enabled ? 'enabled' : 'disabled'}`}>
						{config?.enabled ? 'Enabled' : 'Disabled'}
					</span>
				</div>
				<div className="ado-config-toggle">
					<span className="ado-config-toggle-label">
						Plugin active
					</span>
					<label className="toggle-switch">
						<input
							type="checkbox"
							checked={config?.enabled ?? false}
							onChange={handleToggle}
						/>
						<span className="toggle-slider" />
					</label>
				</div>
			</div>

			{/* Messages */}
			{message && (
				<div className={`ado-config-message ${message.type}`}>
					{message.text}
				</div>
			)}

			{/* Default Profile Selector */}
			{profileNames.length > 0 && (
				<div className="ado-config-default">
					<span className="ado-config-default-label">Default Profile:</span>
					<select
						className="ado-config-default-select"
						value={config?.defaultProfile ?? ''}
						onChange={(e) => handleDefaultChange(e.target.value)}
					>
						{profileNames.map((name) => (
							<option key={name} value={name}>
								{name}
							</option>
						))}
					</select>
				</div>
			)}

			{/* Profiles Section */}
			<div className="ado-config-section-header">
				<h2 className="ado-config-section-title">Profiles</h2>
				<button className="ado-config-add-btn" onClick={openAddModal}>
					+ Add Profile
				</button>
			</div>

			{profileNames.length === 0 ? (
				<div className="ado-config-empty">
					<h3 className="ado-config-empty-title">No profiles configured</h3>
					<p className="ado-config-empty-desc">
						Add an ADO profile to connect to your Azure DevOps organization.
					</p>
				</div>
			) : (
				<div className="ado-config-profiles">
					{profileNames.map((name) => {
						const profile = config!.profiles[name];
						const isDefault = name === config!.defaultProfile;
						return (
							<div
								key={name}
								className={`ado-config-card ${isDefault ? 'default' : ''}`}
							>
								<div className="ado-config-card-header">
									<div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
										<h3 className="ado-config-card-name">{name}</h3>
										{isDefault && (
											<span className="ado-config-card-default-badge">Default</span>
										)}
									</div>
									<div className="ado-config-card-actions">
										<button
											className="ado-config-card-btn"
											onClick={() => openEditModal(name)}
										>
											Edit
										</button>
										<button
											className="ado-config-card-btn danger"
											onClick={() => handleDeleteProfile(name)}
										>
											Delete
										</button>
									</div>
								</div>

								<div className="ado-config-field">
									<span className="ado-config-field-badge">ORG</span>
									<span className="ado-config-field-value">{profile.org}</span>
								</div>

								<div className="ado-config-field">
									<span className="ado-config-field-badge">PRJ</span>
									<span className="ado-config-field-value">{profile.project}</span>
								</div>

								<div className="ado-config-field">
									<span className="ado-config-field-badge">PAT</span>
									<span className="ado-config-field-value">{profile.patEnvVar}</span>
								</div>

								{profile.repos && profile.repos.length > 0 && (
									<div className="ado-config-field">
										<span className="ado-config-field-badge">REPO</span>
										<div className="ado-config-repos">
											{profile.repos.map((repo) => (
												<span key={repo} className="ado-config-repo-pill">
													{repo}
												</span>
											))}
										</div>
									</div>
								)}
							</div>
						);
					})}
				</div>
			)}

			{/* Modal */}
			{showModal && (
				<div className="ado-config-modal-overlay" onClick={closeModal}>
					<div className="ado-config-modal" onClick={(e) => e.stopPropagation()}>
						<div className="ado-config-modal-header">
							<h2 className="ado-config-modal-title">
								{editingName ? `Edit Profile: ${editingName}` : 'Add Profile'}
							</h2>
							<button className="ado-config-modal-close" onClick={closeModal}>
								x
							</button>
						</div>
						<div className="ado-config-modal-body">
							<div className="ado-config-form-group">
								<label className="ado-config-form-label">Profile Name</label>
								<input
									className="ado-config-form-input"
									type="text"
									value={formName}
									onChange={(e) => setFormName(e.target.value)}
									placeholder="e.g. my-project"
									disabled={!!editingName}
								/>
								{editingName && (
									<p className="ado-config-form-hint">Profile name cannot be changed</p>
								)}
							</div>

							<div className="ado-config-form-group">
								<label className="ado-config-form-label">Organization</label>
								<input
									className="ado-config-form-input"
									type="text"
									value={formProfile.org}
									onChange={(e) =>
										setFormProfile({ ...formProfile, org: e.target.value })
									}
									placeholder="e.g. my-org"
								/>
							</div>

							<div className="ado-config-form-group">
								<label className="ado-config-form-label">Project</label>
								<input
									className="ado-config-form-input"
									type="text"
									value={formProfile.project}
									onChange={(e) =>
										setFormProfile({ ...formProfile, project: e.target.value })
									}
									placeholder="e.g. my-project"
								/>
							</div>

							<div className="ado-config-form-group">
								<label className="ado-config-form-label">
									PAT Environment Variable
								</label>
								<input
									className="ado-config-form-input"
									type="text"
									value={formProfile.patEnvVar}
									onChange={(e) =>
										setFormProfile({ ...formProfile, patEnvVar: e.target.value })
									}
									placeholder="AZURE_DEVOPS_PAT"
								/>
								<p className="ado-config-form-hint">
									Name of the environment variable holding your PAT token
								</p>
							</div>

							<div className="ado-config-form-group">
								<label className="ado-config-form-label">Repositories</label>
								<div
									className="ado-config-tag-input-wrapper"
									onClick={(e) => {
										const input = e.currentTarget.querySelector('input');
										if (input) input.focus();
									}}
								>
									{formProfile.repos.map((repo) => (
										<span key={repo} className="ado-config-repo-pill">
											{repo}
											<button
												className="ado-config-repo-pill-remove"
												onClick={() => removeRepo(repo)}
												type="button"
											>
												x
											</button>
										</span>
									))}
									<input
										className="ado-config-tag-input"
										type="text"
										value={repoInput}
										onChange={(e) => setRepoInput(e.target.value)}
										onKeyDown={handleRepoKeyDown}
										placeholder="Type repo name, press Enter"
									/>
								</div>
								<p className="ado-config-form-hint">
									Press Enter to add a repository. Leave empty to access all repos.
								</p>
							</div>
						</div>
						<div className="ado-config-modal-footer">
							<button className="ado-config-btn" onClick={closeModal}>
								Cancel
							</button>
							<button className="ado-config-btn primary" onClick={handleSaveProfile}>
								Save
							</button>
						</div>
					</div>
				</div>
			)}

			<div className="ado-setup-guide">
				<button
					className="ado-setup-toggle"
					onClick={() => setShowSetupGuide(!showSetupGuide)}
				>
					{showSetupGuide ? 'Hide' : 'Show'} Environment Variable Setup Guide
				</button>

				{showSetupGuide && (
					<div className="ado-setup-content">
						<h3>Setting up AZURE_DEVOPS_PAT</h3>
						<p>The PAT (Personal Access Token) is read from an environment variable for security. Set it in your shell profile:</p>

						<div className="ado-script-block">
							<h4>Bash / Zsh (~/.bashrc or ~/.zshrc)</h4>
							<pre><code>{`# Add to ~/.bashrc or ~/.zshrc
export AZURE_DEVOPS_PAT="your-personal-access-token-here"

# To create a PAT:
# 1. Go to https://dev.azure.com/{your-org}/_usersSettings/tokens
# 2. Click "New Token"
# 3. Select scopes: Code (Read & Write), Work Items (Read & Write)
# 4. Copy the token and replace above

# Reload shell after adding:
source ~/.bashrc  # or source ~/.zshrc`}</code></pre>
						</div>

						<div className="ado-script-block">
							<h4>PowerShell ($PROFILE)</h4>
							<pre><code>{`# Add to $PROFILE (run: notepad $PROFILE)
$env:AZURE_DEVOPS_PAT = "your-personal-access-token-here"

# To create a PAT:
# 1. Go to https://dev.azure.com/{your-org}/_usersSettings/tokens
# 2. Click "New Token"
# 3. Select scopes: Code (Read & Write), Work Items (Read & Write)
# 4. Copy the token and replace above

# Reload profile after adding:
. $PROFILE`}</code></pre>
						</div>

						<div className="ado-script-block">
							<h4>Windows System Environment Variable</h4>
							<pre><code>{`# Run in PowerShell as Administrator:
[System.Environment]::SetEnvironmentVariable(
  "AZURE_DEVOPS_PAT",
  "your-personal-access-token-here",
  "User"
)

# Restart your terminal after running this`}</code></pre>
						</div>
					</div>
				)}
			</div>
		</div>
	);
}

export default AdoConfig;
