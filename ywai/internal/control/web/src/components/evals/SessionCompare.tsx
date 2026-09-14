import { useEffect, useMemo, useState } from "react";
import { DAY_OPTIONS } from "./SessionAnalytics";

// Ultra-detailed Session Analytics comparison across every eval environment:
// KPI table with totals, then one matrix per dimension (agents, models,
// skills) plus per-environment project breakdowns. All data comes from the
// same per-env analytics endpoint the single view uses; scans run in
// parallel and the server caches each environment for 30 minutes.

type EvalEnvironment = { name: string; serverUrl?: string; dbPath?: string };

interface CompareSummary {
  sessions: number;
  projects: number;
  skillCalls: number;
  toolCalls: number;
  totalCost: number;
  tokensInput?: number;
  tokensOutput?: number;
  avgToolsPerSession?: number;
  avgCostPerSession?: number;
  skillCoverage?: number;
  delegationCalls?: number;
  installedSkills?: number;
  unusedSkillCount?: number;
}

interface NamedCount {
  name: string;
  count: number;
  sessions?: number;
  share?: number;
  cost?: number;
  tokensInput?: number;
  tokensOutput?: number;
}

interface ProjectStat {
  id: string;
  name: string;
  worktree: string;
  sessions: number;
  skillCalls: number;
  toolCalls: number;
  cost: number;
}

interface AnalyticsData {
  summary: CompareSummary;
  insights?: string[];
  unusedSkills?: string[];
  projects?: ProjectStat[];
  skills?: NamedCount[];
  tools?: NamedCount[];
  agents?: NamedCount[];
  models?: NamedCount[];
}

interface Column {
  name: string;
  loading: boolean;
  error?: string;
  data?: AnalyticsData;
}

const fmtInt = (n: number) => n.toLocaleString("en-US");
const fmtCost = (n: number) => "$" + n.toFixed(n < 10 ? 4 : 2);
const fmtPct = (n: number) => `${Math.round(n * 100)}%`;
const fmtM = (n: number) => (n >= 1e6 ? `${(n / 1e6).toFixed(1)}M` : n >= 1e3 ? `${(n / 1e3).toFixed(1)}k` : fmtInt(n));

const PREVIEW_ROWS = 10;

export default function SessionCompare() {
  const [cols, setCols] = useState<Column[]>([]);
  const [loading, setLoading] = useState(true);
  const [days, setDays] = useState(30);
  const [projectId, setProjectId] = useState("");
  const [attempt, setAttempt] = useState(0);
  const [showAll, setShowAll] = useState<Record<string, boolean>>({});
  // Union of projects seen across environments while unfiltered — captured so
  // the dropdown keeps every option after a project filter shrinks the data.
  const [unionProjects, setUnionProjects] = useState<ProjectStat[]>([]);

  useEffect(() => {
    let alive = true;
    (async () => {
      setLoading(true);
      let envs: EvalEnvironment[] = [];
      try {
        const res = await fetch("/api/evals/environments");
        const body = await res.json();
        envs = body.environments ?? [];
      } catch {
        envs = [];
      }
      if (!alive) return;
      const results = await Promise.all(
        envs.map(async (e): Promise<Column> => {
          const name = e.name || "local";
          const params = new URLSearchParams({ days: String(days) });
          if (e.name) params.set("env", e.name);
          if (projectId) params.set("projectId", projectId);
          if (attempt > 0) params.set("refresh", "1");
          try {
            const res = await fetch(`/api/evals/session-analytics?${params}`, {
              cache: "no-store",
            });
            const body = await res.json().catch(() => ({}));
            if (!res.ok) throw new Error(body.error || `${res.status}: ${res.statusText}`);
            return { name, loading: false, data: body as AnalyticsData };
          } catch (err) {
            return {
              name,
              loading: false,
              error: err instanceof Error ? err.message : String(err),
            };
          }
        }),
      );
      if (alive) {
        setCols(results);
        setLoading(false);
      }
      if (!projectId) {
        // Capture the full project union while unfiltered so the selector
        // keeps every option once a filter narrows the results.
        const byId = new Map<string, ProjectStat>();
        for (const c of results) {
          for (const p of c.data?.projects ?? []) {
            const prev = byId.get(p.id);
            if (prev) {
              prev.sessions += p.sessions;
            } else {
              byId.set(p.id, { ...p });
            }
          }
        }
        setUnionProjects(Array.from(byId.values()).sort((a, b) => b.sessions - a.sessions));
      }
    })();
    return () => {
      alive = false;
    };
  }, [days, projectId, attempt]);

  const ready = cols.filter((c) => c.data);
  const sum = (pick: (c: Column) => number) => ready.reduce((acc, c) => acc + pick(c), 0);
  const maxOf = (pick: (c: Column) => number) => Math.max(0, ...ready.map((c) => pick(c)));

  // Union matrix for one named-count dimension, sorted by total usage desc.
  const matrix = (pick: (c: Column) => NamedCount[] | undefined) => {
    const totals = new Map<string, { perEnv: Record<string, number>; cost: Record<string, number>; total: number }>();
    for (const c of ready) {
      for (const row of pick(c) ?? []) {
        let entry = totals.get(row.name);
        if (!entry) {
          entry = { perEnv: {}, cost: {}, total: 0 };
          totals.set(row.name, entry);
        }
        entry.perEnv[c.name] = (entry.perEnv[c.name] ?? 0) + row.count;
        if (row.cost) entry.cost[c.name] = (entry.cost[c.name] ?? 0) + row.cost;
        entry.total += row.count;
      }
    }
    return Array.from(totals.entries())
      .map(([name, v]) => ({ name, ...v }))
      .sort((a, b) => b.total - a.total);
  };

  const agentRows = useMemo(() => matrix((c) => c.data?.agents), [cols]);
  const modelRows = useMemo(() => matrix((c) => c.data?.models), [cols]);
  const skillRows = useMemo(() => matrix((c) => c.data?.skills), [cols]);

  const matrixSection = (
    title: string,
    rows: { name: string; perEnv: Record<string, number>; cost: Record<string, number>; total: number }[],
    sectionKey: string,
    withCost: boolean,
  ) => {
    if (rows.length === 0) {
      return (
        <section className="env-compare-section">
          <h3 className="env-compare-h3">{title}</h3>
          <p className="env-compare-hint">No usage recorded in this window.</p>
        </section>
      );
    }
    const open = showAll[sectionKey];
    const visible = open ? rows : rows.slice(0, PREVIEW_ROWS);
    return (
      <section className="env-compare-section">
        <h3 className="env-compare-h3">
          {title} <span className="env-compare-count">({rows.length})</span>
        </h3>
        <div className="env-compare-scroll">
          <table className="env-compare-table">
            <thead>
              <tr>
                <th>{title}</th>
                {cols.map((c) => (
                  <th key={c.name}>{c.name}</th>
                ))}
                <th>Total</th>
              </tr>
            </thead>
            <tbody>
              {visible.map((r) => (
                <tr key={r.name}>
                  <td className="env-compare-metric">{r.name}</td>
                  {cols.map((c) => {
                    const n = r.perEnv[c.name] ?? 0;
                    const cost = r.cost[c.name];
                    return (
                      <td key={c.name} className={n > 0 ? "" : "env-compare-zero"}>
                        {n > 0 ? (
                          <>
                            {fmtInt(n)}
                            {withCost && cost ? <span className="env-compare-cost"> · {fmtCost(cost)}</span> : null}
                          </>
                        ) : (
                          <span className="env-compare-zero">·</span>
                        )}
                      </td>
                    );
                  })}
                  <td className="env-compare-total">{fmtInt(r.total)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length > PREVIEW_ROWS && (
            <button className="btn btn-sm env-compare-more" onClick={() => setShowAll((s) => ({ ...s, [sectionKey]: !open }))}>
              {open ? "Show less" : `Show all ${rows.length}`}
            </button>
          )}
        </div>
      </section>
    );
  };

  return (
    <div className="env-compare">
      <div className="env-compare-toolbar">
        <p className="env-compare-hint">
          Ultra comparison across environments — same filters as the single
          view, applied to every environment at once. Scans are cached
          server-side for 30 minutes.
        </p>
        <div className="env-compare-toolbar-actions">
          <label className="recall-field">
            <span className="recall-field-label">Time range</span>
            <select
              className="input"
              value={days}
              onChange={(e) => setDays(Number(e.target.value))}
            >
              {DAY_OPTIONS.map((o) => (
                <option key={o.value} value={o.value}>
                  {o.label}
                </option>
              ))}
            </select>
          </label>
          <label className="recall-field">
            <span className="recall-field-label">Project</span>
            <select
              className="input"
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
            >
              <option value="">All projects</option>
              {unionProjects.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || p.worktree} ({p.sessions})
                </option>
              ))}
            </select>
          </label>
          <button className="btn btn-sm" onClick={() => setAttempt((n) => n + 1)} disabled={loading}>
            {loading ? "Scanning…" : "Refresh"}
          </button>
        </div>
      </div>

      <section className="env-compare-section">
        <h3 className="env-compare-h3">Overview</h3>
        <div className="env-compare-scroll">
          <table className="env-compare-table">
            <thead>
              <tr>
                <th>Metric</th>
                {cols.map((c) => (
                  <th key={c.name}>{c.name}</th>
                ))}
                <th>Σ Total</th>
              </tr>
            </thead>
            <tbody>
              {(
                [
                  ["Sessions", (c) => c.data?.summary.sessions ?? 0, fmtInt, true],
                  ["Projects", (c) => c.data?.summary.projects ?? 0, fmtInt, true],
                  ["Skill calls", (c) => c.data?.summary.skillCalls ?? 0, fmtInt, true],
                  ["Tool calls", (c) => c.data?.summary.toolCalls ?? 0, fmtInt, true],
                  ["Cost", (c) => c.data?.summary.totalCost ?? 0, (n) => fmtCost(n), true],
                  [
                    "Tokens in",
                    (c) => c.data?.summary.tokensInput ?? 0,
                    (n) => fmtM(n),
                    false,
                  ],
                  [
                    "Tokens out",
                    (c) => c.data?.summary.tokensOutput ?? 0,
                    (n) => fmtM(n),
                    false,
                  ],
                  [
                    "Avg tools / session",
                    (c) => c.data?.summary.avgToolsPerSession ?? 0,
                    (n) => n.toFixed(1),
                    false,
                  ],
                  [
                    "Skill coverage",
                    (c) => c.data?.summary.skillCoverage ?? 0,
                    (n) => fmtPct(n),
                    false,
                  ],
                  [
                    "Delegation calls",
                    (c) => c.data?.summary.delegationCalls ?? 0,
                    fmtInt,
                    true,
                  ],
                ] as [string, (c: Column) => number, (n: number) => string, boolean][]
              ).map(([label, pick, fmt, highlightMax]) => {
                const best = highlightMax ? maxOf(pick) : -1;
                return (
                  <tr key={label}>
                    <td className="env-compare-metric">{label}</td>
                    {cols.map((c) => {
                      const v = pick(c);
                      const isBest = highlightMax && ready.length > 1 && v === best && v > 0;
                      return (
                        <td key={c.name} className={isBest ? "env-compare-best" : ""}>
                          {c.loading ? "…" : c.error ? "—" : fmt(v)}
                        </td>
                      );
                    })}
                    <td className="env-compare-total">{fmt(sum(pick))}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </section>

      {matrixSection("Agents", agentRows, "agents", true)}
      {matrixSection("Models", modelRows, "models", true)}
      {matrixSection("Skills", skillRows, "skills", false)}

      <section className="env-compare-section">
        <h3 className="env-compare-h3">Projects per environment</h3>
        <div className="env-compare-projects">
          {cols.map((c) => {
            const projects = (c.data?.projects ?? [])
              .slice()
              .sort((a, b) => b.sessions - a.sessions)
              .slice(0, 6);
            return (
              <div key={c.name} className="env-compare-project-card">
                <h4>{c.name}</h4>
                {c.error ? (
                  <p className="env-compare-err">unavailable</p>
                ) : projects.length === 0 ? (
                  <p className="env-compare-hint">No projects yet.</p>
                ) : (
                  <ul>
                    {projects.map((p) => (
                      <li key={p.id}>
                        <span className="env-compare-project-name">{p.name || p.worktree || p.id}</span>
                        <span className="env-compare-project-meta">
                          {fmtInt(p.sessions)} sess · {fmtCost(p.cost)}
                        </span>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            );
          })}
        </div>
      </section>
    </div>
  );
}
