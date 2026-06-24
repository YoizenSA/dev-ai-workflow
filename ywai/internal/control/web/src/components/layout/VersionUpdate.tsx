import { useEffect, useState } from "react";
import { ArrowUp, Loader2, AlertTriangle } from "lucide-react";
import { configApi } from "../../api/client";

type Status = "idle" | "updating" | "error";

interface VersionInfo {
	current: string;
	latest: string | null;
	updateAvailable: boolean;
}

// Polls /health until the relaunched server answers, then reloads the page so
// the freshly built UI is served. The update pipeline kills and restarts this
// server, so there is a window where requests fail — that is expected.
async function waitForRestartAndReload(): Promise<void> {
	const deadline = Date.now() + 5 * 60 * 1000; // 5 min safety cap
	let sawDown = false;
	while (Date.now() < deadline) {
		await new Promise((r) => setTimeout(r, 2000));
		try {
			const res = await fetch("/health", { cache: "no-store" });
			if (!res.ok) throw new Error("not ok");
			// Only reload once we have observed the server go down and come
			// back, so we don't reload before the new binary is live.
			if (sawDown) {
				window.location.reload();
				return;
			}
		} catch {
			sawDown = true;
		}
	}
	// Timed out — reload anyway as a last resort.
	window.location.reload();
}

export function VersionUpdate(): JSX.Element | null {
	const [info, setInfo] = useState<VersionInfo | null>(null);
	const [status, setStatus] = useState<Status>("idle");
	const [error, setError] = useState<string | null>(null);

	useEffect(() => {
		configApi
			.getVersion()
			.then(setInfo)
			.catch(() => {});
	}, []);

	// Nothing to show unless an update is actually available.
	if (!info || !info.updateAvailable || !info.latest) return null;

	const handleUpdate = async () => {
		setStatus("updating");
		setError(null);
		try {
			const res = await configApi.triggerUpdate();
			if (!res.started) throw new Error(res.error ?? "update did not start");
			await waitForRestartAndReload();
		} catch (e) {
			setStatus("error");
			setError(e instanceof Error ? e.message : String(e));
		}
	};

	if (status === "updating") {
		return (
			<div className="version-update is-updating" role="status">
				<Loader2 size={16} className="version-update-spin" aria-hidden="true" />
				<span className="version-update-text">
					Updating to {info.latest}… the dashboard will reload automatically.
				</span>
			</div>
		);
	}

	return (
		<div className="version-update">
			<div className="version-update-info">
				<ArrowUp size={16} aria-hidden="true" />
				<span className="version-update-text">
					Update available
					<span className="version-update-versions">
						{info.current} → {info.latest}
					</span>
				</span>
			</div>
			{status === "error" && error && (
				<span className="version-update-error" role="alert">
					<AlertTriangle size={13} aria-hidden="true" /> {error}
				</span>
			)}
			<button
				type="button"
				className="version-update-btn"
				onClick={handleUpdate}
			>
				Update now
			</button>
		</div>
	);
}
