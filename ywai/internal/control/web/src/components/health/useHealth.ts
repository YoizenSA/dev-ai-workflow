import { useState, useEffect } from 'react';

interface HealthStatus {
	daemon_ok: boolean;
	db_ok: boolean;
	repo_count: number;
	last_check: string;
}

export function useHealth() {
	const [data, setData] = useState<HealthStatus | null>(null);
	const [loading, setLoading] = useState(true);
	const [error, setError] = useState<Error | null>(null);

	useEffect(() => {
		let cancelled = false;

		fetch('/api/health', {})
			.then((res) => res.json())
			.then((json) => {
				if (!cancelled) {
					setData(json as HealthStatus);
					setLoading(false);
				}
			})
			.catch((err) => {
				if (!cancelled) {
					setError(err instanceof Error ? err : new Error(String(err)));
					setLoading(false);
				}
			});

		return () => {
			cancelled = true;
		};
	}, []);

	return { data, loading, error };
}
