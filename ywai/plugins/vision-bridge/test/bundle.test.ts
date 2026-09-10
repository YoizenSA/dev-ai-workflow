import { describe, expect, test } from "bun:test"
import { $ } from "bun"
import * as os from "node:os"
import * as path from "node:path"

// Same contract the advisor learned the hard way: v2 validates the default
// export against an object schema and drops a callable one with `Expected
// object at ["default"]`, logging a WARN into its own file. The plugin then
// never loads while every unit test passes, so pin the shape here.
describe("bundle shape", () => {
	test("exports one plugin definition v2 can load", async () => {
		const out = path.join(os.tmpdir(), `vision-bridge-bundle-${Date.now()}.js`)
		await $`bun build ${import.meta.dir}/../src/index.ts --outfile ${out} --target node`.quiet()

		const mod = await import(out)

		expect(Object.keys(mod)).toEqual(["default"])
		expect(typeof mod.default).toBe("object")
		expect(mod.default.id).toBe("ywai-vision-bridge")
		expect(typeof mod.default.setup).toBe("function")
		expect(typeof mod.default.server).toBe("function")
	})
})
