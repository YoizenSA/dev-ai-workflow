import { mkdtemp, readFile, rm, stat } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";

// ADAPTED for the ywai vendored copy (bun test): upstream calls
// vi.useFakeTimers() + vi.setSystemTime(), but bun:test's `vi` has no
// setSystemTime and its fake timers can only advance forward, while the
// frozen instants used by the tests lie in the past. We freeze Date
// deterministically with a patched subclass instead; test/setup.ts calls
// useRealTime() in afterEach to restore it.

const tempDirs = new Set<string>();

export async function createRuntimeHarness(options: { preserveState?: boolean } = {}) {
  const dir = await mkdtemp(join(tmpdir(), "subagent-statusline-test-"));
  tempDirs.add(dir);

  const statePath = join(dir, "state.json");
  const textPath = join(dir, "status.txt");
  process.env.OPENCODE_SUBAGENT_STATUSLINE_STATE = statePath;
  process.env.OPENCODE_SUBAGENT_STATUSLINE_PRESERVE_STATE = options.preserveState
    ? "1"
    : "0";
  process.env.NO_COLOR = "1";

  return { dir, statePath, textPath };
}

export async function cleanupRegisteredTempDirs(): Promise<void> {
  await Promise.all(
    [...tempDirs].map(async (dir) => {
      await rm(dir, { force: true, recursive: true });
      tempDirs.delete(dir);
    }),
  );
}

export async function readJsonFixture<T>(name: string): Promise<T> {
  const url = new URL(`../fixtures/events/${name}.json`, import.meta.url);
  return JSON.parse(await readFile(url, "utf8")) as T;
}

export async function readRuntimeState<T>(statePath: string): Promise<T> {
  return JSON.parse(await readFile(statePath, "utf8")) as T;
}

export async function readStatusText(textPath: string): Promise<string> {
  return readFile(textPath, "utf8");
}

export async function pathExists(path: string): Promise<boolean> {
  try {
    await stat(path);
    return true;
  } catch {
    return false;
  }
}

type DateConstructor = typeof Date;
const RealDate: DateConstructor = Date;
let frozenTimeMs: number | null = null;

class FrozenDate extends RealDate {
  constructor();
  constructor(value: number | string);
  constructor(
    year: number,
    month: number,
    ...rest: unknown[]
  );
  constructor(...args: unknown[]) {
    if (args.length === 0) {
      super(frozenTimeMs !== null ? frozenTimeMs : RealDate.now());
    } else {
      super(...(args as ConstructorParameters<DateConstructor>));
    }
  }

  static now(): number {
    return frozenTimeMs !== null ? frozenTimeMs : RealDate.now();
  }
}

export function useFrozenTime(isoTimestamp: string): Date {
  const now = new RealDate(isoTimestamp);
  if (globalThis.Date === RealDate) {
    globalThis.Date = FrozenDate as DateConstructor;
  }
  frozenTimeMs = now.getTime();
  return now;
}

export function useRealTime(): void {
  frozenTimeMs = null;
  if (globalThis.Date !== RealDate) {
    globalThis.Date = RealDate;
  }
}
