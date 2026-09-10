// Type-only alias: tests import from "vitest", while this repo runs the
// suite with `bun test` (bun maps the "vitest" module specifier to its own
// bun:test implementation at runtime). This shim gives tsc the same surface
// for typechecking without adding a dependency.
declare module "vitest" {
  export * from "bun:test";
}
