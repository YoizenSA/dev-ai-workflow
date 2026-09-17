/**
 * Fase 6 eval (PLAN 9).
 *
 * Runs the screen step over every labeled fixture and compares the
 * probabilities against the human labels. It is deliberately NOT an accuracy
 * benchmark: what it hunts is embarrassing failures - security firing on the
 * word "token", a clean diff scoring high, a manipulative comment suppressing
 * a real finding.
 *
 * Results are written under the QUESTIONS_VERSION they were produced with, so
 * two runs are only ever compared when they asked the same questions.
 *
 * Usage:
 *   TYPESAFE_API_KEY=... bun scripts/eval-fixtures.mjs [--threshold 0.7]
 */
import { readFileSync, writeFileSync, mkdirSync } from "node:fs"
import { join } from "node:path"

const PLUGIN = join(import.meta.dir, "../../../plugins/jev-gate/src")
const { HttpJevClient } = await import(join(PLUGIN, "jev/client.ts"))
const { QUESTIONS_VERSION } = await import(join(PLUGIN, "jev/questions.ts"))
const { screenFile, emptyUsage } = await import(join(PLUGIN, "review/judgments.ts"))
const { DIMENSIONS } = await import(join(PLUGIN, "domain/config.ts"))

const thresholdArg = process.argv.indexOf("--threshold")
const THRESHOLD = thresholdArg > -1 ? Number(process.argv[thresholdArg + 1]) : 0.7

const fixturesDir = join(import.meta.dir, "../fixtures")
const { fixtures } = JSON.parse(readFileSync(join(fixturesDir, "labels.json"), "utf8"))

const client = new HttpJevClient()
const usage = emptyUsage()
const rows = []

for (const fixture of fixtures) {
	const patch = readFileSync(join(fixturesDir, fixture.file), "utf8")
	const started = Date.now()
	const scores = await screenFile(client, fixture.path, patch, usage)
	const raised = DIMENSIONS.filter((d) => (scores[d] ?? 0) >= THRESHOLD)

	const expected = fixture.expect
	const hit = expected.length > 0 && expected.some((d) => raised.includes(d))
	const missed = expected.filter((d) => !raised.includes(d))
	// A dimension raised on a fixture labeled clean, or one nobody expected on
	// a dirty fixture, is the noise this eval exists to find.
	const spurious = raised.filter((d) => !expected.includes(d))

	rows.push({
		file: fixture.file,
		label: fixture.label,
		expected,
		raised,
		hit,
		missed,
		spurious,
		scores,
		ms: Date.now() - started,
	})

	const verdict = expected.length === 0 ? (raised.length === 0 ? "OK" : "NOISE") : hit ? "OK" : "MISS"
	console.log(
		`${verdict.padEnd(5)} ${fixture.file.padEnd(28)} raised=[${raised.join(",")}] expected=[${expected.join(",")}]`,
	)
}

const dirty = rows.filter((r) => r.expected.length > 0)
const clean = rows.filter((r) => r.expected.length === 0)
const caught = dirty.filter((r) => r.hit).length
const falseAlarms = clean.filter((r) => r.raised.length > 0).length

// How often each dimension fires where nobody expected it: the number that
// tells you which question needs rewriting.
const spuriousByDimension = {}
for (const row of rows) {
	for (const dimension of row.spurious) {
		spuriousByDimension[dimension] = (spuriousByDimension[dimension] ?? 0) + 1
	}
}

const summary = {
	questionsVersion: QUESTIONS_VERSION,
	threshold: THRESHOLD,
	at: new Date().toISOString(),
	dirtyCaught: `${caught}/${dirty.length}`,
	cleanFixturesWithFindings: `${falseAlarms}/${clean.length}`,
	spuriousByDimension,
	usage,
	rows,
}

console.log("\n" + JSON.stringify({ ...summary, rows: undefined }, null, 2))

const outDir = join(import.meta.dir, "../results")
mkdirSync(outDir, { recursive: true })
const out = join(outDir, `eval-${QUESTIONS_VERSION}-t${THRESHOLD}.json`)
writeFileSync(out, JSON.stringify(summary, null, 2))
console.log(`\nwrote ${out}`)
