import { useState } from "react";
import { ChevronRight, Brain, Wrench, Users } from "lucide-react";
import DiffViewer, { isDiffContent } from "./DiffViewer";

// Inline subagent delegation block (a "subtask" part). Shows which agent was
// spawned and its task; collapsible to reveal the full task description.
export function SubagentBlock({
  agent,
  description,
}: {
  agent?: string;
  description?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className={`part-block subagent ${open ? "open" : ""}`}>
      <button className="part-block-header" onClick={() => setOpen((o) => !o)}>
        <ChevronRight size={16} className="part-chevron" />
        <Users size={16} />
        <span className="part-tool-name">Subagent</span>
        <span className="part-tool-title">{agent || "agent"}</span>
        <span className="part-tool-status">sync</span>
      </button>
      {open && description && (
        <div className="part-block-body">{description}</div>
      )}
    </div>
  );
}

// Collapsible "thinking" (reasoning) block. Collapsed by default.
export function ThinkingBlock({ text }: { text: string }) {
  const [open, setOpen] = useState(false);
  if (!text.trim()) return null;
  return (
    <div className={`part-block thinking ${open ? "open" : ""}`}>
      <button className="part-block-header" onClick={() => setOpen((o) => !o)}>
        <ChevronRight size={16} className="part-chevron" />
        <Brain size={16} />
        <span>Pensando</span>
      </button>
      {open && <div className="part-block-body">{text}</div>}
    </div>
  );
}

const STATUS_LABEL: Record<string, string> = {
  pending: "en cola",
  running: "ejecutando…",
  completed: "listo",
  error: "error",
};

// Collapsible tool-call block. Collapsed by default; shows output when expanded.
export function ToolBlock({
  tool,
  status,
  title,
  output,
}: {
  tool?: string;
  status?: string;
  title?: string;
  output?: string;
}) {
  const [open, setOpen] = useState(false);
  return (
    <div className={`part-block tool status-${status || "pending"} ${open ? "open" : ""}`}>
      <button className="part-block-header" onClick={() => setOpen((o) => !o)}>
        <ChevronRight size={16} className="part-chevron" />
        <Wrench size={16} />
        <span className="part-tool-name">{tool || "Herramienta"}</span>
        {title && <span className="part-tool-title">{title}</span>}
        <span className="part-tool-status">{STATUS_LABEL[status || ""] || status}</span>
      </button>
      {open && output && (
        <div className="part-block-body">
          {isDiffContent(output) ? (
            <DiffViewer content={output} />
          ) : (
            <pre>{output}</pre>
          )}
        </div>
      )}
    </div>
  );
}
