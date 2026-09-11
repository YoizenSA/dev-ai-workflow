import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import BenchHistory from "./BenchHistory";

// ----- Fixtures mirroring the evals API --------------------------------------

const tasks = {
  tasks: [
    { id: "trace-answer", name: "Trace answer", agent: "ask" },
    { id: "find-runs", name: "Find runs", agent: "dev" },
  ],
};

const run1 = {
  id: "run-1",
  taskId: "trace-answer",
  taskName: "Trace answer",
  agent: "ask",
  provider: "opencode-admin",
  rounds: 1,
  models: ["model-a", "model-b"],
  attempts: [],
  status: "done",
  startedAt: "2026-09-11T10:00:00Z",
};

const run2 = {
  ...run1,
  id: "run-2",
  taskId: "find-runs",
  taskName: "Find runs",
  startedAt: "2026-09-11T11:00:00Z",
};

const run1Detail = {
  ...run1,
  attempts: [
    {
      model: "model-a",
      round: 1,
      score: { hits: ["a", "b", "c"], missed: [], total: 3, answered: true, weighted: 0.8333 },
      metrics: { turns: 7 },
      costUsd: 0.0012,
      costKnown: true,
    },
    {
      model: "model-b",
      round: 1,
      score: { hits: [], missed: ["a", "b", "c"], total: 3, answered: false },
      metrics: { turns: 2 },
      costKnown: false,
    },
  ],
};

const leaderboard = {
  rows: [
    { model: "model-a", runs: 2, avgWeighted: 0.8, trend: [0.5, 0.8] },
    { model: "model-b", runs: 1, avgWeighted: 0.3, trend: [] },
  ],
};

// ----- fetch stub routing on URL ----------------------------------------------

function mockFetchByUrl(routes: Record<string, unknown>) {
  const keys = Object.keys(routes);
  globalThis.fetch = vi.fn(async (input: string | URL | Request) => {
    const url = String(input);
    // Exact matches win so /api/evals/runs does not swallow /api/evals/runs/{id}.
    for (const key of keys) {
      if (url === key) return { ok: true, json: async () => routes[key] };
    }
    for (const key of keys) {
      if (url.startsWith(key)) return { ok: true, json: async () => routes[key] };
    }
    return { ok: false, status: 404, json: async () => ({ error: `no route: ${url}` }) };
  }) as unknown as typeof fetch;
}

beforeEach(() => {
  globalThis.fetch = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

// ------------------------------------------------------------------------------

describe("BenchHistory", () => {
  it("renders the loading state while runs are in flight", () => {
    globalThis.fetch = vi.fn(() => new Promise(() => {})) as unknown as typeof fetch;

    render(<BenchHistory />);

    expect(screen.getByText(/loading history/i)).toBeInTheDocument();
  });

  it("renders the empty state when no runs exist", async () => {
    mockFetchByUrl({
      "/api/evals/tasks": { tasks: [] },
      "/api/evals/runs": { runs: [] },
      "/api/evals/leaderboard": { rows: [] },
    });

    render(<BenchHistory />);

    expect(await screen.findByText(/no benchmark runs yet/i)).toBeInTheDocument();
    expect(screen.queryByText(/loading history/i)).not.toBeInTheDocument();
  });

  it("renders runs, status badges, and the leaderboard trends", async () => {
    mockFetchByUrl({
      "/api/evals/tasks": tasks,
      "/api/evals/runs": { runs: [run1, run2] },
      "/api/evals/leaderboard": leaderboard,
    });

    render(<BenchHistory />);

    expect(await screen.findByText("run-1")).toBeInTheDocument();
    expect(screen.getByText("run-2")).toBeInTheDocument();
    expect(screen.getByText("Trace answer")).toBeInTheDocument();
    expect(screen.getAllByText("done").length).toBeGreaterThan(0);
    // Trends plot only models with trend points.
    expect(screen.getByText("model-a")).toBeInTheDocument();
    expect(screen.queryByText("model-b")).not.toBeInTheDocument();
    expect(document.querySelectorAll("polyline")).toHaveLength(1);
  });

  it("expands a run and loads its attempt detail from the run endpoint", async () => {
    const user = userEvent.setup();
    mockFetchByUrl({
      "/api/evals/runs/run-1": run1Detail,
      "/api/evals/tasks": tasks,
      "/api/evals/runs": { runs: [run1] },
      "/api/evals/leaderboard": { rows: [] },
    });

    render(<BenchHistory />);

    await user.click(await screen.findByRole("button", { name: /run-1/ }));

    // RunCard column set: model, score, weighted, cost, turns.
    expect(await screen.findByText("3/3")).toBeInTheDocument();
    expect(screen.getByText("83%")).toBeInTheDocument();
    expect(screen.getByText("$0.0012")).toBeInTheDocument();
    expect(screen.getByText("no answer")).toBeInTheDocument();
    expect(fetch).toHaveBeenCalledWith("/api/evals/runs/run-1");
  });

  it("filters runs and refetches trends when a task is picked", async () => {
    const user = userEvent.setup();
    mockFetchByUrl({
      "/api/evals/tasks": tasks,
      "/api/evals/runs": { runs: [run1, run2] },
      "/api/evals/leaderboard": leaderboard,
    });

    render(<BenchHistory />);

    await screen.findByText("run-1");
    await user.selectOptions(screen.getByRole("combobox"), "find-runs");

    await waitFor(() => {
      expect(fetch).toHaveBeenCalledWith("/api/evals/leaderboard?taskId=find-runs");
    });
    expect(screen.queryByText("run-1")).not.toBeInTheDocument();
    expect(screen.getByText("run-2")).toBeInTheDocument();
    expect(screen.getByText("1 run(s)")).toBeInTheDocument();
  });
});
