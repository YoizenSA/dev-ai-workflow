/**
 * Report to markdown (PLAN 4 / 8).
 *
 * The agent gets a short `summary_md`, never the 40 KB of JSON - that goes to
 * storage under the runId. Every line here is traceable to something Jev
 * answered: no line in this file may describe a problem Jev did not mark.
 */
import type { ReviewReport } from "./domain/types"

function pct(probability: number): string {
	return `${Math.round(probability * 100)}%`
}

/** Severity is continuous (2.97, not 3), so print one decimal. */
function sev(severity: number): string {
	return severity.toFixed(1)
}

export function summarize(report: ReviewReport): string {
	const lines: string[] = []
	const verdict = {
		request_changes: "REQUEST CHANGES",
		comment: "COMMENT",
		clean: "CLEAN",
	}[report.action]

	const fileCount = Object.keys(report.matrix).length
	lines.push(
		`**Jev review - ${verdict}** (${report.findings.length} finding(s) across ${fileCount} file(s), ${(report.latencyMs / 1000).toFixed(1)}s, ${report.usage.requests} requests)`,
	)

	if (report.findings.length === 0) {
		const screened = Object.values(report.matrix).some((scores) =>
			Object.values(scores).some((value) => (value ?? 0) >= 0.7),
		)
		lines.push(
			screened
				? "\nSignals screened over threshold but none could be placed on a changed line, so there is nothing to report as a finding."
				: "\nNo signal reached the screen threshold.",
		)
	}

	for (const finding of report.findings) {
		const owner = finding.owner ? ` - owner: ${finding.owner}` : ""
		const range =
			finding.startLine === finding.endLine
				? `${finding.startLine}`
				: `${finding.startLine}-${finding.endLine}`
		lines.push(
			`\n- \`${finding.file}:${range}\` **${finding.dimension}** (${pct(finding.probability)}, severity ${sev(finding.severity)}${owner})\n  mechanism: \`${finding.mechanism}\``,
		)
	}

	if (report.skipped.length > 0) {
		const shown = report.skipped.slice(0, 5).map((s) => `${s.path} (${s.reason})`)
		const more = report.skipped.length > shown.length ? `, +${report.skipped.length - shown.length} more` : ""
		lines.push(`\nSkipped: ${shown.join("; ")}${more}`)
	}

	lines.push(
		`\nrunId: \`${report.runId}\` - questions ${report.questionsVersion} - source: ${report.source}`,
	)
	lines.push(
		"\nThese findings are Jev's. Do not add your own and attribute them to Jev, and do not restate one as more certain than its probability.",
	)
	return lines.join("\n")
}
