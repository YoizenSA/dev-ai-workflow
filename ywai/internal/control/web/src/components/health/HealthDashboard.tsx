import { useHealth } from './useHealth';

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
		<div>
			<span>{name}</span>
			{ok ? (
				<span data-status="ok" className="ok-icon">✅</span>
			) : (
				<span data-status="error" className="error-icon">❌</span>
			)}
		</div>
	);
}

export function HealthDashboard() {
	const { data, loading, error } = useHealth();

	if (loading) {
		return <div>Loading health status...</div>;
	}

	if (error) {
		return <div>Error: {error.message}</div>;
	}

	if (!data) {
		return null;
	}

	const ok = data.daemon_ok && data.db_ok;

	return (
		<div>
			<h2>{ok ? 'Healthy' : 'Unhealthy'}</h2>
			<HealthStatusCard name="Daemon" ok={data.daemon_ok} />
			<HealthStatusCard name="Database" ok={data.db_ok} />
			<span>{data.repo_count} repos</span>
			<span>Last check: {formatDate(data.last_check)}</span>
		</div>
	);
}
