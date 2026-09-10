import { describe, expect, test } from "bun:test"
import { createJsonErrorRecoveryHook, JSON_ERROR_REMINDER } from "../src/plugin/json-error-recovery"

function makeHook() {
	return createJsonErrorRecoveryHook()
}

async function run(tool: string, outputText: unknown) {
	const hook = makeHook()
	const output = { output: outputText }
	await hook["tool.execute.after"]({ tool, sessionID: "s1", callID: "c1" }, output)
	return output.output
}

describe("json-error-recovery", () => {
	test("appends the reminder on a JSON parse error", async () => {
		const out = await run("edit", "JSON parse error: expected '}' in JSON body")
		expect(out).toContain(JSON_ERROR_REMINDER)
	})

	test("appends the reminder on unexpected end of JSON input", async () => {
		const out = await run("write", "Unexpected end of JSON input")
		expect(out).toContain(JSON_ERROR_REMINDER)
	})

	test("appends the reminder on a SyntaxError token message", async () => {
		const out = await run("edit", "SyntaxError: unexpected token '}' in JSON at position 42")
		expect(out).toContain(JSON_ERROR_REMINDER)
	})

	test("leaves normal output untouched", async () => {
		const out = await run("edit", "File updated successfully")
		expect(out).toBe("File updated successfully")
	})

	test("skips excluded tools even on JSON-looking errors", async () => {
		const out = await run("bash", "JSON parse error: expected '}'")
		expect(out).toBe("JSON parse error: expected '}'")
	})

	test("does not duplicate the reminder on repeated failures", async () => {
		const first = await run("edit", "JSON parse error: expected '}'")
		const hook = makeHook()
		const output = { output: first }
		await hook["tool.execute.after"]({ tool: "edit", sessionID: "s1", callID: "c2" }, output)
		expect((output.output as string).split("[JSON PARSE ERROR").length - 1).toBe(1)
	})

	test("ignores non-string outputs", async () => {
		const out = await run("edit", { error: "JSON parse error: expected '}'" })
		expect(out).toEqual({ error: "JSON parse error: expected '}'" })
	})
})
