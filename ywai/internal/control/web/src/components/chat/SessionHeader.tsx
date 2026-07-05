import { useState, useEffect } from "react";
import { Menu, X, Pencil, Trash2 } from "lucide-react";
import { renameSession, deleteSession, fetchContextUsage } from "../../api/chat";

interface ContextInfo {
  used: number;
  total: number;
  pct: number;
}

interface SessionHeaderProps {
  sessionId: string | null;
  title: string;
  connected: boolean;
  onToggleDrawer: () => void;
  mobileDrawerOpen: boolean;
  onDelete: () => void;
  onRename: (title: string) => void;
}

export default function SessionHeader({
  sessionId,
  title,
  connected,
  onToggleDrawer,
  mobileDrawerOpen,
  onDelete,
  onRename,
}: SessionHeaderProps) {
  const [editing, setEditing] = useState(false);
  const [editValue, setEditValue] = useState(title);
  const [showConfirm, setShowConfirm] = useState(false);
  const [context, setContext] = useState<ContextInfo | null>(null);

  // Fetch context usage when session changes
  useEffect(() => {
    if (!sessionId) { setContext(null); return; }
    fetchContextUsage(sessionId).then(setContext);
  }, [sessionId]);

  const commitRename = async () => {
    const trimmed = editValue.trim();
    if (trimmed && trimmed !== title && sessionId) {
      const ok = await renameSession(sessionId, trimmed);
      if (ok) onRename(trimmed);
    }
    setEditing(false);
  };

  const handleDelete = async () => {
    if (!sessionId) return;
    const ok = await deleteSession(sessionId);
    if (ok) onDelete();
    setShowConfirm(false);
  };

  return (
    <header className="chat-header">
      {/* Mobile hamburger */}
      <button
        className="chat-drawer-toggle"
        onClick={onToggleDrawer}
        aria-label={mobileDrawerOpen ? "Close menu" : "Open menu"}
      >
        {mobileDrawerOpen ? <X size={20} /> : <Menu size={20} />}
      </button>
      <div className="chat-header-left">
        {editing ? (
          <input
            className="chat-header-edit"
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            onBlur={commitRename}
            onKeyDown={(e) => { if (e.key === "Enter") commitRename(); if (e.key === "Escape") setEditing(false); }}
            autoFocus
          />
        ) : (
          <span
            className="chat-header-title"
            onDoubleClick={() => { setEditValue(title); setEditing(true); }}
          >
            {title}
          </span>
        )}
        {sessionId && !editing && (
          <>
            <button className="btn-header-icon" onClick={() => { setEditValue(title); setEditing(true); }} aria-label="Rename" data-tip="Rename">
              <Pencil size={14} />
            </button>
            {showConfirm ? (
              <span className="confirm-delete">
                <button className="btn-confirm-yes" onClick={handleDelete}>Delete</button>
                <button className="btn-confirm-no" onClick={() => setShowConfirm(false)}>Cancel</button>
              </span>
            ) : (
              <button className="btn-header-icon btn-header-delete" onClick={() => setShowConfirm(true)} aria-label="Delete" data-tip="Delete session">
                <Trash2 size={14} />
              </button>
            )}
          </>
        )}
      </div>
      <div className="chat-header-right">
        {context && (
          <span
            className={`context-meter${context.pct >= 80 ? " context-high" : ""}`}
            data-tip={`${context.used.toLocaleString()} / ${context.total.toLocaleString()} tokens (${Math.round(context.pct)}%)`}
          >
            <span className="context-meter-bar" style={{ width: `${Math.min(context.pct, 100)}%` }} />
          </span>
        )}
        <span
          className={`chat-status-dot ${connected ? "on" : "off"}`}
          data-tip={connected ? "Connected" : "Disconnected"}
        />
      </div>
    </header>
  );
}
