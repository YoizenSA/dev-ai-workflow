import { useState, useEffect } from "react";
import { Users, Loader, CheckCircle, AlertCircle } from "lucide-react";
import type { Session } from "./types";
const POLL_INTERVAL = 3000;

// Merge polled children with live SSE updates — live entries override by id.
function mergeChildren(polled: Session[], live?: Record<string, Session>): Session[] {
  if (!live) return polled;
  return polled.map((c) => live[c.id] ?? c);
}

interface SubagentStripProps {
  children: Session[];
  activeSession: string | null;
  isStreaming: boolean;
  liveChildren?: Record<string, Session>;
}

export default function SubagentStrip({ children: initial, activeSession, isStreaming, liveChildren }: SubagentStripProps) {
  const [subagents, setSubagents] = useState(initial);

  // Sync when parent prop changes (session switch, loading completes)
  useEffect(() => { setSubagents(initial); }, [initial]);

  // Poll for live updates while streaming
  useEffect(() => {
    if (!isStreaming || !activeSession) return;
    const id = setInterval(async () => {
      try {
        const resp = await fetch(`/api/chat/sessions/${activeSession}/children`);
        if (resp.ok) {
          const data = await resp.json();
          setSubagents(data.children || []);
        }
      } catch { /* poll silently */ }
    }, POLL_INTERVAL);
    return () => clearInterval(id);
  }, [activeSession, isStreaming]);

  if (subagents.length === 0) return null;

  return (
    <div className="subagents-strip">
      <span className="subagents-label">
        <Users size={16} /> Subagents
      </span>
      {mergeChildren(subagents, liveChildren).map((c) => {
        const running = c.time?.completed == null;
        const error = c.time?.completed != null && c.time.completed < 0;
        // ponytail: negative completed ts signals error in opencode sessions
        return (
          <span
            key={c.id}
            className={`subagent-chip ${running ? "running" : error ? "error" : "done"}`}
            title={error ? "Subagent failed" : running ? "Running\u2026" : c.title}
          >
            {error ? <AlertCircle size={14} /> : running ? <Loader size={14} className="spin pulse" /> : <CheckCircle size={14} />}
            {c.title || c.id.slice(0, 8)}
          </span>
        );
      })}
    </div>
  );
}
