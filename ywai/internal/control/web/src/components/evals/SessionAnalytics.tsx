import { useCallback, useEffect, useMemo, useState, type CSSProperties } from "react";
import {
  Activity,
  Bot,
  Coins,
  Cpu,
  FolderGit2,
  Lightbulb,
  Play,
  RefreshCw,
  Sparkles,
  Wrench,
} from "lucide-react";

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
  tokensInput: number;
  tokensOutput: number;
}

interface DayCount {
  day: string;
  sessions: number;
}

interface AnalyticsSummary {
  sessions: number;
  projects: number;
  skillCalls: number;
  distinctSkills: number;
  toolCalls: number;
  totalCost: number;
  tokensInput: number;
  tokensOutput: number;
  tokensReasoning?: number;
  tokensCacheRead?: number;
  tokensCacheWrite?: number;
  sessionsWithSkill: number;
  childSessions?: number;
  rootSessions?: number;
  avgToolsPerSession?: number;
  avgCostPerSession?: number;
  skillCoverage?: number;
  delegationCalls?: number;
  installedSkills?: number;
  unusedSkillCount?: number;
}

interface SessionAnalyticsData {
  generatedAt: string;
  dbPath: string;
  days: number;
  projectId?: string;
  summary: AnalyticsSummary;
  insights?: string[];
  activity?: DayCount[];
  toolCategories?: NamedCount[];
  unusedSkills?: string[];
  projects: ProjectStat[];
  skills: NamedCount[];
  tools: NamedCount[];
  agents: NamedCount[];
  models?: NamedCount[];
}

const DAY_OPTIONS = [
  { value: 7, label: "7 days" },
  { value: 30, label: "30 days" },
  { value: 90, label: "90 days" },
  { value: 0, label: "All time" },
];

function formatCost(n: number): string {
  if (!n) return "$0";
  if (n < 0.01) return `$${n.toFixed(4)}`;
  return `$${n.toFixed(2)}`;
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

function formatShare(share?: number): string {
  if (share == null || Number.isNaN(share)) return "—";
  return `${(share * 100).toFixed(share >= 0.1 ? 0 : 1)}%`;
}

function maxCount(items: NamedCount[]): number {
  return items.reduce((m, x) => Math.max(m, x.count), 0) || 1;
}

function RankList({
  items,
  unit = "sessions",
  showCost = false,
  limit,
}: {
  items: NamedCount[];
  unit?: string;
  showCost?: boolean;
  limit?: number;
}) {
  const list = limit ? items.slice(0, limit) : items;
  const max = maxCount(list);
  if (list.length === 0) {
    return <p className="empty-desc muted">No data in this range.</p>;
  }
  return (
    <ol className="sa-rank-list">
      {list.map((item, i) => {
        const pct = Math.max(3, Math.round((item.count / max) * 100));
        return (
          <li key={item.name} className="sa-rank-row">
            <span className="sa-rank-n tnum">{i + 1}</span>
            <div className="sa-rank-main">
              <div className="sa-rank-top">
                <span className="sa-rank-name" title={item.name}>
                  {item.name}
                </span>
                <span className="sa-rank-meta tnum">
                  <strong>{item.count.toLocaleString()}</strong>
                  <span className="muted"> {unit}</span>
                  <span className="sa-rank-share">{formatShare(item.share)}</span>
                </span>
              </div>
              <div className="sa-bar-track" aria-hidden>
                <div className="sa-bar-fill" style={{ width: `${pct}%` }} />
              </div>
              {showCost && (item.cost || item.tokensInput) ? (
                <div className="sa-rank-sub muted tnum">
                  {formatCost(item.cost ?? 0)}
                  {item.tokensInput != null
                    ? ` · ${formatTokens(item.tokensInput)} in / ${formatTokens(item.tokensOutput ?? 0)} out`
                    : null}
                </div>
              ) : item.sessions != null && unit !== "sessions" ? (
                <div className="sa-rank-sub muted tnum">
                  {item.sessions} session{item.sessions === 1 ? "" : "s"}
                </div>
              ) : null}
            </div>
          </li>
        );
      })}
    </ol>
  );
}

function ActivityChart({ days }: { days: DayCount[] }) {
  if (!days.length) {
    return <p className="empty-desc muted">No daily activity.</p>;
  }
  const max = days.reduce((m, d) => Math.max(m, d.sessions), 0) || 1;
  return (
    <div className="sa-activity" role="img" aria-label="Sessions per day">
      {days.map((d) => {
        const h = Math.max(4, Math.round((d.sessions / max) * 100));
        return (
          <div key={d.day} className="sa-activity-col" title={`${d.day}: ${d.sessions}`}>
            <div className="sa-activity-bar" style={{ height: `${h}%` }} />
            <span className="sa-activity-n tnum">{d.sessions}</span>
            <span className="sa-activity-d">{d.day.slice(5)}</span>
          </div>
        );
      })}
    </div>
  );
}

export default function SessionAnalytics({ autoRun = true }: { autoRun?: boolean }) {
  const [days, setDays] = useState(30);
  const [projectId, setProjectId] = useState("");
  const [data, setData] = useState<SessionAnalyticsData | null>(null);
  const [loading, setLoading] = useState(!!autoRun);
  const [error, setError] = useState<string | null>(null);
  const [lastMs, setLastMs] = useState<number | null>(null);
  const [hasRun, setHasRun] = useState(false);
  const [showAllUnused, setShowAllUnused] = useState(false);

  const fetchData = useCallback(
    async (opts?: { refresh?: boolean }) => {
      setLoading(true);
      setError(null);
      const t0 = performance.now();
      try {
        const params = new URLSearchParams();
        params.set("days", String(days));
        if (projectId) params.set("projectId", projectId);
        // Force-regenerate only when the user clicks Run evaluation.
        if (opts?.refresh) {
          params.set("refresh", "1");
          params.set("_ts", String(Date.now()));
        }
        const res = await fetch(`/api/evals/session-analytics?${params}`, {
          cache: "no-store",
          headers: opts?.refresh ? { "Cache-Control": "no-cache" } : undefined,
        });
        const body = await res.json().catch(() => ({}));
        if (!res.ok) {
          throw new Error(body.error || `${res.status}: ${res.statusText}`);
        }
        setData(body as SessionAnalyticsData);
        setHasRun(true);
        setLastMs(Math.round(performance.now() - t0));
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : String(err));
      } finally {
        setLoading(false);
      }
    },
    [days, projectId],
  );

  // Auto-load on mount / filter change (cached ok). Button always force-refreshes.
  useEffect(() => {
    if (!autoRun && !hasRun) return;
    void fetchData({ refresh: false });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [days, projectId, autoRun]);

  const [allProjects, setAllProjects] = useState<ProjectStat[]>([]);
  useEffect(() => {
    if (data && !projectId && data.projects?.length) {
      setAllProjects(data.projects);
    }
  }, [data, projectId]);

  const projectOptions = allProjects.length ? allProjects : data?.projects ?? [];
  const topAgent = data?.agents?.[0];
  const topSkill = data?.skills?.[0];
  const topModel = data?.models?.[0];
  const skillCoverage =
    data?.summary.skillCoverage ??
    (data && data.summary.sessions > 0
      ? data.summary.sessionsWithSkill / data.summary.sessions
      : 0);

  const unusedPreview = useMemo(() => {
    const list = data?.unusedSkills ?? [];
    if (showAllUnused) return list;
    return list.slice(0, 12);
  }, [data, showAllUnused]);

  // Regenerating rescans multi-gigabyte databases and takes tens of seconds, so it is
  // never triggered implicitly: the cached view loads instantly and a full rescan is
  // confirmed, with the age of what is on screen so the choice is informed.
  const runEval = () => {
    const age = data?.generatedAt
      ? Math.round((Date.now() - new Date(data.generatedAt).getTime()) / 60000)
      : null;
    const shown =
      age === null
        ? "No analysis is loaded yet."
        : `The analysis on screen was generated ${age < 1 ? "less than a minute" : `${age} minute${age === 1 ? "" : "s"}`} ago.`;
    if (!window.confirm(`${shown}\n\nRescan OpenCode now? This re-reads the full session history and usually takes 20-60 seconds.`)) {
      return;
    }
    void fetchData({ refresh: true });
  };

  return (
    <div className="session-analytics">
      <div className="sa-toolbar card card-pad">
        <div className="sa-toolbar-row">
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
          <label className="recall-field sa-project-field">
            <span className="recall-field-label">Project</span>
            <select
              className="input"
              value={projectId}
              onChange={(e) => setProjectId(e.target.value)}
            >
              <option value="">All projects</option>
              {projectOptions.map((p) => (
                <option key={p.id} value={p.id}>
                  {p.name || p.worktree} ({p.sessions})
                </option>
              ))}
            </select>
          </label>
          <button
            type="button"
            className="btn btn-primary sa-run-btn"
            onClick={runEval}
            disabled={loading}
          >
            {loading ? (
              <RefreshCw size={16} className="sa-spin" aria-hidden />
            ) : (
              <Play size={16} aria-hidden />
            )}
            {loading ? "Regenerating…" : "Run evaluation"}
          </button>
        </div>
        <p className="recall-eval-explainer muted">
          Regenerates the full session analysis from OpenCode (
          <code>opencode.db</code>) — same as <code>ywai eval run</code>. Forces a
          fresh scan (no cache).
          {lastMs != null && !loading ? (
            <>
              {" "}
              · last run <span className="tnum">{(lastMs / 1000).toFixed(1)}s</span>
              {data?.generatedAt
                ? ` · ${new Date(data.generatedAt).toLocaleString()}`
                : null}
            </>
          ) : null}
        </p>
      </div>

      {error && (
        <div className="alert alert-danger">
          Could not load session analytics: {error}
        </div>
      )}

      {!hasRun && !loading && !error ? (
        <div className="empty-state sa-empty-run">
          <div className="empty-icon">
            <Play size={24} />
          </div>
          <p className="empty-title">Ready to evaluate</p>
          <p className="empty-desc">
            Click <strong>Run evaluation</strong> to rank agents, skills, models and
            tools from your OpenCode sessions.
          </p>
          <button type="button" className="btn btn-primary" onClick={runEval}>
            <Play size={16} aria-hidden />
            Run evaluation
          </button>
        </div>
      ) : loading && !data ? (
        <div className="skeleton skel-card" style={{ margin: "var(--space-4)" }} aria-busy>
          <div className="skel-line title" />
          <div className="skel-line desc" />
          <p className="muted" style={{ marginTop: "var(--space-3)" }}>
            Scanning OpenCode sessions… regenerating analysis.
          </p>
        </div>
      ) : data ? (
        <>
          {loading && (
            <div className="alert alert-info sa-regenerating">
              <RefreshCw size={14} className="sa-spin" /> Regenerating analysis…
            </div>
          )}

          {/* Insights */}
          {(data.insights?.length ?? 0) > 0 && (
            <section className="sa-panel card card-pad sa-insights">
              <header className="sa-panel-head">
                <Lightbulb size={16} />
                <h3>Insights</h3>
              </header>
              <ul className="sa-insight-list">
                {data.insights!.map((line) => (
                  <li key={line}>{line}</li>
                ))}
              </ul>
            </section>
          )}

          {/* KPIs */}
          <div className="kpi-grid">
            <div className="kpi">
              <div className="kpi-top">
                <div
                  className="kpi-icon"
                  style={
                    {
                      "--kpi-icon-bg": "rgba(var(--info-rgb), 0.16)",
                      "--kpi-icon-color": "var(--tint-info)",
                    } as CSSProperties
                  }
                >
                  <Activity size={20} />
                </div>
              </div>
              <div className="kpi-value tnum">{data.summary.sessions}</div>
              <div className="kpi-label">Sessions</div>
              <div className="kpi-subtitle">
                {data.summary.projects} projects
                {data.summary.childSessions
                  ? ` · ${data.summary.childSessions} child`
                  : ""}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-top">
                <div
                  className="kpi-icon"
                  style={
                    {
                      "--kpi-icon-bg": "var(--success-soft)",
                      "--kpi-icon-color": "var(--tint-success)",
                    } as CSSProperties
                  }
                >
                  <Bot size={20} />
                </div>
              </div>
              <div className="kpi-value" style={{ fontSize: "var(--text-lg)" }}>
                {topAgent?.name ?? "—"}
              </div>
              <div className="kpi-label">Top agent</div>
              <div className="kpi-subtitle">
                {topAgent
                  ? `${topAgent.count} · ${formatShare(topAgent.share)}`
                  : "—"}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-top">
                <div
                  className="kpi-icon"
                  style={
                    {
                      "--kpi-icon-bg": "rgba(var(--yz-primary-2-rgb), 0.16)",
                      "--kpi-icon-color": "var(--tint-purple)",
                    } as CSSProperties
                  }
                >
                  <Sparkles size={20} />
                </div>
              </div>
              <div className="kpi-value" style={{ fontSize: "var(--text-lg)" }}>
                {topSkill?.name ?? "—"}
              </div>
              <div className="kpi-label">Top skill</div>
              <div className="kpi-subtitle">
                {(skillCoverage * 100).toFixed(0)}% sessions used a skill
                {data.summary.unusedSkillCount
                  ? ` · ${data.summary.unusedSkillCount} unused`
                  : ""}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-top">
                <div
                  className="kpi-icon"
                  style={
                    {
                      "--kpi-icon-bg":
                        "rgba(var(--warning-rgb, 234, 179, 8), 0.16)",
                      "--kpi-icon-color": "var(--warning)",
                    } as CSSProperties
                  }
                >
                  <Cpu size={20} />
                </div>
              </div>
              <div className="kpi-value" style={{ fontSize: "var(--text-lg)" }}>
                {topModel?.name?.split("/").pop() ?? "—"}
              </div>
              <div className="kpi-label">Top model</div>
              <div className="kpi-subtitle">
                {topModel ? formatShare(topModel.share) : "—"}
                {data.summary.avgToolsPerSession
                  ? ` · ${data.summary.avgToolsPerSession.toFixed(1)} tools/sess`
                  : ""}
              </div>
            </div>
          </div>

          <div className="kpi-grid sa-kpi-secondary">
            <div className="kpi">
              <div className="kpi-value tnum">{data.summary.skillCalls}</div>
              <div className="kpi-label">Skill calls</div>
              <div className="kpi-subtitle">
                {data.summary.distinctSkills} distinct
                {data.summary.installedSkills
                  ? ` / ${data.summary.installedSkills} installed`
                  : ""}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-value tnum">
                {data.summary.toolCalls.toLocaleString()}
              </div>
              <div className="kpi-label">Tool calls</div>
              <div className="kpi-subtitle">
                {data.summary.delegationCalls
                  ? `${data.summary.delegationCalls} delegation`
                  : "all tools"}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-value tnum">
                {formatCost(data.summary.totalCost)}
              </div>
              <div className="kpi-label">Cost</div>
              <div className="kpi-subtitle">
                {formatTokens(data.summary.tokensInput)} in ·{" "}
                {formatTokens(data.summary.tokensOutput)} out
                {data.summary.tokensCacheRead
                  ? ` · ${formatTokens(data.summary.tokensCacheRead)} cache`
                  : ""}
              </div>
            </div>
            <div className="kpi">
              <div className="kpi-value tnum">
                <Coins size={18} style={{ verticalAlign: "middle" }} />{" "}
                {formatCost(data.summary.avgCostPerSession ?? 0)}
              </div>
              <div className="kpi-label">Avg / session</div>
              <div className="kpi-subtitle">
                {(data.summary.avgToolsPerSession ?? 0).toFixed(1)} tools avg
              </div>
            </div>
          </div>

          {/* Activity */}
          {(data.activity?.length ?? 0) > 0 && (
            <section className="sa-panel card card-pad">
              <header className="sa-panel-head">
                <Activity size={16} />
                <h3>Activity by day</h3>
                <span className="muted tnum">{data.activity!.length} days</span>
              </header>
              <ActivityChart days={data.activity!} />
            </section>
          )}

          {/* Rankings */}
          <div className="sa-grid sa-grid-3">
            <section className="sa-panel card card-pad sa-rank-panel">
              <header className="sa-panel-head">
                <Bot size={16} />
                <h3>Most used agents</h3>
                <span className="muted tnum">{data.agents.length}</span>
              </header>
              <RankList items={data.agents} unit="sessions" showCost limit={15} />
            </section>
            <section className="sa-panel card card-pad sa-rank-panel">
              <header className="sa-panel-head">
                <Sparkles size={16} />
                <h3>Most used skills</h3>
                <span className="muted tnum">{data.skills.length}</span>
              </header>
              <RankList items={data.skills} unit="calls" limit={15} />
            </section>
            <section className="sa-panel card card-pad sa-rank-panel">
              <header className="sa-panel-head">
                <Cpu size={16} />
                <h3>Most used models</h3>
                <span className="muted tnum">{data.models?.length ?? 0}</span>
              </header>
              <RankList
                items={data.models ?? []}
                unit="sessions"
                showCost
                limit={15}
              />
            </section>
          </div>

          <div className="sa-grid">
            <section className="sa-panel card card-pad">
              <header className="sa-panel-head">
                <Wrench size={16} />
                <h3>Tool mix</h3>
              </header>
              <RankList
                items={data.toolCategories ?? []}
                unit="calls"
                limit={10}
              />
              <header
                className="sa-panel-head"
                style={{ marginTop: "var(--space-4)" }}
              >
                <Wrench size={16} />
                <h3>Top tools</h3>
              </header>
              <RankList items={data.tools} unit="calls" limit={12} />
            </section>

            <section className="sa-panel card card-pad">
              <header className="sa-panel-head">
                <FolderGit2 size={16} />
                <h3>By project</h3>
              </header>
              {data.projects.length === 0 ? (
                <p className="empty-desc muted">No projects.</p>
              ) : (
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Project</th>
                        <th>Sessions</th>
                        <th>Skills</th>
                        <th>Tools</th>
                        <th>Cost</th>
                        <th>Path</th>
                      </tr>
                    </thead>
                    <tbody>
                      {data.projects.map((p) => (
                        <tr
                          key={p.id}
                          className={
                            projectId === p.id
                              ? "selected clickable"
                              : "clickable"
                          }
                          onClick={() =>
                            setProjectId(projectId === p.id ? "" : p.id)
                          }
                          title="Filter by this project"
                        >
                          <td>
                            <strong>{p.name}</strong>
                          </td>
                          <td className="tnum">{p.sessions}</td>
                          <td className="tnum">{p.skillCalls}</td>
                          <td className="tnum">
                            {p.toolCalls.toLocaleString()}
                          </td>
                          <td className="tnum">{formatCost(p.cost)}</td>
                          <td className="cell-mono cell-muted sa-path">
                            {p.worktree}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {(data.unusedSkills?.length ?? 0) > 0 && (
                <>
                  <header
                    className="sa-panel-head"
                    style={{ marginTop: "var(--space-4)" }}
                  >
                    <Sparkles size={16} />
                    <h3>Installed skills not used</h3>
                    <span className="muted tnum">
                      {data.unusedSkills!.length}
                    </span>
                  </header>
                  <div className="sa-chips">
                    {unusedPreview.map((name) => (
                      <span key={name} className="sa-chip">
                        {name}
                      </span>
                    ))}
                  </div>
                  {data.unusedSkills!.length > 12 && (
                    <button
                      type="button"
                      className="btn btn-secondary btn-sm"
                      style={{ marginTop: 8 }}
                      onClick={() => setShowAllUnused((v) => !v)}
                    >
                      {showAllUnused
                        ? "Show less"
                        : `Show all ${data.unusedSkills!.length}`}
                    </button>
                  )}
                </>
              )}
            </section>
          </div>

          <p className="sa-footnote muted">
            Generated {new Date(data.generatedAt).toLocaleString()} ·{" "}
            {data.days === 0 ? "All time" : `Last ${data.days} days`} ·{" "}
            <span className="cell-mono">{data.dbPath}</span>
          </p>
        </>
      ) : null}
    </div>
  );
}
