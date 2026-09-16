import { useEffect, useMemo, useState } from "react";
import { configApi } from "../../api/client";
import type { SkillStandardizeAction, SkillSurface, SkillSurfaceSkill } from "../../api/types";
import Modal from "../shared/Modal";

// SkillSurfacePanel shows every SKILL.md OpenCode can load: the three global
// roots plus the project roots walked up from the server cwd. Same name with
// different bytes is shadowing; same bytes in several places is a duplicate
// (OpenCode loads every copy). Standardize previews and cleans both.

type Filter = "all" | "duplicate" | "shadowed" | "debris";

const FILTER_LABEL: Record<Filter, string> = {
  all: "All",
  duplicate: "Duplicates",
  shadowed: "Shadowed",
  debris: "Debris",
};

const KIND_LABEL: Record<SkillStandardizeAction["kind"], string> = {
  "remove-duplicate": "Duplicate copies",
  "resolve-shadow": "Conflicting copies",
  "delete-empty-dir": "Empty folders",
  "delete-broken-link": "Broken links",
};
const KIND_HELP: Record<SkillStandardizeAction["kind"], string> = {
  "remove-duplicate": "Identical copies. The plan keeps one copy.",
  "resolve-shadow": "Same name with different content. The plan keeps the top-ranked copy.",
  "delete-empty-dir": "Folders with no readable SKILL.md. The plan removes empty folders only.",
  "delete-broken-link": "Links with a missing target. The plan removes the link only.",
};
const FILTER_HELP: Record<Filter, string> = {
  all: "Show every skill name found in the scanned roots.",
  duplicate: "Show names with 2 or more identical copies. OpenCode loads each copy.",
  shadowed: "Show names with 2 or more different contents. OpenCode needs unique names.",
  debris: "Show entries with no readable SKILL.md or with a broken link.",
};
const STATUS_HELP: Record<SkillSurfaceSkill["status"], string> = {
  unique: "One readable copy. No action is needed.",
  duplicate: "Two or more identical copies. OpenCode loads each copy.",
  shadowed: "Same name with different content. Keep one copy.",
  unreadable: "No readable SKILL.md in this entry.",
};
const KIND_ORDER: SkillStandardizeAction["kind"][] = ["remove-duplicate", "resolve-shadow", "delete-empty-dir", "delete-broken-link"];

const hasDebris = (s: SkillSurfaceSkill) => s.entries.some((e) => e.broken || !e.hash);
const msg = (e: unknown) => (e instanceof Error ? e.message : String(e));

export default function SkillSurfacePanel() {
  const [surface, setSurface] = useState<SkillSurface | null>(null);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
  const [query, setQuery] = useState("");
  // Standardize preview state.
  const [planOpen, setPlanOpen] = useState(false);
  const [plan, setPlan] = useState<SkillStandardizeAction[] | null>(null);
  const [dedupe, setDedupe] = useState(true);
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);

  const load = () => {
    configApi
      .listSkillSurface()
      .then((s) => {
        setSurface(s);
        setError("");
      })
      .catch((e) => setError(msg(e)));
  };

  useEffect(load, []);

  const counts = useMemo(() => {
    const skills = surface?.skills ?? [];
    return {
      all: skills.length,
      duplicate: skills.filter((s) => s.status === "duplicate").length,
      shadowed: skills.filter((s) => s.status === "shadowed").length,
      debris: skills.filter(hasDebris).length,
    };
  }, [surface]);

  const visible = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (surface?.skills ?? []).filter((s) => {
      if (filter === "duplicate" && s.status !== "duplicate") return false;
      if (filter === "shadowed" && s.status !== "shadowed") return false;
      if (filter === "debris" && !hasDebris(s)) return false;
      return !q || s.name.toLowerCase().includes(q) || s.entries.some((e) => e.path.toLowerCase().includes(q));
    });
  }, [surface, filter, query]);

  const handleDelete = async (name: string, path: string) => {
    if (!confirm(`Delete skill "${name}" at ${path}?`)) return;
    try {
      await configApi.deleteSurfaceSkill(path);
      setNotice(`Deleted ${name} at ${path}.`);
      load();
    } catch (e) {
      setError(msg(e));
    }
  };

  const loadPlan = async (withDedupe: boolean) => {
    setBusy(true);
    setPlan(null);
    try {
      const res = await configApi.standardizeSkillSurface({ dryRun: true, dedupe: withDedupe });
      const actions = res.actions ?? [];
      setPlan(actions);
      setPicked(new Set(actions.map((a) => a.path)));
    } catch (e) {
      setError(msg(e));
      setPlanOpen(false);
    } finally {
      setBusy(false);
    }
  };

  const openPlan = () => {
    setNotice("");
    setPlanOpen(true);
    void loadPlan(dedupe);
  };

  const applyPlan = async () => {
    setBusy(true);
    try {
      const res = await configApi.standardizeSkillSurface({ dryRun: false, dedupe, paths: [...picked] });
      const deleted = res.deleted?.length ?? 0;
      const failed = res.failed ?? [];
      setNotice(`Standardized: removed ${deleted} item(s).`);
      setError(failed.length ? `${failed.length} item(s) could not be removed: ${failed.map((f) => `${f.path} (${f.error})`).join("; ")}` : "");
      setPlanOpen(false);
      load();
    } catch (e) {
      setError(msg(e));
    } finally {
      setBusy(false);
    }
  };

  const toggle = (paths: string[], on: boolean) =>
    setPicked((prev) => {
      const next = new Set(prev);
      for (const p of paths) {
        if (on) next.add(p);
        else next.delete(p);
      }
      return next;
    });

  if (error && !surface) {
    return <div className="alert alert-danger">{error}</div>;
  }
  if (!surface) {
    return <p className="muted">Loading skill surface…</p>;
  }

  const shadowed = surface.skills.filter((s) => s.status === "shadowed");
  const grouped = KIND_ORDER.map((k) => [k, (plan ?? []).filter((a) => a.kind === k)] as const).filter(([, a]) => a.length > 0);

  return (
    <section className="skill-surface">
      <div className="ss-head">
        <div>
          <h3>Skill surface</h3>
          <p
            className="muted"
            title="Distinct skill names found in all scanned roots. Shadowed means same name with different content. Duplicated means same name with identical copies. Debris means entries with no readable SKILL.md or with a broken link."
          >
            {surface.skills.length} skill(s) visible to OpenCode · {counts.shadowed} shadowed · {counts.duplicate} duplicated ·{" "}
            {counts.debris} with debris · project root {surface.projectDir}
          </p>
          <details className="ss-help">
            <summary>What do these labels mean?</summary>
            <ul>
              <li>
                <strong>Visible</strong> — distinct skill names found in all scanned roots.
              </li>
              <li>
                <strong>Unique</strong> — one readable copy. No action is needed.
              </li>
              <li>
                <strong>Duplicate</strong> — two or more identical copies. OpenCode loads each copy.
              </li>
              <li>
                <strong>Shadowed</strong> — same name with different content. OpenCode needs unique names, so keep one copy.
              </li>
              <li>
                <strong>Debris</strong> — entries with no readable SKILL.md or with a dangling symlink.
              </li>
              <li>
                <strong>Standardize</strong> — previews cleanup. Nothing is deleted until you apply.
              </li>
            </ul>
          </details>
        </div>
        <button
          type="button"
          className="btn btn-primary btn-sm"
          onClick={openPlan}
          disabled={busy}
          title="Preview cleanup: remove broken links, empty folders, and extra copies. Nothing is deleted until you apply."
        >
          Standardize…
        </button>
      </div>

      {notice && <div className="alert alert-success">{notice}</div>}
      {error && <div className="alert alert-danger">{error}</div>}
      {shadowed.length > 0 && (
        <div className="alert alert-danger">
          Shadowed names (same skill, different content — OpenCode requires unique names):{" "}
          {shadowed.map((s) => s.name).join(", ")}
        </div>
      )}

      <div className="ss-toolbar">
        <div className="toggle-seg" role="radiogroup" aria-label="Filter skills">
          {(Object.keys(FILTER_LABEL) as Filter[]).map((f) => (
            <button
              key={f}
              type="button"
              role="radio"
              aria-checked={filter === f}
              className={`seg-btn ${filter === f ? "active" : ""}`}
              onClick={() => setFilter(f)}
              title={FILTER_HELP[f]}
            >
              {FILTER_LABEL[f]} <span className="ss-count">{counts[f]}</span>
            </button>
          ))}
        </div>
        <input
          className="input ss-search"
          placeholder="Search skills or paths…"
          aria-label="Search skills"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
        />
      </div>

      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Skill</th>
              <th>Status</th>
              <th>Locations</th>
            </tr>
          </thead>
          <tbody>
            {visible.map((skill) => (
              <tr key={skill.name}>
                <td className="cell-mono">{skill.name}</td>
                <td>
                  {skill.status === "shadowed" ? (
                    <span className="pill pill-danger" title={STATUS_HELP.shadowed}>
                      shadowed
                    </span>
                  ) : skill.status === "duplicate" ? (
                    <span className="pill ss-pill-dup" title={STATUS_HELP.duplicate}>
                      duplicate
                    </span>
                  ) : skill.status === "unreadable" ? (
                    <span className="pill" title={STATUS_HELP.unreadable}>
                      unreadable
                    </span>
                  ) : (
                    <span className="pill pill-accent" title={STATUS_HELP.unique}>
                      unique
                    </span>
                  )}
                </td>
                <td>
                  <div className="ss-chips">
                    {skill.entries.map((e) => (
                      <span
                        key={`${e.location}:${e.path}`}
                        className={`ss-chip ${e.broken || !e.hash ? "is-debris" : ""}`}
                        title={`${e.path}${e.symlink ? ` → ${e.symlink}` : ""}`}
                      >
                        <span>
                          {e.location}
                          {e.hash ? ` · ${e.hash.slice(0, 7)}` : " · ?"}
                          {e.broken ? " · broken link" : ""}
                        </span>
                        <button
                          type="button"
                          className="ss-chip-del"
                          title={`Delete ${skill.name} at ${e.path}`}
                          aria-label={`Delete ${skill.name} at ${e.path}`}
                          onClick={() => handleDelete(skill.name, e.path)}
                        >
                          ×
                        </button>
                      </span>
                    ))}
                  </div>
                </td>
              </tr>
            ))}
            {!visible.length && (
              <tr>
                <td colSpan={3} className="muted">
                  {surface.skills.length ? "No skills match this filter." : "No skills found in any scanned location."}
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <p
        className="muted"
        style={{ marginTop: "var(--space-2)" }}
        title="Each root that was scanned for SKILL.md files. Missing means the folder does not exist."
      >
        Scanned:{" "}
        {surface.locations
          .map((l) => `${l.label} (${l.found ? l.path : "missing"})`)
          .join(" · ")}
      </p>

      <Modal
        open={planOpen}
        onClose={() => setPlanOpen(false)}
        title="Standardize skills"
        subtitle="Preview — nothing is deleted until you apply."
        width="820px"
        footer={
          <>
            <button type="button" className="btn btn-ghost" onClick={() => setPlanOpen(false)} disabled={busy}>
              Cancel
            </button>
            <button type="button" className="btn btn-danger" onClick={() => void applyPlan()} disabled={busy || !plan || picked.size === 0}>
              {busy && plan ? "Applying…" : `Apply ${picked.size} change(s)`}
            </button>
          </>
        }
      >
        <label className="ss-opt">
          <input
            type="checkbox"
            checked={dedupe}
            disabled={busy}
            onChange={(e) => {
              setDedupe(e.target.checked);
              void loadPlan(e.target.checked);
            }}
          />
          <span>
            <strong>Also remove identical duplicate copies</strong>
            <span className="field-help">
              Keeps the copy the most tools can read: project → ~/.agents/skills → ~/.claude/skills → ~/.config/opencode/skills.
            </span>
          </span>
        </label>
        {plan === null ? (
          <p className="muted">Planning…</p>
        ) : plan.length === 0 ? (
          <p className="muted">Nothing to standardize — every skill has exactly one clean copy.</p>
        ) : (
          <div className="ss-plan">
            {grouped.map(([kind, actions]) => {
              const on = actions.filter((a) => picked.has(a.path)).length;
              return (
                <div key={kind} className="ss-plan-group">
                  <div className="ss-plan-head" title={KIND_HELP[kind]}>
                    <strong>{KIND_LABEL[kind]}</strong>
                    <span className="field-help ss-plan-kind-help">{KIND_HELP[kind]}</span>
                    <span className="ss-count">
                      {on}/{actions.length}
                    </span>
                    <button
                      type="button"
                      className="ss-link"
                      onClick={() => toggle(actions.map((a) => a.path), on !== actions.length)}
                    >
                      {on === actions.length ? "Select none" : "Select all"}
                    </button>
                  </div>
                  {actions.map((a) => (
                    <label key={a.path} className="ss-plan-row">
                      <input type="checkbox" checked={picked.has(a.path)} onChange={(e) => toggle([a.path], e.target.checked)} />
                      <span className="cell-mono ss-plan-name">{a.name}</span>
                      <span className="ss-plan-path" title={a.path}>
                        {a.path}
                      </span>
                      <span className="ss-plan-detail">{a.detail}</span>
                    </label>
                  ))}
                </div>
              );
            })}
          </div>
        )}
      </Modal>
    </section>
  );
}
