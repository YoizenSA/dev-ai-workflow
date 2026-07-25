import { describe, expect, test } from "bun:test"
import { $ } from "bun"
import * as os from "node:os"
import * as path from "node:path"

// OpenCode loads a plugin by importing the bundle and calling its export. It
// rejects the whole module — silently, into its own log file — when an export
// is not a callable plugin factory: "Plugin export is not a function".
//
// This cost a full debugging session. Re-exporting the helpers from index.ts
// (`export * from "./emission-guard"`) shipped 20 exports including classes,
// and the plugin never loaded while every unit test still passed. Nothing but
// running it inside OpenCode showed the failure, so it is pinned here.
describe("bundle shape", () => {
  test("exports exactly one callable plugin", async () => {
    const out = path.join(os.tmpdir(), `advisor-bundle-${Date.now()}.js`)
    await $`bun build ${import.meta.dir}/../src/index.ts --outfile ${out} --target node`.quiet()

    const mod = await import(out)
    const names = Object.keys(mod)

    expect(names).toEqual(["default"])
    expect(typeof mod.default).toBe("function")

    // A class would satisfy `typeof === "function"` and still throw when the
    // loader calls it without `new`.
    const hooks = await mod.default(
      { client: {}, directory: os.tmpdir() },
      { configPath: path.join(os.tmpdir(), "definitely-absent.yaml") },
    )
    expect(hooks).toBeDefined()
  })
})
