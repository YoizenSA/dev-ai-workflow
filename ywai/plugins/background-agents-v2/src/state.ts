/**
 * Delegation record persistence. One JSON file per delegation under
 * ~/.local/share/opencode/delegations/<projectKey>/ — survives restarts so
 * `delegation_read` can still serve completed output and the record list
 * stays honest after a crash.
 *
 * The on-disk shape mirrors the v1 plugin's format where it matters
 * (id/status/agent/prompt/result) so historical records stay readable.
 */

import * as fs from "node:fs/promises"
import * as path from "node:path"

const STATE_VERSION = 2

export type DelegationStatus =
	| "running"
	| "completed"
	| "failed"
	| "interrupted"
	| "timeout"
	| "stopped"

export interface ModelRef {
	providerID: string
	id: string
	/** Model variant — v2's effort selector ("high" | "medium" | "low" | custom). */
	variant?: string
}

export interface DelegationRecord {
	id: string
	parentSessionID: string
	parentMessageID?: string
	childSessionID: string
	agent: string
	model?: ModelRef
	mode: "sync" | "async"
	prompt: string
	title?: string
	status: DelegationStatus
	createdAt: number
	startedAt: number
	completedAt?: number
	lastActivityAt: number
	/** Epoch ms deadline; 0 = no deadline (supervisor steers/stops manually). */
	deadlineAt: number
	steerCount: number
	toolCalls: number
	result?: string
	error?: string
	notified: boolean
}

interface StateFile {
	version: number
	delegation: DelegationRecord
}

function serializeDelegation(record: DelegationRecord): string {
	const file: StateFile = { version: STATE_VERSION, delegation: record }
	return JSON.stringify(file, null, "\t")
}

function deserializeDelegation(json: string): DelegationRecord | undefined {
	try {
		const parsed = JSON.parse(json) as StateFile
		if (!parsed?.delegation?.id) return undefined
		return parsed.delegation
	} catch {
		return undefined
	}
}

/** Records dir for a project key; created on demand. */
async function ensureDir(recordsDir: string): Promise<void> {
	await fs.mkdir(recordsDir, { recursive: true })
}

async function writeFile(recordsDir: string, record: DelegationRecord): Promise<void> {
	await ensureDir(recordsDir)
	await fs.writeFile(
		path.join(recordsDir, `${record.id}.json`),
		serializeDelegation(record),
		"utf8",
	)
}

async function removeFile(recordsDir: string, id: string): Promise<void> {
	await fs.rm(path.join(recordsDir, `${id}.json`), { force: true })
}

/** Lists every persisted record for the dir, skipping corrupt files. */
async function listRecords(recordsDir: string): Promise<DelegationRecord[]> {
	let entries: string[]
	try {
		entries = await fs.readdir(recordsDir)
	} catch {
		return []
	}
	const out: DelegationRecord[] = []
	for (const name of entries) {
		if (!name.endsWith(".json")) continue
		try {
			const raw = await fs.readFile(path.join(recordsDir, name), "utf8")
			const record = deserializeDelegation(raw)
			if (record) out.push(record)
		} catch {
			// unreadable file: skip, never fail the listing
		}
	}
	out.sort((a, b) => b.startedAt - a.startedAt)
	return out
}

export {
	STATE_VERSION,
	serializeDelegation,
	deserializeDelegation,
	listRecords,
	removeFile as removeDelegationFile,
	writeFile as writeDelegationFile,
}
