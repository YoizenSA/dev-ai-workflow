import { useCallback } from "react";
import { useSearchParams } from "react-router-dom";

// useUrlTab keeps a tab / sub-view selection in a query param (default ?tab=)
// so the view survives reload (F5) and can be deep-linked / shared. Unknown or
// missing values fall back to defaultTab, and selecting the default clears the
// param to keep URLs clean.
export function useUrlTab<T extends string>(
	defaultTab: T,
	valid: readonly T[],
	param = "tab",
): [T, (tab: T) => void] {
	const [searchParams, setSearchParams] = useSearchParams();
	const raw = searchParams.get(param) as T | null;
	const tab = raw && valid.includes(raw) ? raw : defaultTab;

	const setTab = useCallback(
		(next: T) => {
			const params = new URLSearchParams(searchParams);
			if (next === defaultTab) params.delete(param);
			else params.set(param, next);
			setSearchParams(params, { replace: true });
		},
		[searchParams, setSearchParams, defaultTab, param],
	);

	return [tab, setTab];
}
