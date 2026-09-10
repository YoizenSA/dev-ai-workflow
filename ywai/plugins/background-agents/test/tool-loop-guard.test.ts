import { describe, expect, test } from "bun:test"
import { createToolLoopGuardHook, fingerprint, LOOP_GUARD_WARNING } from "../src/plugin/tool-loop-guard"

function makeHooks() {
	return createToolLoopGuardHook()
}

async function call(hooks: ReturnType<typeof makeHooks>, tool: string, args: any, output: any, callID: string) {
	const beforeOutput = { args }
	await hooks["tool.execute.before"]({ tool, sessionID: "s1", callID }, beforeOutput)
	const afterOutput = { output }
	await hooks["tool.execute.after"]({ tool, sessionID: "s1", callID }, afterOutput)
	return afterOutput.output
}

describe("fingerprint", () => {
	test("is insensitive to key order", () => {
		expect(fingerprint("read", { path: "/a", limit: 5 })).toBe(fingerprint("read", { limit: 5, path: "/a" }))
	})

	test("distinguishes different tools and args", () => {
		expect(fingerprint("read", { path: "/a" })).not.toBe(fingerprint("grep", { path: "/a" }))
		expect(fingerprint("read", { path: "/a" })).not.toBe(fingerprint("read", { path: "/b" }))
	})
})

describe("tool loop guard", () => {
	test("lets distinct calls through untouched", async () => {
		const hooks = makeHooks()
		let n = 0
		for (let i = 0; i < 5; i++) {
			const out = await call(hooks, "read", { path: "/a" }, `result ${i}`, `c${i}`)
			n = i
			expect(out).toBe(`result ${n}`)
		}
	})

	test("warns after 3 identical calls (same args, same result)", async () => {
		const hooks = makeHooks()
		for (let i = 0; i < 3; i++) {
			const out = await call(hooks, "read", { path: "/a" }, "same", `c${i}`)
			if (i < 2) {
				expect(out).toBe("same")
			} else {
				expect(out).toContain(LOOP_GUARD_WARNING)
			}
		}
	})

	test("resets the run when the result changes (progress)", async () => {
		const hooks = makeHooks()
		// Two identical pairs, then a different result — run restarts.
		await call(hooks, "read", { path: "/a" }, "same", "c1")
		await call(hooks, "read", { path: "/a" }, "same", "c2")
		const out = await call(hooks, "read", { path: "/a" }, "changed", "c3")
		expect(out).toBe("changed")
		const out2 = await call(hooks, "read", { path: "/a" }, "changed", "c4")
		expect(out2).toBe("changed") // runs=2, still below warn
	})

	test("blocks the 6th identical read-only call", async () => {
		const hooks = makeHooks()
		for (let i = 0; i < 5; i++) {
			await call(hooks, "read", { path: "/a" }, "same", `c${i}`)
		}
		// The 6th identical call is refused in before.
		expect(
			hooks["tool.execute.before"]({ tool: "read", sessionID: "s1", callID: "c6" }, { args: { path: "/a" } }),
		).rejects.toThrow(/infinite loop/)
	})

	test("never blocks write tools, only warns", async () => {
		const hooks = makeHooks()
		for (let i = 0; i < 8; i++) {
			const out = await call(hooks, "edit", { path: "/a" }, "same", `c${i}`)
			// No throw, warnings after run 3.
			expect(out).toContain(i >= 2 ? "REPEATED TOOL CALLS" : "same")
		}
	})

	test("exempts delegation tools from tracking", async () => {
		const hooks = makeHooks()
		for (let i = 0; i < 6; i++) {
			const out = await call(hooks, "delegation_status", {}, "same", `c${i}`)
			expect(out).toBe("same")
		}
	})

	test("observeNewUserMessage clears per-turn state", async () => {
		const hooks = makeHooks()
		await call(hooks, "read", { path: "/a" }, "same", "c1")
		await call(hooks, "read", { path: "/a" }, "same", "c2")
		hooks.observeNewUserMessage("s1", "m2")
		const out = await call(hooks, "read", { path: "/a" }, "same", "c3")
		expect(out).toBe("same")
	})

	test("different sessions are tracked independently", async () => {
		const hooks = makeHooks()
		for (let i = 0; i < 4; i++) {
			await hooks["tool.execute.before"]({ tool: "read", sessionID: "s1", callID: `c${i}` }, { args: { path: "/a" } })
			await hooks["tool.execute.after"]({ tool: "read", sessionID: "s1", callID: `c${i}` }, { output: "same" })
		}
		// s2 doing the same call once is fresh.
		const beforeOutput = { args: { path: "/a" } }
		await hooks["tool.execute.before"]({ tool: "read", sessionID: "s2", callID: "x1" }, beforeOutput)
		const afterOutput = { output: "same" }
		await hooks["tool.execute.after"]({ tool: "read", sessionID: "s2", callID: "x1" }, afterOutput)
		expect(afterOutput.output).toBe("same")
	})
})
