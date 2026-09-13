import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { useHealth } from './useHealth';
import { envsApi, type EnvProfile } from '../../api/envs';
import { SkeletonScreen } from '../../bones/SkeletonScreen';
import { HealthBonesFallback, HealthCaptureFixture } from '../../bones/fallbacks';
import './health.css';

function formatDate(iso: string): string {
	// A "last check" is an instant: the day matters, not just the month.
	return new Date(iso).toLocaleDateString('en-US', {
		year: 'numeric',
		month: 'long',
		day: 'numeric',
	});
}

interface HealthStatusCardProps {
	name: string;
	ok: boolean;
}

export function HealthStatusCard({ name, ok }: HealthStatusCardProps) {
	return (
		<div className="card card-pad health-status-card">
			<span className="health-status-card__name">{name}</span>
			{ok ? (
				<span data-status="ok" className="ok-icon health-status-card__icon ok">✓</span>
			) : (
				<span data-status="error" className="error-icon health-status-card__icon error">✗</span>
			)}
		</div>
	);
}

function EnvsSection() {
	const [envs, setEnvs] = useState<EnvProfile[] | null>(null);
	const [envsError, setEnvsError] = useState<string | null>(null);

	useEffect(() => {
		let cancelled = false;
		envsApi
			.list()
			.then((res) => {
				if (!cancelled) setEnvs(res.envs ?? []);
			})
			.catch((err) => {
				if (!cancelled) {
					setEnvs([]);
					setEnvsError(err instanceof Error ? err.message : String(err));
				}
			});
		return () => {
			cancelled = true;
		};
	}, []);

	return (
		<section className="health-envs" aria-label="Environments">
			<div className="health-envs-head">
				<h3>Environments{envs ? ` (${envs.length})` : ''}</h3>
				<Link to="/envs" className="health-envs-link">
					Manage environments →
				</Link>
			</div>
			{envsError && (
				<p className="health-envs-error" role="alert">
					Could not load environments: {envsError}
				</p>
			)}
			{envs === null ? (
				<p className="health-envs-muted">Loading environments…</p>
			) : envs.length === 0 && !envsError ? (
				<p className="health-envs-muted">
					No environments yet. <Link to="/envs">Create one</Link> to get an isolated
					opencode setup.
				</p>
			) : envs.length > 0 ? (
				<div className="health-envs-grid">
					{envs.map((env) => (
						<div
							key={env.name}
							className={`card card-pad health-env-card${env.running ? ' is-running' : ''}`}
							data-env={env.name}
							data-running={env.running ? 'true' : 'false'}
						>
							<div className="health-env-card-head">
								<span
									className={`health-env-dot ${env.running ? 'on' : 'off'}`}
									aria-hidden
								/>
								<span className="health-env-name">{env.name}</span>
								<span className={`health-env-badge ${env.running ? 'on' : 'off'}`}>
									{env.running ? 'Running' : 'Stopped'}
								</span>
							</div>
							<div className="health-env-meta">
								<span>{env.preset}</span>
								{env.url && <code>{env.url}</code>}
							</div>
						</div>
					))}
				</div>
			) : null}
		</section>
	);
}

export function HealthDashboard() {
	const { data, loading, error } = useHealth();

	if (error) {
		return <div className="health-dashboard"><div className="hub-page__empty">Error: {error.message}</div></div>;
	}

	const ok = data ? data.daemon_ok && data.db_ok : false;

	return (
		<SkeletonScreen
			name="health-dashboard"
			loading={loading || !data}
			fallback={<HealthBonesFallback />}
			fixture={<HealthCaptureFixture />}
		>
			{data ? (
				<div className="health-dashboard">
					<div className={`health-summary ${ok ? 'healthy' : 'unhealthy'}`}>
						<h2>{ok ? 'Healthy' : 'Unhealthy'}</h2>
						<p className="health-subtitle">Last check: {formatDate(data.last_check)}</p>
					</div>
					<div className="health-cards">
						<HealthStatusCard name="Daemon" ok={data.daemon_ok} />
						<HealthStatusCard name="Database" ok={data.db_ok} />
					</div>
					<div className="health-meta">
						<span>{data.repo_count} repos</span>
					</div>
					<EnvsSection />
				</div>
			) : (
				// Placeholder children for boneyard capture when still loading
				<div className="health-dashboard" />
			)}
		</SkeletonScreen>
	);
}
