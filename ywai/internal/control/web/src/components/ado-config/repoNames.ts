// Shared repo-name handling for the ADO setup wizard and the management page.
// Both render a tag input whose pending text must survive Save: committing
// only on Enter silently dropped whatever the user had typed when they clicked
// the button instead, which is why profiles saved from the UI arrived with an
// empty repos list while `ado init` from the CLI worked.

const REPO_NAME = /^[a-zA-Z0-9._-]+$/;

export interface ParsedRepoNames {
	valid: string[];
	rejected: string[];
}

/** Splits a comma-separated tag input into valid and rejected repo names. */
export function parseRepoNames(raw: string): ParsedRepoNames {
	const names = (raw ?? "")
		.split(",")
		.map((s) => s.trim())
		.filter(Boolean);
	const valid: string[] = [];
	const rejected: string[] = [];
	for (const n of names) {
		(REPO_NAME.test(n) ? valid : rejected).push(n);
	}
	return { valid, rejected };
}

/**
 * Returns committed repos plus anything still sitting in the tag input.
 * Call this when building the save payload so a typed-but-not-Entered name is
 * not lost. Order is preserved and duplicates collapse.
 */
export function mergeRepos(committed: string[], pending: string): string[] {
	return [...new Set([...(committed ?? []), ...parseRepoNames(pending).valid])];
}
