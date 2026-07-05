import {
  Plus,
  Star,
  Sparkles,
  MessageSquarePlus,
  PanelLeftClose,
  PanelLeftOpen,
  X,
} from "lucide-react";
import type { Session } from "./types";

interface SidebarProps {
  sessions: Session[];
  activeSession: string | null;
  pinned: string[];
  projects: { id: string; path: string; name: string }[];
  workspaceFilter: string;
  sidebarCollapsed: boolean;
  mobileDrawerOpen: boolean;
  onSelectSession: (id: string) => void;
  onCreateSession: () => void;
  onTogglePin: (id: string, e: React.MouseEvent) => void;
  onWorkspaceFilterChange: (filter: string) => void;
  onToggleCollapse: () => void;
  onToggleDrawer: () => void;
}

function workspaceLabel(dir?: string): string {
  if (!dir) return "No workspace";
  const parts = dir.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || dir;
}

function formatTime(ms?: number): string {
  if (!ms) return "";
  const d = new Date(ms);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return sameDay
    ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleDateString([], { day: "2-digit", month: "short" });
}

export default function Sidebar({
  sessions,
  activeSession,
  pinned,
  projects,
  workspaceFilter,
  sidebarCollapsed,
  mobileDrawerOpen,
  onSelectSession,
  onCreateSession,
  onTogglePin,
  onWorkspaceFilterChange,
  onToggleCollapse,
  onToggleDrawer,
}: SidebarProps) {
  // Sort: pinned first, then by created time desc, falling back to completed time.
  const sortedSessions = [...sessions].sort((a, b) => {
    const aPinned = pinned.includes(a.id) ? 1 : 0;
    const bPinned = pinned.includes(b.id) ? 1 : 0;
    if (aPinned !== bPinned) return bPinned - aPinned;
    const pa = a.time?.created || 0;
    const pb = b.time?.created || 0;
    if (pa !== pb) return pb - pa;
    return (b.time?.created || 0) - (a.time?.created || 0);
  });

  // Group sessions by workspace (directory)
  const groupedSessions: [string, Session[]][] = [];
  const groupIndex = new Map<string, Session[]>();
  for (const s of sortedSessions) {
    const key = s.directory || "";
    let bucket = groupIndex.get(key);
    if (!bucket) {
      bucket = [];
      groupIndex.set(key, bucket);
      groupedSessions.push([key, bucket]);
    }
    bucket.push(s);
  }

  const filteredSessions =
    workspaceFilter
      ? groupedSessions.filter(([dir]) => dir === workspaceFilter)
      : groupedSessions;

  return (
    <aside
      className={`chat-sidebar ${sidebarCollapsed ? "collapsed" : ""} ${
        mobileDrawerOpen ? "drawer-open" : ""
      }`}
    >
      <div className="chat-sessions-header">
        <span className="chat-brand">
          <Sparkles size={20} />
          <span className="chat-brand-text">Chat</span>
        </span>
        <button
          className="btn-new-session"
          onClick={onCreateSession}
          data-tip="New conversation"
          aria-label="New conversation"
        >
          <Plus size={16} />
          <span className="btn-new-session-text">New</span>
        </button>
        {/* Mobile close affordance */}
        <button
          className="chat-drawer-close"
          onClick={onToggleDrawer}
          aria-label="Close menu"
        >
          <X size={16} />
        </button>
      </div>

      <div className="chat-sidebar-scroll">
        <select
          className="chat-filter"
          value={workspaceFilter}
          onChange={(e) => onWorkspaceFilterChange(e.target.value)}
          aria-label="Filter by workspace"
        >
          <option value="">All workspaces</option>
          {projects.map((p) => (
            <option key={p.id} value={p.path}>
              {p.name}
            </option>
          ))}
        </select>

        {filteredSessions.length === 0 && (
          <div className="chat-empty">
            No conversations yet.
            <br />
            Tap <strong>New</strong> to start.
          </div>
        )}

        {filteredSessions.map(([dir, items]) => {
          const groupPinned = items.some((s) => pinned.includes(s.id));
          return (
            <div className="session-group" key={dir || "none"}>
              <div className="session-group-label">
                <span>{workspaceLabel(dir)}</span>
                {groupPinned && <Star size={12} fill="currentColor" />}
              </div>
              {items.map((s) => {
                const isPinned = pinned.includes(s.id);
                return (
                  <div
                    key={s.id}
                    className={`session-item ${s.id === activeSession ? "active" : ""}`}
                    onClick={() => onSelectSession(s.id)}
                  >
                    <MessageSquarePlus size={16} className="session-icon" />
                    <span className="session-title">
                      {s.title || `Session ${s.id.slice(0, 8)}`}
                    </span>
                    <span className="session-time">
                      {formatTime(s.time?.created)}
                    </span>
                    <button
                      className={`btn-pin ${isPinned ? "pinned" : ""}`}
                      data-tip={isPinned ? "Unpin" : "Pin"}
                      aria-label={isPinned ? "Unpin session" : "Pin session"}
                      onClick={(e) => onTogglePin(s.id, e)}
                    >
                      <Star
                        size={14}
                        fill={isPinned ? "currentColor" : "none"}
                      />
                    </button>
                  </div>
                );
              })}
            </div>
          );
        })}
      </div>

      <button
        className="chat-collapse-toggle"
        onClick={onToggleCollapse}
        data-tip={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
        aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
      >
        {sidebarCollapsed ? <PanelLeftOpen size={16} /> : <PanelLeftClose size={16} />}
      </button>
    </aside>
  );
}
