// TrendChart — hand-rolled SVG sparklines over leaderboard trend arrays.
// Dependency-free on purpose: one polyline per model, no chart library.

export interface TrendSeries {
  model: string;
  /** Per-run weighted averages, oldest first (the leaderboard `trend` array). */
  values: number[];
}

const W = 120;
const H = 32;
const PAD = 3;
const COLORS = 6; // trend-color-0 … trend-color-5 in BenchHistory.css

function pointsOf(values: number[]): string {
  // Weighted scores live in 0..1; widening the domain to at least that keeps
  // every model drawn on the same scale, and still fits any other data.
  const lo = Math.min(0, ...values);
  const hi = Math.max(1, ...values);
  const span = hi - lo || 1;
  return values
    .map((v, i) => {
      const x = values.length === 1 ? W / 2 : PAD + (i / (values.length - 1)) * (W - 2 * PAD);
      const y = H - PAD - ((v - lo) / span) * (H - 2 * PAD);
      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");
}

export default function TrendChart({ series }: { series: TrendSeries[] }) {
  const plotted = series.filter((s) => s.values.length > 0);
  if (!plotted.length) {
    return (
      <p className="trend-empty muted">No trend data yet — run a benchmark to plot per-run scores.</p>
    );
  }
  return (
    <div className="trend-chart">
      {plotted.map((s, i) => (
        <div className="trend-row" key={s.model}>
          <span className="trend-model" title={s.model}>
            {s.model}
          </span>
          <svg
            className={`trend-svg trend-color-${i % COLORS}`}
            viewBox={`0 0 ${W} ${H}`}
            width={W}
            height={H}
            role="img"
            aria-label={`${s.model} trend: ${s.values.map((v) => v.toFixed(2)).join(", ")}`}
          >
            <polyline className="trend-line" points={pointsOf(s.values)} />
          </svg>
          <span className="trend-last" title="Latest run">
            {s.values[s.values.length - 1].toFixed(2)}
          </span>
        </div>
      ))}
    </div>
  );
}
