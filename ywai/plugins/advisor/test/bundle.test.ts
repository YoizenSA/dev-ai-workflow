import { describe, expect, test } from "bun:test"
import { $ } from "bun"
import * as os from "node:os"
import * as path from "node:path"

// OpenCode v2 loads a plugin by importing the bundle and reading { id, setup }.
describe("bundle shape", () => {
  test("exports exactly one v2 plugin object", async () => {
    const out = path.join(os.tmpdir(), `advisor-bundle-${Date.now()}.js`)
    await $`bun build ${import.meta.dir}/../src/index.ts --outfile ${out} --target node`.quiet()

    const mod = await import(out)
    const names = Object.keys(mod)

    expect(names).toEqual(["default"])
    expect(typeof mod.default).toBe("object")
    expect(mod.default.id).toBe("advisor")
    expect(typeof mod.default.setup).toBe("function")

    await mod.default.setup({
      directory: os.tmpdir(),
      options: { configPath: path.join(os.tmpdir(), "definitely-absent.yaml") },
      tool: { transform: async () => {} },
      event: { subscribe: async () => {} },
      session: { hook: async () => {} },
    })
  })
})
