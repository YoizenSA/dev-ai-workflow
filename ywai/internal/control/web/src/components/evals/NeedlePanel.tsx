import { useState } from "react";
import "./RunDiff.css";

/**
 * NeedlePanel — per-attempt needle breakdown for one benchmark run.
 *
 * PURE: no fetch, no store, no effects. The integrator fetches the run and
 * maps each evals.Attempt (ywai/internal/evals/runner.go) into the narrowed
 * NeedleAttempt view model below; `responsePreview` is the integrator's
 * truncated `response` string, so this panel never holds full transcripts.
 *
 * Props:
 *   attempts — one entry per attempt, keyed by model + round:
 *     model           — model name, as stored on the attempt.
 *     round           — round number within the run.
 *     score.hits      — labels of the expectations the answer hit.
 *     score.missed    — labels of the expectations it missed.
 *     score.gotHard   — true when every hard expectation was hit.
 *     score.answered  — false marks an errored/empty attempt (no percent).
 *     score.weighted  — weight share the hits cover, 0..1 (see scoring.go).
 *     responsePreview — short text of the attempt's response.
 *
 * Each row is a collapsible header (model, round, hard badge, weighted
 * percent); expanding it reveals the hit/missed label chips and the response
 * preview inside a native <details>. The hard badge renders only when
 * gotHard is true, so its absence is meaningful.
 *
 * Recommended mount point: inside AgentBenchmarks' RunCard (or next to it),
 * fed from the same GET /api/evals/runs payload that card already has —
 * one panel per run, attempts in stored order. Example:
 *   <NeedlePanel attempts={run.attempts.map(toNeedleAttempt)} />
 */

export interface NeedleScore {
  hits: string[];
  missed: string[];
  gotHard: boolean;
  answered: boolean;
  weighted: number;
}

export interface NeedleAttempt {
  model: string;
  round: number;
  score: NeedleScore;
  responsePreview: string;
}

export interface NeedlePanelProps {
  attempts: NeedleAttempt[];
}

function attemptKey(a: NeedleAttempt, i: number): string {
  // Model + round is the run's own identity for an attempt; the index keeps
  // duplicate pairs from colliding instead of dropping one from the panel.
  return `${a.model}#${a.round}#${i}`;
}

export default function NeedlePanel({ attempts }: NeedlePanelProps) {
  const [open, setOpen] = useState<Set<string>>(new Set());

  const toggle = (key: string) => {
    setOpen((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  if (!attempts.length) {
    return (
      <div className="empty-state">
        <p>No attempts to inspect.</p>
      </div>
    );
  }

  return (
    <div className="np">
      {attempts.map((a, i) => {
        const key = attemptKey(a, i);
        const expanded = open.has(key);
        const bodyId = `np-body-${key}`;
        return (
          <div className="np-row" key={key}>
            <button
              type="button"
              className="np-row-head"
              aria-expanded={expanded}
              aria-controls={bodyId}
              onClick={() => toggle(key)}
            >
              <span className="np-model">{a.model}</span>
              <span className="np-round">round {a.round}</span>
              {a.score.gotHard && <span className="np-hard">hard</span>}
              {a.score.answered ? (
                <span className="np-percent">{Math.round(a.score.weighted * 100)}%</span>
              ) : (
                <span className="np-percent muted">no answer</span>
              )}
              <span className="np-caret" aria-hidden="true" data-expanded={expanded}>
                ▶
              </span>
            </button>
            {expanded && (
              <div className="np-body" id={bodyId}>
                <div className="np-chips">
                  {a.score.hits.map((label) => (
                    <span key={label} className="np-chip np-hit" title="hit">
                      {label}
                    </span>
                  ))}
                  {a.score.missed.map((label) => (
                    <span key={label} className="np-chip np-miss" title="missed">
                      {label}
                    </span>
                  ))}
                </div>
                <details className="np-response">
                  <summary>Response</summary>
                  <pre>{a.responsePreview || "(no response text)"}</pre>
                </details>
              </div>
            )}
          </div>
        );
      })}
    </div>
  );
}
