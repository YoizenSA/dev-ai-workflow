import { Fragment, useEffect, useState } from "react";
import { ChevronDown, ChevronRight } from "lucide-react";
import TrendChart, { type TrendSeries } from "./TrendChart";
import "./BenchHistory.css";

// Types mirror the evals API: Run (internal/evals runner.go) over
// GET /api/evals/runs, Task (task.go) over GET /api/evals/tasks, LeaderRow
// (leaderboard.go) over GET /api/evals/leaderboard.
interface Score {
  hits: string[];
  total: number;
  answered: boolean;
  // Weighted is the weight share the hits cover, 0..1. Runs scored before
  // weighted scoring omit it, so it stays optional.
  weighted?: number;
}

interface Attempt {
  model: string;
  round: number;
  score: Score;
  metrics: { turns: number };
  costUsd?: number;
  costKnown?: boolean;
}

interface Run {
  id: string;
  taskId: string;
  taskName: string;
  agent: string;
  models: string[];
  attempts: Attempt[];
  status: string;
  startedAt: string;
}

interface Task {
  id: string;
  name: string;
  agent: string;
}

interface LeaderRow {
  model: string;
  trend: number[];
}

// Same conventions as the RunCard in AgentBenchmarks: sub-cent amounts keep
// four decimals so they do not round to a misleading "$0.00"; weighted share
// drops the decimal once it is above 10%.
function formatCost(n: number): string {
  if (!n) return "$0";
  if (n < 0.01) return `$${n.toFixed(4)}`;
  return `$${n.toFixed(2)}`;
}

function formatWeighted(weighted: number): string {
  return `${(weighted * 100).toFixed(weighted >= 0.1 ? 0 : 1)}%`;
}

function statusClass(status: string): string {
  if (status === "done") return "bh-status bh-status-done";
  if (status === "failed") return "bh-status bh-status-failed";
  return "bh-status bh-status-running";
}

export default function BenchHistory() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [runs, setRuns] = useState<Run[]>([]);
  const [rows, setRows] = useState<LeaderRow[]>([]);
  const [taskFilter, setTaskFilter] = useState("");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [details, setDetails] = useState<Record<string, Run>>({});
  const [detailError, setDetailError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const [t, r] = await Promise.all([
          fetch("/api/evals/tasks").then((res) => res.json()),
          fetch("/api/evals/runs").then((res) => res.json()),
        ]);
        setTasks(t.tasks ?? []);
        setRuns(r.runs ?? []);
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e));
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  // Trends come from the leaderboard; with no task picked it ranks across
  // every task, which is exactly what the filter's All option shows below.
  useEffect(() => {
    const query = taskFilter ? `?taskId=${encodeURIComponent(taskFilter)}` : "";
    let alive = true;
    fetch(`/api/evals/leaderboard${query}`)
      .then((res) => res.json())
      .then((data) => {
        if (alive) setRows(data.rows ?? []);
      })
      .catch(() => {
        if (alive) setRows([]);
      });
    return () => {
      alive = false;
    };
  }, [taskFilter]);

  const visible = taskFilter ? runs.filter((r) => r.taskId === taskFilter) : runs;
  const series: TrendSeries[] = rows.map((row) => ({ model: row.model, values: row.trend ?? [] }));

  // Expansion reads GET /api/evals/runs/{id} once per run, so the detail shows
  // the run as stored — attempts keep landing while a running run is open.
  async function toggleRun(run: Run) {
    if (expandedId === run.id) {
      setExpandedId(null);
      return;
    }
    setExpandedId(run.id);
    setDetailError("");
    if (details[run.id]) return;
    try {
      const res = await fetch(`/api/evals/runs/${run.id}`);
      const data = await res.json();
      if (!res.ok) throw new Error(data.error ?? res.statusText);
      setDetails((prev) => ({ ...prev, [run.id]: data }));
    } catch (e) {
      setDetailError(e instanceof Error ? e.message : String(e));
    }
  }

  if (loading) {
    return (
      <div className="bench-history">
        <p className="muted">Loading history…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="bench-history">
        <div className="alert alert-danger">{error}</div>
      </div>
    );
  }

  return (
    <div className="bench-history">
      <div className="bh-toolbar">
        <label className="bh-field">
          <span>Task</span>
          <select value={taskFilter} onChange={(e) => setTaskFilter(e.target.value)}>
            <option value="">All tasks</option>
            {tasks.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name} · @{t.agent}
              </option>
            ))}
          </select>
        </label>
        <p className="bh-note muted">{visible.length} run(s)</p>
      </div>

      {series.length > 0 && (
        <section className="bh-panel">
          <h3>Weighted score per run</h3>
          <TrendChart series={series} />
        </section>
      )}

      {runs.length === 0 ? (
        <div className="empty-state">
          <p>No benchmark runs yet. Start one from the Agent Benchmarks tab.</p>
        </div>
      ) : visible.length === 0 ? (
        <div className="empty-state">
          <p>No runs for this task yet.</p>
        </div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Run</th>
                <th>Date</th>
                <th>Task</th>
                <th>Models</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((run) => (
                <Fragment key={run.id}>
                  <tr>
                    <td>
                      <button
                        type="button"
                        className="bh-expand"
                        aria-expanded={expandedId === run.id}
                        onClick={() => toggleRun(run)}
                      >
                        {expandedId === run.id ? (
                          <ChevronDown size={14} />
                        ) : (
                          <ChevronRight size={14} />
                        )}
                        <span className="cell-mono">{run.id}</span>
                      </button>
                    </td>
                    <td>{new Date(run.startedAt).toLocaleString()}</td>
                    <td>{run.taskName}</td>
                    <td className="bh-models" title={run.models.join(", ")}>
                      {run.models.join(", ")}
                    </td>
                    <td>
                      <span className={statusClass(run.status)}>{run.status}</span>
                    </td>
                  </tr>
                  {expandedId === run.id && (
                    <tr>
                      <td colSpan={5} className="bh-detail-cell">
                        {detailError && <div className="alert alert-danger">{detailError}</div>}
                        {!detailError && !details[run.id] && <p className="muted">Loading run…</p>}
                        {details[run.id] && <AttemptTable run={details[run.id]} />}
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

// Attempt columns reuse the RunCard set that matters after the fact: model,
// score, weighted, cost, turns.
function AttemptTable({ run }: { run: Run }) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th>Model</th>
            <th>Score</th>
            <th>Weighted</th>
            <th>Cost</th>
            <th>Turns</th>
          </tr>
        </thead>
        <tbody>
          {run.attempts.map((a) => (
            <tr key={`${a.model}-${a.round}`}>
              <td className="cell-mono">{a.model}</td>
              <td>
                {a.score.answered ? (
                  <strong>
                    {a.score.hits.length}/{a.score.total}
                  </strong>
                ) : (
                  <span className="muted">no answer</span>
                )}
              </td>
              <td>{a.score.weighted != null ? formatWeighted(a.score.weighted) : "-"}</td>
              <td>{a.costKnown ? formatCost(a.costUsd ?? 0) : "-"}</td>
              <td>{a.metrics.turns}</td>
            </tr>
          ))}
          {!run.attempts.length && (
            <tr>
              <td colSpan={5} className="muted">
                No attempts recorded.
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
