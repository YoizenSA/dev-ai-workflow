/**
 * Report to markdown (PLAN 4 / 8).
 *
 * The agent gets a short `summary_md`, never the 40 KB of JSON - that goes to
 * storage under the runId. Every line here is traceable to something Jev
 * answered: no line in this file may describe a problem Jev did not mark.
 */
import { DIMENSIONS, LOCATABLE_DIMENSIONS, SCREEN_THRESHOLD } from "./domain/config"
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
	// A run that screened a locatable signal and could not place it is not a
	// clean run: Jev saw something and the pipeline lost it. Saying CLEAN there
	// is the one output that reads as an all-clear when nobody checked.
	const verdict =
		report.action === "clean" && report.unplaced > 0
			? "INCONCLUSIVE"
			: {
					request_changes: "REQUEST CHANGES",
					comment: "COMMENT",
					clean: "CLEAN",
				}[report.action]

	const fileCount = Object.keys(report.matrix).length
	lines.push(
		`**Jev review - ${verdict}** (${report.findings.length} finding(s) across ${fileCount} file(s), ${(report.latencyMs / 1000).toFixed(1)}s, ${report.usage.requests} requests)`,
	)

	if (report.findings.length === 0) {
		lines.push(
			report.unplaced > 0
				? `\n${report.unplaced} signal(s) screened over threshold but could not be placed on a changed line, so none became a finding. This is not a clean result - it is an unfinished one.`
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

/**
 * The screen matrix, which is where Jev's categorisation actually lives.
 *
 * The summary only shows what became a finding, so a run that scored 0.83 on
 * correctness and could not place it looks identical to one that scored 0.02.
 * This prints the scores behind the verdict: every file against every
 * dimension, with the two that can never open a finding marked as such.
 */
export function detail(report: ReviewReport): string {
	const lines: string[] = [
		`**Jev run \`${report.runId}\`** - action: ${report.action}, ` +
			`${report.findings.length} finding(s), ${report.unplaced} unplaced, ` +
			`threshold ${SCREEN_THRESHOLD}, questions ${report.questionsVersion}`,
	]

	const files = Object.keys(report.matrix).sort()
	if (files.length === 0) {
		lines.push("\nNo file was screened: nothing reached Jev.")
	} else {
		lines.push(`\n| file | ${DIMENSIONS.join(" | ")} |`)
		lines.push(`| --- | ${DIMENSIONS.map(() => "---").join(" | ")} |`)
		for (const file of files) {
			const scores = report.matrix[file]
			const cells = DIMENSIONS.map((dimension) => {
				const value = scores[dimension]
				if (value === undefined) return "-"
				// A score at or over the threshold only means something for a
				// dimension allowed to open a finding; mark the rest so the
				// number is not read as a missed defect.
				const over = value >= SCREEN_THRESHOLD
				const locatable = LOCATABLE_DIMENSIONS.includes(dimension)
				return over && locatable ? `**${pct(value)}**` : pct(value)
			})
			lines.push(`| \`${file}\` | ${cells.join(" | ")} |`)
		}
		lines.push(
			`\nBold = at or over the ${SCREEN_THRESHOLD} threshold on a dimension that can open a finding ` +
				`(${LOCATABLE_DIMENSIONS.join(", ")}). ` +
				`\`compatibility\` and \`testGap\` are screened and shown but never promoted: ` +
				"both score high on almost everything, so a high number there is noise, not a defect.",
		)
	}

	if (report.findings.length > 0) {
		lines.push("\nFindings opened from those scores:")
		for (const finding of report.findings) {
			lines.push(
				`- \`${finding.file}:${finding.startLine}-${finding.endLine}\` ${finding.dimension} ` +
					`${pct(finding.probability)}, severity ${sev(finding.severity)}, mechanism \`${finding.mechanism}\``,
			)
		}
	}

	if (report.unplaced > 0) {
		lines.push(
			`\n${report.unplaced} signal(s) crossed the threshold but could not be placed on a changed ` +
				"line. They are in the table above, not in the findings.",
		)
	}

	if (report.skipped.length > 0) {
		lines.push(
			`\nNot screened: ${report.skipped.map((s) => `\`${s.path}\` (${s.reason})`).join("; ")}`,
		)
	}

	return lines.join("\n")
}
