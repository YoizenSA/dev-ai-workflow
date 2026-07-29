import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, Play } from "lucide-react";

interface Expectation {
  needle: string;
  label: string;
  hard?: boolean;
}

interface Task {
  id: string;
  name: string;
  agent: string;
  description?: string;
  brief: string;
  expect: Expectation[];
}

interface Score {
  hits: string[];
  missed: string[];
  total: number;
  gotHard: boolean;
  answered: boolean;
}

interface Metrics {
  turns: number;
  calls: number;
  reads: number;
  codegraph: number;
  invalid: number;
  worstFileReads: number;
  tokensInput: number;
  tokensOutput: number;
}

interface Attempt {
  model: string;
  round: number;
  sessionId: string;
  seconds: number;
  score: Score;
  metrics: Metrics;
  response?: string;
  error?: string;
}

interface Run {
  id: string;
  taskId: string;
  taskName: string;
  agent: string;
  provider: string;
  rounds: number;
  models: string[];
  attempts: Attempt[];
  status: string;
  error?: string;
  startedAt: string;
  endedAt?: string;
}

const PROVIDER = "opencode-admin";

export default function AgentBenchmarks() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [models, setModels] = useState<string[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [taskId, setTaskId] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [rounds, setRounds] = useState(1);
  const [error, setError] = useState("");
  const [starting, setStarting] = useState(false);
  const pollRef = useRef<number | null>(null);

  const loadRuns = useCallback(async () => {
    try {
      const res = await fetch("/api/evals/runs");
      const data = await res.json();
      setRuns(data.runs ?? []);
    } catch {
      /* a failed poll is not worth interrupting a running benchmark for */
    }
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const [t, p] = await Promise.all([
          fetch("/api/evals/tasks").then((r) => r.json()),
          fetch("/api/chat/providers").then((r) => r.json()).catch(() => null),
        ]);
        const list: Task[] = t.tasks ?? [];
        setTasks(list);
        if (list.length) setTaskId(list[0].id);

        const provider = (p?.providers ?? []).find((x: { id: string }) => x.id === PROVIDER);
        setModels(Object.keys(provider?.models ?? {}).sort());
      } catch (e) {
        setError(String(e));
      }
    })();
    loadRuns();
  }, [loadRuns]);

  const activeRun = runs.find((r) => r.status === "running");

  // Poll only while something is in flight; a finished board does not need refreshing.
  useEffect(() => {
    if (!activeRun) {
      if (pollRef.current) window.clearInterval(pollRef.current);
      pollRef.current = null;
      return;
    }
    pollRef.current = window.setInterval(loadRuns, 4000);
    return () => {
      if (pollRef.current) window.clearInterval(pollRef.current);
    };
  }, [activeRun, loadRuns]);

  const task = tasks.find((t) => t.id === taskId);

  async function start() {
    setError("");
    setStarting(true);
    try {
      const res = await fetch("/api/evals/runs", {
        method: "POST",
        headers: { "content-type": "application/json" },
        body: JSON.stringify({ taskId, models: picked, provider: PROVIDER, rounds }),
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error ?? res.statusText);
      setRuns((prev) => [data, ...prev]);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setStarting(false);
    }
  }

  return (
    <div className="bench">
      <section className="bench-config">
        <label className="bench-field">
          <span>Task</span>
          <select value={taskId} onChange={(e) => setTaskId(e.target.value)}>
            {tasks.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} · @{t.agent}
              </option>
            ))}
          </select>
        </label>

        <label className="bench-field">
          <span>Rounds</span>
          <select value={rounds} onChange={(e) => setRounds(Number(e.target.value))}>
            {[1, 2, 3, 5].map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>

        <div className="bench-field bench-models">
          <span>
            Models ({picked.length}/{models.length})
          </span>
          <div className="bench-model-grid">
            {models.map((m) => (
              <label key={m} className={`bench-chip${picked.includes(m) ? " on" : ""}`}>
                <input
                  type="checkbox"
                  checked={picked.includes(m)}
                  onChange={(e) =>
                    setPicked((prev) =>
                      e.target.checked ? [...prev, m] : prev.filter((x) => x !== m),
                    )
                  }
                />
                {m}
              </label>
            ))}
          </div>
        </div>

        <button
          className="btn btn-primary"
          onClick={start}
          disabled={starting || !!activeRun || !picked.length || !taskId}
        >
          <Play size={14} />
          {activeRun ? "Benchmark running…" : `Run ${picked.length * rounds} attempt(s)`}
        </button>
      </section>

      {task?.description && <p className="bench-note muted">{task.description}</p>}

      <p className="bench-note muted">
        Attempts run one at a time — concurrent runs would contend for the same CodeGraph
        index and provider, inflating the timings this table compares. The server checks
        first that the agent's permissions actually apply; if they do not, the run stops
        instead of quietly measuring the default agent.
      </p>

      {error && (
        <div className="alert alert-danger">
          <AlertTriangle size={14} /> {error}
        </div>
      )}

      {runs.map((run) => (
        <RunCard key={run.id} run={run} />
      ))}

      {!runs.length && (
        <div className="empty-state">
          <p>No benchmark runs yet. Pick a task and some models above.</p>
        </div>
      )}
    </div>
  );
}

function RunCard({ run }: { run: Run }) {
  // Rank by correctness first: a model that answers in eight turns while missing half
  // the expected findings is not better than one that grinds to the complete answer.
  const rows = useMemo(
    () =>
      [...run.attempts].sort((a, b) => {
        const sa = a.score.answered ? a.score.hits.length : -1;
        const sb = b.score.answered ? b.score.hits.length : -1;
        return sb - sa || a.metrics.turns - b.metrics.turns;
      }),
    [run.attempts],
  );
  const expected = run.attempts[0]?.score.total ?? 0;

  return (
    <section className="bench-run">
      <header className="bench-run-head">
        <strong>{run.taskName}</strong>
        <span className="muted">
          @{run.agent} · {run.rounds} round(s) · {run.models.length} model(s)
        </span>
        <span className={`badge badge-${run.status === "done" ? "ok" : run.status === "failed" ? "danger" : "warn"}`}>
          {run.status}
        </span>
      </header>

      {run.error && (
        <div className="alert alert-danger">
          <AlertTriangle size={14} /> {run.error}
        </div>
      )}

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Model</th>
              <th>R</th>
              <th>Score</th>
              <th>Hard</th>
              <th>Turns</th>
              <th>Reads</th>
              <th>CG</th>
              <th>Inv</th>
              <th>Worst file</th>
              <th>Tokens</th>
              <th>Time</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((a) => (
              <tr key={`${a.model}-${a.round}`} title={a.score.missed.length ? `Missed: ${a.score.missed.join(", ")}` : undefined}>
                <td className="cell-mono">{a.model}</td>
                <td>{a.round}</td>
                <td>
                  {a.score.answered ? (
                    <strong>
                      {a.score.hits.length}/{expected}
                    </strong>
                  ) : (
                    <span className="muted">no answer</span>
                  )}
                </td>
                <td>{a.score.gotHard ? "✓" : "—"}</td>
                <td>{a.metrics.turns}</td>
                <td>{a.metrics.reads}</td>
                <td>{a.metrics.codegraph}</td>
                <td>{a.metrics.invalid || ""}</td>
                <td>{a.metrics.worstFileReads}</td>
                <td>{(a.metrics.tokensInput + a.metrics.tokensOutput).toLocaleString()}</td>
                <td>{a.seconds.toFixed(0)}s</td>
              </tr>
            ))}
            {!rows.length && (
              <tr>
                <td colSpan={11} className="muted">
                  Waiting for the first attempt…
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}
