// Upstream: test/setup.ts from opencode-subagent-statusline @ 070fd66.
// Adapted for the ywai vendored copy: upstream relies on vitest's
// `globals: true` for `afterEach`/`vi`; this repo runs the suite with
// `bun test` (see bunfig.toml preload), so the hooks are imported from
// `bun:test` explicitly. Everything else is upstream logic unchanged.
import { afterEach, vi } from "bun:test";
import {
  cleanupRegisteredTempDirs,
  useRealTime,
} from "./helpers/runtime-harness.js";

const envKeys = [
  "NO_COLOR",
  "OPENCODE_SUBAGENT_STATUSLINE_COLOR",
  "OPENCODE_SUBAGENT_STATUSLINE_INSTANCE",
  "OPENCODE_SUBAGENT_STATUSLINE_PRESERVE_STATE",
  "OPENCODE_SUBAGENT_STATUSLINE_STATE",
  "XDG_RUNTIME_DIR",
];

const originalEnv = new Map(
  envKeys.map((key) => [key, process.env[key]]),
);

afterEach(async () => {
  vi.useRealTimers();
  vi.restoreAllMocks();
  vi.clearAllMocks();
  useRealTime();

  for (const key of envKeys) {
    const original = originalEnv.get(key);
    if (original === undefined) {
      delete process.env[key];
    } else {
      process.env[key] = original;
    }
  }

  await cleanupRegisteredTempDirs();
});
