import { useState } from "react";
import AgentBenchmarks from "./AgentBenchmarks";
import MemoryRecallEval from "./MemoryRecallEval";
import SessionAnalytics from "./SessionAnalytics";
import "./Evals.css";

type EvalKind = "tasks" | "recall" | "sessions";

function initialKind(): EvalKind {
  // Deep-linkable so a tab is reachable by URL: ?tab=tasks opens Agent Benchmarks
  // directly, which also keeps the view shareable and survives a reload.
  const t = new URLSearchParams(window.location.search).get("tab");
  return t === "tasks" || t === "recall" || t === "sessions" ? t : "sessions";
}

export default function Evals() {
  const [kind, setKindState] = useState<EvalKind>(initialKind);

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
      </div>

      {kind === "sessions" ? (
        <SessionAnalytics />
      ) : kind === "recall" ? (
        <MemoryRecallEval />
      ) : (
        <AgentBenchmarks />
      )}
    </>
  );
}
