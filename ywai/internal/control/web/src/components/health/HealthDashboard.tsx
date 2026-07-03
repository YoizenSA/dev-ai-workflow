import { useHealth } from './useHealth';
import './health.css';

function formatDate(iso: string): string {
	return new Date(iso).toLocaleDateString('en-US', {
		year: 'numeric',
		month: 'long',
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

export function HealthDashboard() {
	const { data, loading, error } = useHealth();

	if (loading) {
		return <div className="health-dashboard"><div className="hub-page__empty">Loading health status...</div></div>;
	}

	if (error) {
		return <div className="health-dashboard"><div className="hub-page__empty">Error: {error.message}</div></div>;
	}

	if (!data) {
		return null;
	}

	const ok = data.daemon_ok && data.db_ok;

	return (
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
		</div>
	);
}
