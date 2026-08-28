import { expect, test } from "bun:test"

// Mirrors aliasTool in src/index.ts. Kept here so the naming contract is
// checked without booting the plugin runtime.
function alias(name: string): string {
  return name === "delegate" ? "subagent" : name.replace(/^delegation_/, "subagent_")
}

test("delegate becomes the v2 subagent name", () => {
  expect(alias("delegate")).toBe("subagent")
})

test("supervision tools keep their verb", () => {
  expect(alias("delegation_peek")).toBe("subagent_peek")
  expect(alias("delegation_steer")).toBe("subagent_steer")
  expect(alias("delegation_stop")).toBe("subagent_stop")
  expect(alias("delegation_read")).toBe("subagent_read")
})

test("an already-aliased name is left alone, so aliasing twice is safe", () => {
  expect(alias(alias("delegation_peek"))).toBe("subagent_peek")
})
