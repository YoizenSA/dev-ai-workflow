import { describe, expect, test } from "bun:test"
import { deliverTerminalGlance, noticeFromTerminal, postWristNotice, wristTarget } from "../src/plugin/wrist-notice"
import type { BackgroundAgentsTerminalEvent } from "../src/plugin/terminal-events"

describe("noticeFromTerminal", () => {
	test("a finished delegation is one done glance", () => {
		expect(
			noticeFromTerminal({
				kind: "terminal",
				delegationID: "dew-pine",
				agent: "dev",
				status: "complete",
				sessionID: "ses_1",
				parentSessionID: "ses_parent",
				remaining: 0,
				title: "fix login",
			}),
		).toEqual({ kind: "done", agent: "dev", label: "fix login", remaining: undefined, detail: undefined })
	})

	test("an error keeps a one-line reason", () => {
		const notice = noticeFromTerminal({
			kind: "terminal",
			delegationID: "dew-pine",
			agent: "qa",
			status: "error",
			sessionID: "ses_1",
			parentSessionID: "ses_parent",
			remaining: 2,
			error: "model key missing",
		})
		expect(notice).toEqual({
			kind: "failed",
			agent: "qa",
			label: "dew-pine",
			detail: "model key missing",
			remaining: 2,
		})
	})

	test("a finished delegation keeps what the task was", () => {
		expect(
			noticeFromTerminal({
				kind: "terminal",
				delegationID: "dew-pine",
				agent: "dev",
				status: "complete",
				sessionID: "ses_1",
				parentSessionID: "ses_parent",
				remaining: 0,
				title: "Fix login",
				description: "Add the password check on the form",
			}),
		).toEqual({
			kind: "done",
			agent: "dev",
			label: "Fix login",
			detail: "Add the password check on the form",
			remaining: undefined,
		})
	})

	test("the batch-complete event does not buzz again", () => {
		const event: BackgroundAgentsTerminalEvent = { kind: "all-complete", parentSessionID: "ses_parent", cycle: 1 }
		expect(noticeFromTerminal(event)).toBeNull()
	})

	test("deliver posts the glance and swallows a dead control server", async () => {
		const posted: unknown[] = []
		deliverTerminalGlance(
			{
				kind: "terminal",
				delegationID: "dew-pine",
				agent: "dev",
				status: "timeout",
				sessionID: "ses_1",
				parentSessionID: "ses_parent",
				remaining: 0,
			},
			(notice) => {
				posted.push(notice)
			},
		)
		expect(posted).toEqual([
			{ kind: "failed", agent: "dev", label: "dew-pine", detail: undefined, remaining: undefined },
		])

		const fetchImpl = () => Promise.reject(new Error("connection refused"))
		expect(() =>
			postWristNotice(
				{ kind: "done", agent: "dev", label: "fix login" },
				fetchImpl as unknown as typeof fetch,
				{ enabled: true, url: "http://127.0.0.1:5768" },
			),
		).not.toThrow()
	})

	test("the settings file decides the url, and off posts nowhere", async () => {
		expect(wristTarget({ enabled: false, url: "http://127.0.0.1:9999" })).toBeNull()
		const calls: string[] = []
		const fetchImpl = (input: string | URL | Request) => {
			calls.push(String(input))
			return Promise.resolve(new Response())
		}
		postWristNotice(
			{ kind: "done", agent: "dev", label: "fix login" },
			fetchImpl as unknown as typeof fetch,
			{ enabled: true, url: "http://10.0.0.8:5768/" },
		)
		await new Promise((resolve) => setTimeout(resolve, 10))
		expect(calls).toEqual(["http://10.0.0.8:5768/api/push/notify"])
	})
})
