/**
 * Readable delegation ids: "adjective-noun-Animal" style, dependency-free
 * (the v1 plugin used unique-names-generator; the v2 bundle inlines nothing
 * but node builtins, so the word lists live here).
 */

const ADJECTIVES = [
	"swift", "calm", "bright", "quiet", "bold", "clever", "eager", "gentle",
	"keen", "lucid", "mellow", "noble", "polite", "rapid", "subtle", "tidy",
	"vivid", "warm", "witty", "zesty", "brave", "crisp", "deft", "smart",
] as const

const NOUNS = [
	"otter", "falcon", "cedar", "harbor", "juniper", "lagoon", "meadow", "pebble",
	"quartz", "river", "summit", "thistle", "umbra", "violet", "willow", "zenith",
	"amber", "basalt", "clover", "dune", "ember", "fjord", "glacier", "iris",
] as const

const ANIMALS = [
	"tiger", "panda", "heron", "lynx", "orca", "ibex", "falcon", "badger",
	"crow", "elk", "fox", "gecko", "hare", "koala", "lemur", "marten",
] as const

function pick<T>(list: readonly T[]): T {
	return list[Math.floor(Math.random() * list.length)]
}

/** Collisions are broken by the caller appending a counter. */
function generateReadableId(): string {
	return `${pick(ADJECTIVES)}-${pick(NOUNS)}-${pick(ANIMALS)}`
}

function generateUniqueId(taken: (id: string) => boolean): string {
	for (let attempt = 0; attempt < 8; attempt++) {
		const id = generateReadableId()
		if (!taken(id)) return id
	}
	// Fall back to a suffixed id after repeated collisions.
	let n = 2
	let base = generateReadableId()
	while (taken(`${base}-${n}`)) n++
	return `${base}-${n}`
}

export { generateReadableId, generateUniqueId }
