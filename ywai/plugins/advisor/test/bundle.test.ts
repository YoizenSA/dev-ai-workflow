import { describe, expect, test } from "bun:test"
import { $ } from "bun"
import * as os from "node:os"
import * as path from "node:path"

// OpenCode loads a plugin by importing the bundle and validating its default
// export, and it rejects the whole module — silently, into its own log file —
// when the shape is wrong. v1 wanted a callable factory ("Plugin export is not
// a function"); v2 wants an object carrying id and setup ("Expected object at
// [\"default\"]"). Either way the plugin is dropped while every unit test
// still passes.
//
// Both shapes have shipped broken here: first extra exports from
// `export * from "./emission-guard"`, then the callable form under v2. Only
// running it inside OpenCode ever showed it, so the shape is pinned here.
describe("bundle shape", () => {
  test("exports one plugin definition v2 can load", async () => {
    const out = path.join(os.tmpdir(), `advisor-bundle-${Date.now()}.js`)
    await $`bun build ${import.meta.dir}/../src/index.ts --outfile ${out} --target node`.quiet()

    const mod = await import(out)
    const names = Object.keys(mod)

    expect(names).toEqual(["default"])

    // v2 validates the default export against an object schema: a callable
    // export is rejected with `Expected object at ["default"]` and the plugin
    // is dropped behind a WARN nobody reads. This assertion used to demand the
    // callable form, which is precisely how the advisor shipped never loading.
    expect(typeof mod.default).toBe("object")
    expect(mod.default.id).toBe("ywai-advisor")
    expect(typeof mod.default.setup).toBe("function")

    // v1 reaches the same plugin through server().
    const hooks = await mod.default.server(
      { client: {}, directory: os.tmpdir() },
      { configPath: path.join(os.tmpdir(), "definitely-absent.yaml") },
    )
    expect(hooks).toBeDefined()
  })
})
