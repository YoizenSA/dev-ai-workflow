import { useEffect, useState } from "react";
import { configApi } from "../../api/client";
import type { SkillSurface } from "../../api/types";

// SkillSurfacePanel shows every SKILL.md OpenCode can load: the three global
// roots plus the project roots walked up from the server cwd. Same name with
// different bytes in two places is a shadowing warning, not an error.
export default function SkillSurfacePanel() {
  const [surface, setSurface] = useState<SkillSurface | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    configApi
      .listSkillSurface()
      .then(setSurface)
      .catch((e) => setError(e instanceof Error ? e.message : String(e)));
  }, []);

  if (error) {
    return <div className="alert alert-danger">{error}</div>;
  }
  if (!surface) {
    return <p className="muted">Loading skill surface…</p>;
  }

  const shadowed = surface.skills.filter((s) => s.status === "shadowed");

  return (
    <section className="skill-surface" style={{ marginTop: "var(--space-4)" }}>
      <h3>Skill surface</h3>
      <p className="muted">
        {surface.skills.length} skill(s) visible to OpenCode · {shadowed.length} shadowed ·{" "}
        project root {surface.projectDir}
      </p>

      {shadowed.length > 0 && (
        <div className="alert alert-danger">
          Shadowed names (same skill, different content — OpenCode requires unique names):{" "}
          {shadowed.map((s) => s.name).join(", ")}
        </div>
      )}

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
            {surface.skills.map((skill) => (
              <tr key={skill.name}>
                <td className="cell-mono">{skill.name}</td>
                <td>
                  {skill.status === "shadowed" ? (
                    <span className="pill pill-danger">shadowed</span>
                  ) : skill.status === "unreadable" ? (
                    <span className="pill">unreadable</span>
                  ) : (
                    <span className="pill pill-accent">unique</span>
                  )}
                </td>
                <td>
                  {skill.entries.map((e) => (
                    <span
                      key={`${e.location}:${e.path}`}
                      className="pill"
                      title={`${e.path}${e.symlink ? ` → ${e.symlink}` : ""}`}
                      style={{ marginRight: "var(--space-1)" }}
                    >
                      {e.location}
                      {e.hash ? ` · ${e.hash.slice(0, 7)}` : " · ?"}
                      {e.broken ? " · broken link" : ""}
                    </span>
                  ))}
                </td>
              </tr>
            ))}
            {!surface.skills.length && (
              <tr>
                <td colSpan={3} className="muted">
                  No skills found in any scanned location.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <p className="muted" style={{ marginTop: "var(--space-2)" }}>
        Scanned:{" "}
        {surface.locations
          .map((l) => `${l.label} (${l.found ? l.path : "missing"})`)
          .join(" · ")}
      </p>
    </section>
  );
}
