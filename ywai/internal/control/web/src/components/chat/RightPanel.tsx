import { useState, useEffect, useCallback } from "react";
import { X, Bot, GitBranch, FolderTree, RefreshCw } from "lucide-react";
import type { Session } from "./types";
import { SubagentsList } from "./SubagentsPanel";
import DiffViewer from "./DiffViewer";

export type RightTab = "subagents" | "diff" | "files";

interface RightPanelProps {
  open: boolean;
  onClose: () => void;
  tab: RightTab;
  onTabChange: (t: RightTab) => void;
  activeSessionId: string | null;
  // Workspace directory of the active session (for git-scoped diffs).
  workspaceDir: string;
  parent: Session | null;
  siblings: Session[];
  children: Session[];
  // Session IDs currently busy (from live status events).
  busySessions: Set<string>;
  onSelectSession: (id: string) => void;
  // Inserts a file reference into the composer input.
  onInsertFile: (path: string) => void;
  // Panel width in px (controlled by the resize sash). Falls back to CSS default.
  width?: number;
}

const API_BASE = "/api/chat";

// Extract a unified-diff string from whatever the diff endpoint returns.
// OpenCode may return a raw string, an array of { path, patch }, or an object.
function extractDiff(raw: string): string {
  const trimmed = raw.trim();
  if (!trimmed.startsWith("{") && !trimmed.startsWith("[")) return trimmed;
  try {
    const data = JSON.parse(trimmed);
    if (typeof data === "string") return data;
    if (Array.isArray(data)) {
      return data
        .map((d: any) => (typeof d === "string" ? d : d.patch || d.diff || ""))
        .filter(Boolean)
        .join("\n");
    }
    return data.diff || data.patch || "";
  } catch {
    return trimmed;
  }
}

type DiffScope = "session" | "unstaged" | "staged" | "branch";

const SCOPE_LABELS: { value: DiffScope; label: string }[] = [
  { value: "session", label: "Last turn" },
  { value: "unstaged", label: "Unstaged" },
  { value: "staged", label: "Staged" },
  { value: "branch", label: "All branch changes" },
];

function DiffTab({
  sessionId,
  workspaceDir,
}: {
  sessionId: string | null;
  workspaceDir: string;
}) {
  const [diff, setDiff] = useState("");
  const [loading, setLoading] = useState(false);
  const [scope, setScope] = useState<DiffScope>("session");

  const load = useCallback(async () => {
    setLoading(true);
    try {
      let url: string;
      if (scope === "session") {
        if (!sessionId) {
          setDiff("");
          setLoading(false);
          return;
        }
        url = `${API_BASE}/sessions/${sessionId}/diff`;
      } else {
        if (!workspaceDir) {
          setDiff("");
          setLoading(false);
          return;
        }
        url = `${API_BASE}/gitdiff?dir=${encodeURIComponent(workspaceDir)}&scope=${scope}`;
      }
      const resp = await fetch(url);
      setDiff(resp.ok ? extractDiff(await resp.text()) : "");
    } catch {
      setDiff("");
    }
    setLoading(false);
  }, [sessionId, workspaceDir, scope]);

  useEffect(() => {
    load();
  }, [load]);

  return (
    <div className="right-panel-tab-body">
      <div className="right-panel-toolbar">
        <select
          className="right-panel-scope"
          value={scope}
          onChange={(e) => setScope(e.target.value as DiffScope)}
          disabled={scope === "session" ? !sessionId : !workspaceDir}
        >
          {SCOPE_LABELS.map((s) => (
            <option key={s.value} value={s.value}>
              {s.label}
            </option>
          ))}
        </select>
        <button className="right-panel-refresh" onClick={load} data-tip="Refresh">
          <RefreshCw size={14} /> Refresh
        </button>
      </div>
      {loading ? (
        <div className="right-panel-empty">Loading diff…</div>
      ) : diff ? (
        <DiffViewer content={diff} />
      ) : (
        <div className="right-panel-empty">
          {scope === "session"
            ? "No changes in this session."
            : "No changes."}
        </div>
      )}
    </div>
  );
}

function FilesTab({ onInsertFile }: { onInsertFile: (p: string) => void }) {
  const [query, setQuery] = useState("");
  const [files, setFiles] = useState<string[]>([]);

  useEffect(() => {
    const t = setTimeout(async () => {
      try {
        const resp = await fetch(`/api/files?q=${encodeURIComponent(query)}`);
        setFiles(resp.ok ? (await resp.json()).files || [] : []);
      } catch {
        setFiles([]);
      }
    }, 150);
    return () => clearTimeout(t);
  }, [query]);

  return (
    <div className="right-panel-tab-body">
      <input
        className="right-panel-search"
        placeholder="Search files…"
        value={query}
        onChange={(e) => setQuery(e.target.value)}
      />
      {files.length === 0 ? (
        <div className="right-panel-empty">No files.</div>
      ) : (
        <ul className="right-panel-files">
          {files.map((f) => (
            <li key={f}>
              <button
                className="right-panel-file"
                onClick={() => onInsertFile(f)}
                data-tip="Insert as @mention"
              >
                {f}
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

export default function RightPanel({
  open,
  onClose,
  tab,
  onTabChange,
  activeSessionId,
  workspaceDir,
  parent,
  siblings,
  children,
  busySessions,
  onSelectSession,
  onInsertFile,
  width,
}: RightPanelProps) {
  if (!open) return null;

  return (
    <aside
      className="right-panel"
      aria-label="Session tools"
      style={width ? { flex: `0 0 ${width}px`, width } : undefined}
    >
      <header className="right-panel-header">
        <div className="right-panel-tabs" role="tablist">
          <button
            className={`right-panel-tab ${tab === "subagents" ? "active" : ""}`}
            onClick={() => onTabChange("subagents")}
            role="tab"
            aria-selected={tab === "subagents"}
          >
            <Bot size={16} /> Subagents
          </button>
          <button
            className={`right-panel-tab ${tab === "diff" ? "active" : ""}`}
            onClick={() => onTabChange("diff")}
            role="tab"
            aria-selected={tab === "diff"}
          >
            <GitBranch size={16} /> Diff
          </button>
          <button
            className={`right-panel-tab ${tab === "files" ? "active" : ""}`}
            onClick={() => onTabChange("files")}
            role="tab"
            aria-selected={tab === "files"}
          >
            <FolderTree size={16} /> Files
          </button>
        </div>
        <button
          className="right-panel-close"
          onClick={onClose}
          aria-label="Close panel"
          data-tip="Close"
        >
          <X size={18} />
        </button>
      </header>

      {tab === "subagents" && (
        <div className="right-panel-tab-body">
          <SubagentsList
            activeSessionId={activeSessionId}
            parent={parent}
            siblings={siblings}
            children={children}
            busySessions={busySessions}
            onSelectSession={onSelectSession}
          />
        </div>
      )}
      {tab === "diff" && (
        <DiffTab sessionId={activeSessionId} workspaceDir={workspaceDir} />
      )}
      {tab === "files" && <FilesTab onInsertFile={onInsertFile} />}
    </aside>
  );
}
