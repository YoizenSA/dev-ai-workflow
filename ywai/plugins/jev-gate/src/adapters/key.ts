/**
 * Where the Jev key comes from (PLAN 3.1, corrected by a Fase 2 measurement).
 *
 * The plan assumed `TYPESAFE_API_KEY` in the environment would be enough. It
 * is not: on v2.0.6 plugins run inside a long-lived server process that does
 * NOT inherit the environment of the `opencode2` invocation (measured: a
 * plugin probe saw `hasKey: false` and `cwd` = the user's home, with the key
 * exported in the calling shell). So the env stays supported - it works when
 * the server itself was started with the key - and a key file is the path
 * that actually works from a terminal.
 *
 * Read order, first hit wins:
 *   1. TYPESAFE_API_KEY in the plugin process
 *   2. <project>/.opencode/jev-gate.json   (per project; git-ignore it)
 *   3. ~/.config/opencode/jev-gate.json    (per user)
 *   4. ~/.ywai/jev-gate.json               (ywai-managed)
 *
 * The file is `{ "apiKey": "..." }`. Nothing here ever logs or returns the key
 * in an error message.
 */
import { readFileSync } from "node:fs"
import { homedir } from "node:os"
import { join } from "node:path"

export const KEY_FILE_NAME = "jev-gate.json"

export function keyFileCandidates(projectDir?: string): string[] {
	const home = homedir()
	const candidates: string[] = []
	if (projectDir) candidates.push(join(projectDir, ".opencode", KEY_FILE_NAME))
	candidates.push(join(home, ".config", "opencode", KEY_FILE_NAME))
	candidates.push(join(home, ".ywai", KEY_FILE_NAME))
	return candidates
}

function readKeyFile(path: string): string | undefined {
	try {
		const parsed = JSON.parse(readFileSync(path, "utf8")) as { apiKey?: unknown }
		return typeof parsed.apiKey === "string" && parsed.apiKey.length > 0
			? parsed.apiKey
			: undefined
	} catch {
		// Missing or malformed: try the next candidate. A bad file must not
		// crash plugin load, and must not leak its contents either.
		return undefined
	}
}

/** The key, or undefined. Callers decide how to refuse (they all fail closed). */
export function findApiKey(projectDir?: string): string | undefined {
	const fromEnv = process.env.TYPESAFE_API_KEY
	if (fromEnv) return fromEnv
	for (const path of keyFileCandidates(projectDir)) {
		const key = readKeyFile(path)
		if (key) return key
	}
	return undefined
}

/** Where to tell the user to put a key, without printing any key. */
export function keyHelp(projectDir?: string): string {
	return (
		"Set TYPESAFE_API_KEY in the environment that starts the OpenCode server, " +
		`or write {"apiKey": "..."} to one of: ${keyFileCandidates(projectDir).join(", ")}`
	)
}
