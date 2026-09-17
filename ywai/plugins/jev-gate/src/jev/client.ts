/**
 * Jev (TypeSafe) client - one POST, no SDK dependency.
 *
 * The wire format was read off @typesafe-ai/sdk 0.6.0 rather than guessed:
 * POST https://api.typesafe.ai/v1/systemone, `Authorization: Bearer <key>`,
 * body `{ state, questions, model? }`, questions keyed by name with
 * `{ type, instructions, criteria }`. Vendoring the fetch keeps the plugin
 * bundle self-contained (PLAN 6.7 Q6 allowed exactly this).
 *
 * This file imports nothing from OpenCode or git.
 */
import { JEV_TIMEOUT_MS } from "../domain/config"

const BASE_URL = "https://api.typesafe.ai"

/**
 * `model` is REQUIRED by the API: omitting it returns 422
 * `{"type":"missing","loc":["body","model"]}`. The SDK hides this by always
 * sending its default, so the field is easy to miss when vendoring the fetch.
 * `jev-latest` is that default (@typesafe-ai/sdk 0.6.0), and it resolved to
 * `jev-1.13.0` in the Spike A run.
 */
const DEFAULT_MODEL = "jev-latest"

export type QuestionType = "noul" | "choice" | "score"

export interface Question {
	type: QuestionType
	instructions: unknown
	criteria?: unknown
}

export interface SystemOneRequest {
	state: unknown
	questions: Record<string, Question>
	model?: string
}

/**
 * One answer. Exactly one field is set, matching the question type.
 *
 * Spike A finding: a noul answer carries no `confidence` field - it is
 * `{ type: "noul", noul: 0.98 }` and nothing else. Any gate that wants
 * confidence has to derive it from probability and margin.
 */
export interface Answer {
	type: QuestionType
	noul?: number
	choice?: string
	score?: number
	probabilities?: Record<string, number>
}

export interface SystemOneResponse {
	model: string
	answers: Record<string, Answer>
	usage?: { input_tokens?: number; output_tokens?: number }
}

export interface JevClient {
	systemOne(request: SystemOneRequest): Promise<SystemOneResponse>
}

export class JevError extends Error {
	constructor(
		message: string,
		readonly status?: number,
	) {
		super(message)
		this.name = "JevError"
	}
}

/** Thrown when there is no key: review and find fail closed (PLAN 3.1). */
export class MissingKeyError extends JevError {
	constructor(help?: string) {
		super(
			"No Jev API key - review and find refuse to run without one, by design. " +
				(help ?? "Set TYPESAFE_API_KEY in the environment of the OpenCode server."),
		)
		this.name = "MissingKeyError"
	}
}

export interface HttpClientOptions {
	apiKey?: string
	baseUrl?: string
	timeoutMs?: number
	fetchImpl?: typeof fetch
	model?: string
}

export function resolveApiKey(explicit?: string): string {
	const key = explicit ?? process.env.TYPESAFE_API_KEY
	if (!key) throw new MissingKeyError()
	return key
}

export class HttpJevClient implements JevClient {
	readonly #apiKey: string
	readonly #baseUrl: string
	readonly #timeoutMs: number
	readonly #fetch: typeof fetch
	readonly #model: string

	constructor(options: HttpClientOptions = {}) {
		this.#apiKey = resolveApiKey(options.apiKey)
		this.#baseUrl = (options.baseUrl ?? BASE_URL).replace(/\/+$/, "")
		this.#timeoutMs = options.timeoutMs ?? JEV_TIMEOUT_MS
		this.#fetch = options.fetchImpl ?? globalThis.fetch
		this.#model = options.model ?? process.env.TYPESAFE_DEFAULT_MODEL ?? DEFAULT_MODEL
	}

	async systemOne(request: SystemOneRequest): Promise<SystemOneResponse> {
		if (Object.keys(request.questions).length === 0) {
			throw new JevError("At least one question is required.")
		}

		const controller = new AbortController()
		const timer = setTimeout(() => controller.abort(), this.#timeoutMs)
		try {
			const response = await this.#fetch(`${this.#baseUrl}/v1/systemone`, {
				method: "POST",
				headers: {
					Authorization: `Bearer ${this.#apiKey}`,
					"Content-Type": "application/json",
				},
				body: JSON.stringify({ model: this.#model, ...request }),
				signal: controller.signal,
			})
			const text = await response.text()
			if (!response.ok) {
				throw new JevError(
					`Jev request failed (${response.status}): ${text.slice(0, 300)}`,
					response.status,
				)
			}
			return JSON.parse(text) as SystemOneResponse
		} catch (err) {
			if (err instanceof JevError) throw err
			if ((err as { name?: string })?.name === "AbortError") {
				throw new JevError(`Jev request timed out after ${this.#timeoutMs}ms`)
			}
			throw new JevError(`Jev request failed: ${String(err)}`)
		} finally {
			clearTimeout(timer)
		}
	}
}

/** Read one probability, refusing to invent a number when the answer is absent. */
export function noulOf(answers: Record<string, Answer>, name: string): number | undefined {
	const value = answers[name]?.noul
	return typeof value === "number" ? value : undefined
}
