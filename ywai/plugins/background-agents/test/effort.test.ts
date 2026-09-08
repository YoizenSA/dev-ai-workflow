import { describe, expect, test } from "bun:test"
import { effortToVariant, formatModelRef, parseModelString } from "../src/plugin/types"

describe("effort → model variant", () => {
	// OpenCode expresses reasoning effort as a model variant, so effort only
	// reaches the model if it survives as `#variant` all the way to the prompt.
	test("maps the spellings a supervisor reaches for", () => {
		expect(effortToVariant("high")).toBe("high")
		expect(effortToVariant("max")).toBe("high")
		expect(effortToVariant("minimal")).toBe("low")
		expect(effortToVariant("min")).toBe("low")
		expect(effortToVariant("  MEDIUM  ")).toBe("medium")
	})

	// The variant space belongs to the provider: rejecting unknown values here
	// would block variants we have never heard of.
	test("passes an unrecognised variant through untouched", () => {
		expect(effortToVariant("ultra")).toBe("ultra")
	})

	test("renders the variant the way OpenCode reads it", () => {
		expect(formatModelRef({ providerID: "anthropic", modelID: "claude-opus-5" })).toBe(
			"anthropic/claude-opus-5",
		)
		expect(
			formatModelRef({ providerID: "anthropic", modelID: "claude-opus-5", variant: "high" }),
		).toBe("anthropic/claude-opus-5#high")
	})
})

describe("parseModelString", () => {
	test("keeps an inline variant instead of folding it into the model id", () => {
		expect(parseModelString("anthropic/claude-opus-5#high")).toEqual({
			providerID: "anthropic",
			modelID: "claude-opus-5",
			variant: "high",
		})
	})

	test("leaves a plain reference without a variant", () => {
		expect(parseModelString("anthropic/claude-opus-5")).toEqual({
			providerID: "anthropic",
			modelID: "claude-opus-5",
		})
	})

	// Model ids can contain slashes; only the first segment is the provider.
	test("keeps slashes inside the model id", () => {
		expect(parseModelString("openrouter/meta/llama-4")).toEqual({
			providerID: "openrouter",
			modelID: "meta/llama-4",
		})
	})

	test("rejects references that cannot address a model", () => {
		expect(parseModelString("anthropic")).toBeUndefined()
		expect(parseModelString("anthropic/")).toBeUndefined()
		expect(parseModelString("anthropic/#high")).toBeUndefined()
	})

	test("round-trips through formatModelRef", () => {
		const ref = parseModelString("anthropic/claude-opus-5#high")
		expect(ref && formatModelRef(ref)).toBe("anthropic/claude-opus-5#high")
	})
})
