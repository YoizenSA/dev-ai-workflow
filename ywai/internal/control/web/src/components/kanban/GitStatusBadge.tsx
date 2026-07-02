import { useEffect, useState } from "react";
import { gitApi, type GitStatus } from "../../api/client";

export function GitStatusBadge() {
	const [status, setStatus] = useState<GitStatus | null>(null);

	useEffect(() => {
		gitApi.getStatus().then(setStatus).catch(() => {});
	}, []);

	if (!status) return null;

	const parts: string[] = [status.branch];
	if (status.ahead > 0) parts.push(`↑${status.ahead}`);
	if (status.behind > 0) parts.push(`↓${status.behind}`);
	if (status.changed_files > 0) parts.push(`~${status.changed_files}`);
	if (status.untracked_files > 0) parts.push(`+${status.untracked_files}`);

	return (
		<span
			className={`git-status-badge ${status.dirty ? "git-dirty" : "git-clean"}`}
			title={status.remote_url || status.branch}
		>
			{status.dirty ? parts.join(" ") : `${status.branch} | clean`}
		</span>
	);
}
