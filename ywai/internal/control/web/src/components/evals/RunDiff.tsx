import "./RunDiff.css";

/**
 * RunDiff — per-model delta table for a current-vs-baseline benchmark pair.
 *
 * PURE: no fetch, no store, no effects. The integrator fetches the deltas and
 * passes them in. Backed by evals.DiffModels (ywai/internal/evals/delta.go):
 * each delta is "current minus baseline", and a null field means "no baseline
 * to compare" — never a computed zero.
 *
 * Props:
 *   aTitle   — label of the A (current) side, shown in the header.
 *   bTitle   — label of the B (baseline) side, shown in the header.
 *   deltas   — one row per model in the current run, ModelDelta JSON:
 *              { model, weightedDelta, hardDelta, turnsDelta, costDeltaUsd }
 *              with every delta a number | null (null = no comparison).
 *
 * Reading a cell: the glyph is the direction of change (▲ rose, ▼ fell,
 * — unchanged) and the colour is whether that direction is good — up is good
 * for weighted/hard, up is heavier (bad) for turns/cost, per DiffModels.
 * A null delta renders a muted em dash titled "no comparison", so an absent
 * baseline stays visually distinct from a real 0.00.
 *
 * Recommended mount point: inside AgentBenchmarks (or a run-comparison panel
 * it mounts), fed from GET /api/evals/runs — pick the two Run ids to compare,
 * aggregate each with the server's summaries and diff them, or mount where the
 * deltas endpoint lands (evals.ModelDelta JSON, see delta.go). Example:
 *   <RunDiff aTitle={runA.id} bTitle={runB.id} deltas={diffModels(sumA, sumB)} />
 */

export interface RunDiffDelta {
  model: string;
  weightedDelta: number | null;
  hardDelta: number | null;
  turnsDelta: number | null;
  costDeltaUsd: number | null;
}

export interface RunDiffProps {
  aTitle: string;
  bTitle: string;
  deltas: RunDiffDelta[];
}

// Up is good for the score columns, up is heavier for the cost columns.
type ColumnTone = "upGood" | "upBad";

interface Column {
  key: keyof Omit<RunDiffDelta, "model">;
  label: string;
  tone: ColumnTone;
  // Score deltas keep two decimals; money needs four or small moves round away.
  decimals: number;
  prefix: string;
}

const COLUMNS: Column[] = [
  { key: "weightedDelta", label: "Weighted", tone: "upGood", decimals: 2, prefix: "" },
  { key: "hardDelta", label: "Hard", tone: "upGood", decimals: 2, prefix: "" },
  { key: "turnsDelta", label: "Turns", tone: "upBad", decimals: 2, prefix: "" },
  { key: "costDeltaUsd", label: "Cost", tone: "upBad", decimals: 4, prefix: "$" },
];

function DeltaCell({ value, col }: { value: number | null; col: Column }) {
  if (value === null) {
    return (
      <span className="rd-none" title="no comparison">
        —
      </span>
    );
  }
  const glyph = value > 0 ? "▲" : value < 0 ? "▼" : "—";
  // A heavier trace is a worse turns/cost delta; a higher score is a better one.
  const good = col.tone === "upGood" ? value > 0 : value < 0;
  const bad = col.tone === "upGood" ? value < 0 : value > 0;
  const cls = good ? " rd-good" : bad ? " rd-bad" : "";
  const shown = `${value > 0 ? "+" : ""}${value.toFixed(col.decimals)}`;
  return (
    <span className={`rd-delta${cls}`}>
      <span aria-hidden="true">{glyph}</span> {col.prefix}
      {shown}
    </span>
  );
}

export default function RunDiff({ aTitle, bTitle, deltas }: RunDiffProps) {
  if (!deltas.length) {
    return (
      <div className="empty-state">
        <p>No models to compare.</p>
      </div>
    );
  }

  return (
    <section className="rd" aria-label={`Run diff: ${aTitle} vs ${bTitle}`}>
      <header className="rd-head">
        <strong>{aTitle}</strong> <span className="muted">vs</span>{" "}
        <strong>{bTitle}</strong>
        <span className="muted rd-legend">
          deltas are current minus baseline · ▲ rose · ▼ fell · — unchanged
        </span>
      </header>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Model</th>
              {COLUMNS.map((c) => (
                <th key={c.key} className={c.tone === "upBad" ? "num" : undefined}>
                  {c.label}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {deltas.map((d) => (
              <tr key={d.model}>
                <td className="cell-mono">{d.model}</td>
                {COLUMNS.map((c) => (
                  <td key={c.key} className={c.tone === "upBad" ? "cell-num" : undefined}>
                    <DeltaCell value={d[c.key]} col={c} />
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );
}
