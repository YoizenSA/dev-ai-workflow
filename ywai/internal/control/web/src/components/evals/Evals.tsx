import { useState, useEffect } from "react";
import { Check, FileText, SquareCheck, X } from "lucide-react";
import MemoryRecallEval from "./MemoryRecallEval";
import "./Evals.css";

type EvalKind = "tasks" | "recall";

interface TaskResult {
  taskId: string;
  passed: boolean;
  mode: string;
  model: string;
  agent: string;
  duration: number;
  tokens: number;
  cost: number;
  error: string;
  output: string;
}

interface RunSummary {
  total: number;
  passed: number;
  failed: number;
  passRate: number;
  avgDuration: number;
  totalTokens: number;
  totalCost: number;
}

interface EvalRun {
  id: string;
  startedAt: string;
  endedAt: string;
  status: string;
  mode: string;
  model: string;
  agent: string;
  results: TaskResult[];
  summary: RunSummary;
}

export default function Evals() {
  const [kind, setKind] = useState<EvalKind>("tasks");
  const [runs, setRuns] = useState<EvalRun[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedRun, setSelectedRun] = useState<EvalRun | null>(null);

  useEffect(() => {
    if (kind === "tasks") fetchRuns();
  }, [kind]);

  async function fetchRuns() {
    try {
      setLoading(true);
      const res = await fetch("/api/evals/runs");
      if (!res.ok) throw new Error(`${res.status}: ${res.statusText}`);
      const data = await res.json();
      setRuns(data.runs ?? []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  }

  function formatDuration(ns: number) {
    const ms = ns / 1_000_000;
    if (ms < 1000) return `${ms.toFixed(0)}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  }

  function passRateColor(rate: number): string {
    if (rate >= 0.8) return "var(--tint-success)";
    if (rate >= 0.5) return "var(--warning)";
    return "var(--tint-danger)";
  }

  const latest = runs[0];

  return (
    <>
      <header className="page-header">
        <div className="page-heading">
          <span className="page-eyebrow">Benchmarks</span>
          <h1 className="page-title">Evals</h1>
          <p className="page-subtitle">
            Measure agent accuracy and memory retrieval quality across benchmark runs
          </p>
        </div>
      </header>

      <div className="tabs">
        <button
          className={`tab${kind === "tasks" ? " active" : ""}`}
          onClick={() => setKind("tasks")}
        >
          Agent Benchmarks
        </button>
        <button
          className={`tab${kind === "recall" ? " active" : ""}`}
          onClick={() => setKind("recall")}
        >
          Memory Recall
        </button>
      </div>

      {kind === "recall" ? (
        <MemoryRecallEval />
      ) : loading ? (
        <div className="skeleton skel-card" style={{ margin: 'var(--space-4)' }} aria-busy="true">
          <div className="skel-line title" />
          <div className="skel-line desc" />
          <div className="skel-line desc sm" />
        </div>
      ) : error ? (
        <div className="alert alert-danger">Error: {error}</div>
      ) : runs.length === 0 ? (
        <div className="empty-state">
          <div className="empty-icon">
            <FileText size={24} />
          </div>
          <p className="empty-title">No benchmark runs yet</p>
          <p className="empty-desc">Run <code>ywai eval run</code> to measure agent performance on standard tasks</p>
        </div>
      ) : (
        <div className="eval-runs-list">
          {/* KPI summary cards */}
          <div className="kpi-grid">
            <div className="kpi">
              <div className="kpi-top">
                <div className="kpi-icon" style={{ '--kpi-icon-bg': 'rgba(var(--info-rgb), 0.16)', '--kpi-icon-color': 'var(--tint-info)' } as React.CSSProperties}>
                  <FileText size={20} />
                </div>
              </div>
              <div className="kpi-value tnum">{runs.length}</div>
              <div className="kpi-label">Total Runs</div>
              <div className="kpi-subtitle">All evaluation runs</div>
            </div>
            <div className="kpi">
              <div className="kpi-top">
                <div className="kpi-icon" style={{ '--kpi-icon-bg': 'var(--success-soft)', '--kpi-icon-color': 'var(--tint-success)' } as React.CSSProperties}>
                  <Check size={20} />
                </div>
              </div>
              <div className="kpi-value tnum" style={{ color: passRateColor(latest?.summary.passRate ?? 0) }}>
                {((latest?.summary.passRate ?? 0) * 100).toFixed(0)}%
              </div>
              <div className="kpi-label">Latest Pass Rate</div>
              <div className="kpi-subtitle">Tasks passed / total</div>
            </div>
            <div className="kpi">
              <div className="kpi-top">
                <div className="kpi-icon" style={{ '--kpi-icon-bg': 'rgba(var(--yz-primary-2-rgb), 0.16)', '--kpi-icon-color': 'var(--tint-purple)' } as React.CSSProperties}>
                  <SquareCheck size={20} />
                </div>
              </div>
              <div className="kpi-value" style={{ fontSize: 'var(--text-lg)' }}>
                {latest?.model || "(default)"}
              </div>
              <div className="kpi-label">Latest Model</div>
            </div>
          </div>

          {/* Runs table */}
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th title="Unique run identifier">Run ID</th>
                  <th title="Eval mode (coder/architect/qa)">Mode</th>
                  <th title="Agent model used">Model</th>
                  <th title="Percentage of tasks passed">Pass Rate</th>
                  <th title="Tasks completed / total">Tasks</th>
                  <th title="Duration per task">Duration</th>
                  <th title="Run date">Date</th>
                </tr>
              </thead>
              <tbody>
                {runs.map((run) => (
                  <tr
                    key={run.id}
                    className={`clickable ${selectedRun?.id === run.id ? "selected" : ""}`}
                    onClick={() =>
                      setSelectedRun(selectedRun?.id === run.id ? null : run)
                    }
                    style={selectedRun?.id === run.id ? { background: 'var(--surface-hover)' } : undefined}
                  >
                    <td className="cell-mono eval-run-id">{run.id}</td>
                    <td>
                      <span className={`eval-mode-badge ${run.mode}`}>
                        {run.mode}
                      </span>
                    </td>
                    <td>{run.model || "(default)"}</td>
                    <td>
                      <span
                        className="tnum"
                        style={{
                          color: passRateColor(run.summary.passRate),
                          fontWeight: 600,
                        }}
                      >
                        {(run.summary.passRate * 100).toFixed(0)}%
                      </span>
                    </td>
                    <td className="tnum">
                      {run.summary.passed}/{run.summary.total}
                    </td>
                    <td className="tnum">{formatDuration(run.summary.avgDuration)}</td>
                    <td className="cell-muted">
                      {new Date(run.startedAt).toLocaleDateString()}{" "}
                      {new Date(run.startedAt).toLocaleTimeString()}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {/* Detail view */}
          {selectedRun && (
            <div className="eval-detail">
              <h3>Run Details: <span className="mono" style={{ color: 'var(--tint-info)' }}>{selectedRun.id}</span></h3>
              <div className="eval-detail-meta">
                <span>Mode: <strong style={{ color: 'var(--text)' }}>{selectedRun.mode}</strong></span>
                <span>Model: <strong style={{ color: 'var(--text)' }}>{selectedRun.model || "(default)"}</strong></span>
                <span>Agent: <strong style={{ color: 'var(--text)' }}>{selectedRun.agent || "(default)"}</strong></span>
                <span>Tokens: <strong className="tnum" style={{ color: 'var(--text)' }}>{selectedRun.summary.totalTokens.toLocaleString()}</strong></span>
                <span>Cost: <strong className="tnum" style={{ color: 'var(--text)' }}>${selectedRun.summary.totalCost.toFixed(4)}</strong></span>
              </div>
              <div className="table-wrap">
                <table className="data-table">
                  <thead>
                    <tr>
                      <th style={{ width: '40px' }}></th>
                      <th>Task</th>
                      <th>Duration</th>
                      <th>Error</th>
                    </tr>
                  </thead>
                  <tbody>
                    {selectedRun.results.map((r) => (
                      <tr key={r.taskId}>
                        <td>{r.passed ? <Check size={16} /> : <X size={16} />}</td>
                        <td className="cell-mono">{r.taskId}</td>
                        <td className="tnum">{formatDuration(r.duration)}</td>
                        <td className="eval-error">{r.error || "—"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      )}
    </>
  );
}
