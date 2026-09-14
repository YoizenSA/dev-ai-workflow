import { useEffect, useState } from "react";
import AgentBenchmarks from "./AgentBenchmarks";
import BenchHistory from "./BenchHistory";
import MemoryRecallEval from "./MemoryRecallEval";
import SessionAnalytics from "./SessionAnalytics";
import SessionCompare from "./SessionCompare";
import "./Evals.css";

type EvalKind = "tasks" | "recall" | "sessions" | "history";

type EvalEnvironment = { name: string; serverUrl?: string; dbPath?: string };

function initialKind(): EvalKind {
  // Deep-linkable so a tab is reachable by URL: ?tab=tasks opens Agent Benchmarks
  // directly, which also keeps the view shareable and survives a reload.
  const t = new URLSearchParams(window.location.search).get("tab");
  return t === "tasks" || t === "recall" || t === "sessions" || t === "history" ? t : "sessions";
}

export default function Evals() {
  const [kind, setKindState] = useState<EvalKind>(initialKind);
  // "" = every environment (local database, unfiltered runs) — the historical
  // default. Any other value targets that environment's database and filters
  // runs to it.
  const [env, setEnv] = useState("");
  const [environments, setEnvironments] = useState<EvalEnvironment[]>([]);
  const [compare, setCompare] = useState(false);

  useEffect(() => {
    fetch("/api/evals/environments")
      .then((r) => r.json())
      .then((d) => setEnvironments(d.environments ?? []))
      .catch(() => setEnvironments([]));
  }, []);

  const setKind = (next: EvalKind) => {
    setKindState(next);
    const url = new URL(window.location.href);
    url.searchParams.set("tab", next);
    window.history.replaceState(null, "", url);
  };

  return (
    <>
      <header className="page-header">
        <div className="page-heading">
          <span className="page-eyebrow">Benchmarks</span>
          <h1 className="page-title">Evals</h1>
          <p className="page-subtitle">
            Benchmarks, memory recall, and live OpenCode skill/tool usage by project
          </p>
        </div>
      </header>

      {environments.length > 1 && (
        <div className="evals-env-row">
          <label className="evals-env-label" htmlFor="evals-env-select">
            Environment
          </label>
          <select
            id="evals-env-select"
            className="select evals-env-select"
            value={env}
            onChange={(e) => {
              setEnv(e.target.value);
              setCompare(false);
            }}
            disabled={compare}
            title={compare ? "Comparison covers every environment" : "Which environment's data to show"}
          >
            <option value="">All environments</option>
            {environments.map((e) => (
              <option key={e.name} value={e.name}>
                {e.name}
              </option>
            ))}
          </select>
          {kind === "sessions" && (
            <button
              className={`btn btn-sm${compare ? " btn-primary" : ""}`}
              onClick={() => setCompare((c) => !c)}
              title="Side-by-side session analytics for every environment"
            >
              {compare ? "Exit comparison" : "Compare environments"}
            </button>
          )}
        </div>
      )}

      <div className="tabs evals-tabs">
        <button
          className={`tab${kind === "sessions" ? " active" : ""}`}
          onClick={() => setKind("sessions")}
        >
          Session Analytics
        </button>
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
        <button
          className={`tab${kind === "history" ? " active" : ""}`}
          onClick={() => setKind("history")}
        >
          History
        </button>
      </div>

      {kind === "sessions" ? (
        compare ? (
          <SessionCompare />
        ) : (
          <SessionAnalytics env={env} />
        )
      ) : kind === "recall" ? (
        <MemoryRecallEval />
      ) : kind === "history" ? (
        <BenchHistory env={env} />
      ) : (
        <AgentBenchmarks env={env} />
      )}
    </>
  );
}
