import { useState, useEffect, useRef, useCallback } from "react";
import {
  Plus,
  Star,
  Send,
  User,
  Bot,
  FileText,
  Sparkles,
  MessageSquarePlus,
  Trash2,
  Users,
} from "lucide-react";
import Autocomplete, { type AutocompleteItem } from "./Autocomplete";
import Markdown from "./Markdown";
import { ThinkingBlock, ToolBlock, SubagentBlock } from "./PartBlock";
import "./Chat.css";
import { getEventStreamURL, getMessagesURL } from "../../api/chat";

const SUGGESTIONS = [
  "Explain what this project does",
  "Help me write a message",
  "Summarize a text I paste",
  "Give me ideas to get started",
];

function formatTime(ms?: number): string {
  if (!ms) return "";
  const d = new Date(ms);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return sameDay
    ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleDateString([], { day: "2-digit", month: "short" });
}

interface Session {
  id: string;
  title?: string;
  time?: { created?: number; completed?: number };
  directory?: string;
}

type PartKind = "text" | "reasoning" | "tool" | "subtask";

interface MsgPart {
  id: string;
  kind: PartKind;
  text: string;
  tool?: string;
  status?: string;
  title?: string;
  output?: string;
  agent?: string;
  description?: string;
}

interface Message {
  id: string;
  role: string;
  parts: MsgPart[];
  timestamp: number;
}

// Human-friendly workspace label from an absolute directory path.
function workspaceLabel(dir?: string): string {
  if (!dir) return "No workspace";
  const parts = dir.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || dir;
}

const API_BASE = "/api/chat";

const HELP_TEXT = `**Available commands:**
- \`/new\` — Start a new chat session
- \`/compact\` — Compact the current session (sends to OpenCode)
- \`/clear\` — Clear messages (UI only)
- \`/help\` — Show this help

**@file mentions**
Type \`@\` followed by a filename to search and insert file references.`;

export default function Chat() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");

  // Model selection (per active session, in-memory).
  const [providers, setProviders] = useState<
    { id: string; name: string; models: string[] }[]
  >([]);
  const [selectedModel, setSelectedModel] = useState<{
    providerID: string;
    modelID: string;
  } | null>(null);

  // Agent selection (per active session, in-memory).
  const [agents, setAgents] = useState<
    { name: string; description: string; mode: string }[]
  >([]);
  const [selectedAgent, setSelectedAgent] = useState<string>("");

  // Workspaces (OpenCode projects). Empty filter = all workspaces.
  const [projects, setProjects] = useState<
    { id: string; path: string; name: string }[]
  >([]);
  const [workspaceFilter, setWorkspaceFilter] = useState<string>("");

  // Child (subagent) sessions of the active session, for async subagent viz.
  const [children, setChildren] = useState<Session[]>([]);

  // Session pins and prompt templates (persisted server-side in ~/.ywai).
  const [pinned, setPinned] = useState<string[]>([]);
  const [templates, setTemplates] = useState<
    { id: string; name: string; content: string }[]
  >([]);
  const [showTemplates, setShowTemplates] = useState(false);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Accumulates messages by id and their typed parts (by partID) so streaming
  // deltas and full-part snapshots from OpenCode reconstruct in order. Parts can
  // be plain text, reasoning ("thinking"), or tool calls.
  type StoredMessage = {
    id: string;
    role: string;
    created: number;
    order: string[];
    parts: Map<string, MsgPart>;
  };
  const msgStore = useRef<Map<string, StoredMessage>>(new Map());

  const ensureMsg = (id: string, role?: string, created?: number) => {
    let m = msgStore.current.get(id);
    if (!m) {
      m = { id, role: role || "assistant", created: created || Date.now(), order: [], parts: new Map() };
      msgStore.current.set(id, m);
    }
    if (role) m.role = role;
    if (created) m.created = created;
    return m;
  };

  const ensurePart = (m: StoredMessage, partID: string, kind: PartKind): MsgPart => {
    let p = m.parts.get(partID);
    if (!p) {
      p = { id: partID, kind, text: "" };
      m.parts.set(partID, p);
      m.order.push(partID);
    }
    return p;
  };

  // Full-snapshot upsert of a part (from message.part.updated).
  const setPart = (
    msgID: string,
    partID: string,
    fields: Partial<MsgPart> & { kind: PartKind },
  ) => {
    const p = ensurePart(ensureMsg(msgID), partID, fields.kind);
    Object.assign(p, fields);
  };

  // Incremental text append (from message.part.delta).
  const appendPart = (msgID: string, partID: string, delta: string) => {
    const p = ensurePart(ensureMsg(msgID), partID, "text");
    p.text = (p.text || "") + delta;
  };

  const rebuildMessages = () => {
    const arr = Array.from(msgStore.current.values())
      .sort((a, b) => a.created - b.created)
      .map((m) => ({
        id: m.id,
        role: m.role,
        parts: m.order.map((id) => m.parts.get(id)!).filter(Boolean),
        timestamp: m.created,
      }));
    setMessages(arr);
  };

  // Autocomplete state
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [showFileMenu, setShowFileMenu] = useState(false);
  const [slashQuery, setSlashQuery] = useState("");
  const [slashIndex, setSlashIndex] = useState(0);
  const [fileIndex, setFileIndex] = useState(0);
  const [fileItems, setFileItems] = useState<AutocompleteItem[]>([]);
  const fileFetchRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // ponytail: single cursor pos for both menus, reset on close
  const [cursorPos, setCursorPos] = useState(0);

  const slashCommands: AutocompleteItem[] = [
    { label: "/new", description: "New session", value: "/new" },
    { label: "/compact", description: "Compact session", value: "/compact" },
    { label: "/clear", description: "Clear messages", value: "/clear" },
    { label: "/help", description: "Show commands", value: "/help" },
  ];

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  // Auto-grow the composer textarea to fit its content (capped by CSS max-height).
  useEffect(() => {
    const ta = textareaRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = `${Math.min(ta.scrollHeight, 200)}px`;
  }, [input]);

  useEffect(() => {
    if (activeSession) {
      connectSSE(activeSession);
      loadMessages(activeSession);
    }
    return () => {
      disconnectSSE();
    };
  }, [activeSession]);

  const loadSessions = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`);
      if (resp.ok) {
        const data = await resp.json();
        setSessions(data.sessions || []);
      }
    } catch {
      // ignore
    }
  };

  const loadProviders = async () => {
    try {
      const resp = await fetch(`${API_BASE}/providers`);
      if (!resp.ok) return;
      const data = await resp.json();
      const list = (data.providers || []).map((p: any) => ({
        id: p.id,
        name: p.name || p.id,
        models: Object.keys(p.models || {}).sort(),
      }));
      setProviders(list);
      // Seed the default selection from OpenCode's configured defaults.
      const def = data.default || {};
      const firstProviderID = Object.keys(def)[0];
      if (firstProviderID && def[firstProviderID]) {
        setSelectedModel({
          providerID: firstProviderID,
          modelID: def[firstProviderID],
        });
      }
    } catch {
      // ignore
    }
  };

  const loadAgents = async () => {
    try {
      const resp = await fetch(`${API_BASE}/agents`);
      if (!resp.ok) return;
      const data = await resp.json();
      const list = data.agents || [];
      setAgents(list);
      // Default to "build" (OpenCode's default) when present.
      const build = list.find((a: any) => a.name === "build");
      setSelectedAgent(build ? "build" : list[0]?.name || "");
    } catch {
      // ignore
    }
  };

  const loadProjects = async () => {
    try {
      const resp = await fetch(`${API_BASE}/projects`);
      if (resp.ok) setProjects((await resp.json()).projects || []);
    } catch {
      // ignore
    }
  };

  const loadChildren = async (sessionId: string) => {
    try {
      const resp = await fetch(`${API_BASE}/sessions/${sessionId}/children`);
      if (resp.ok) setChildren((await resp.json()).children || []);
    } catch {
      // ignore
    }
  };

  const loadPins = async () => {
    try {
      const resp = await fetch(`${API_BASE}/pins`);
      if (resp.ok) setPinned((await resp.json()).pins || []);
    } catch {
      // ignore
    }
  };

  const togglePin = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const isPinned = pinned.includes(id);
    try {
      const resp = await fetch(`${API_BASE}/pins/${id}`, {
        method: isPinned ? "DELETE" : "POST",
      });
      if (resp.ok) setPinned((await resp.json()).pins || []);
    } catch {
      // ignore
    }
  };

  const loadTemplates = async () => {
    try {
      const resp = await fetch(`${API_BASE}/templates`);
      if (resp.ok) setTemplates((await resp.json()).templates || []);
    } catch {
      // ignore
    }
  };

  const saveTemplate = async () => {
    const content = input.trim();
    if (!content) return;
    const name = window.prompt("Template name?");
    if (!name) return;
    try {
      const resp = await fetch(`${API_BASE}/templates`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, content }),
      });
      if (resp.ok) loadTemplates();
    } catch {
      // ignore
    }
  };

  const deleteTemplate = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await fetch(`${API_BASE}/templates/${id}`, { method: "DELETE" });
      loadTemplates();
    } catch {
      // ignore
    }
  };

  const insertTemplate = (content: string) => {
    setInput((prev) => (prev ? `${prev}\n${content}` : content));
    setShowTemplates(false);
    textareaRef.current?.focus();
  };

  useEffect(() => {
    loadSessions();
    loadProviders();
    loadAgents();
    loadProjects();
    loadPins();
    loadTemplates();
  }, []);

  // Refresh subagent (child) sessions when the session changes or streaming ends.
  useEffect(() => {
    if (activeSession) loadChildren(activeSession);
    else setChildren([]);
  }, [activeSession, isStreaming]);

  const connectSSE = (sessionId: string) => {
    disconnectSSE();
    const es = new EventSource(getEventStreamURL(sessionId));
    eventSourceRef.current = es;

    es.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data);
        handleSSEEvent(data);
      } catch {
        // ignore parse errors
      }
    };

    es.onerror = () => {
      setConnected(false);
      // Auto-reconnect after 3s
      setTimeout(() => {
        if (activeSession) connectSSE(activeSession);
      }, 3000);
    };

    es.onopen = () => setConnected(true);
  };

  const disconnectSSE = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  };

  // Parses OpenCode's /event stream. Relevant events:
  //   message.updated       -> establishes a message's role/created time
  //   message.part.updated  -> full text for a part
  //   message.part.delta    -> incremental text chunk for a part
  //   session.status/idle   -> streaming indicator
  const handleSSEEvent = (data: any) => {
    const p = data.properties;
    if (!p) return;

    switch (data.type) {
      case "message.updated":
        if (p.info) {
          ensureMsg(p.info.id, p.info.role, p.info.time?.created);
          rebuildMessages();
        }
        break;
      case "message.part.updated": {
        const part = p.part;
        if (!part) break;
        if (part.type === "text") {
          setPart(part.messageID, part.id, { kind: "text", text: part.text || "" });
        } else if (part.type === "reasoning") {
          setPart(part.messageID, part.id, {
            kind: "reasoning",
            text: part.text || "",
          });
        } else if (part.type === "tool") {
          const st = part.state || {};
          const out =
            st.error ||
            (typeof st.output === "string"
              ? st.output
              : st.output
                ? JSON.stringify(st.output, null, 2)
                : "");
          setPart(part.messageID, part.id, {
            kind: "tool",
            tool: part.tool,
            status: st.status,
            title: st.title,
            output: out,
          });
        } else if (part.type === "subtask") {
          setPart(part.messageID, part.id, {
            kind: "subtask",
            agent: part.agent,
            description: part.description || part.prompt || "",
          });
        } else {
          break;
        }
        rebuildMessages();
        break;
      }
      case "message.part.delta":
        if (p.field === "text") {
          appendPart(p.messageID, p.partID, p.delta || "");
          rebuildMessages();
        }
        break;
      case "session.status":
        setIsStreaming(p.status?.type === "busy");
        break;
      case "session.idle":
        setIsStreaming(false);
        break;
    }
  };

  const sendMessage = async () => {
    if (!input.trim() || !activeSession || isStreaming) return;

    // The user message and its echo stream back via /event, so we don't add an
    // optimistic bubble here (it would duplicate the event-sourced one).
    const msgText = input;
    setInput("");
    setError("");
    setIsStreaming(true);

    try {
      await fetch(`${API_BASE}/sessions/${activeSession}/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          content: msgText,
          model: selectedModel || undefined,
          agent: selectedAgent || undefined,
        }),
      });
    } catch {
      setError("Send failed — network error");
      setIsStreaming(false);
    }
  };

  const createSession = async () => {
    try {
      // When a workspace is selected, scope the new session to that directory.
      const qs = workspaceFilter
        ? `?directory=${encodeURIComponent(workspaceFilter)}`
        : "";
      const resp = await fetch(`${API_BASE}/sessions${qs}`, {
        method: "POST",
      });
      if (resp.ok) {
        const data = await resp.json();
        setActiveSession(data.id);
        setMessages([]);
        loadSessions();
      }
    } catch {
      // ignore
    }
  };

  const handleCompact = async () => {
    if (!activeSession) return;
    try {
      await fetch(`${API_BASE}/sessions/${activeSession}/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: "/compact" }),
      });
      setIsStreaming(false);
    } catch {
      // ignore
    }
  };

  const handleClear = () => setMessages([]);

  const loadMessages = async (sessionId: string) => {
    try {
      const resp = await fetch(getMessagesURL(sessionId));
      if (resp.ok) {
        const data = await resp.json();
        msgStore.current = new Map();
        (data.messages || []).forEach((m: any, i: number) => {
          const e = ensureMsg(m.id, m.role, m.created_at || i + 1);
          e.order = [];
          e.parts = new Map();
          (m.parts || []).forEach((p: any, idx: number) => {
            const pid = p.id || `${m.id}-${idx}`;
            e.order.push(pid);
            e.parts.set(pid, {
              id: pid,
              kind: p.kind,
              text: p.text || "",
              tool: p.tool,
              status: p.status,
              title: p.title,
              output: p.output,
              agent: p.agent,
              description: p.description,
            });
          });
        });
        rebuildMessages();
      }
    } catch {
      // ignore
    }
  };

  const fetchFiles = useCallback(async (prefix: string) => {
    try {
      const resp = await fetch(`/api/files?q=${encodeURIComponent(prefix)}`);
      if (resp.ok) {
        const data = await resp.json();
        setFileItems(
          (data.files || []).map((f: string) => ({
            label: f,
            value: f,
          })),
        );
      }
    } catch {
      // ignore
    }
  }, []);

  // Input change handler — detects / and @ triggers
  const handleInputChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    const pos = e.target.selectionStart;
    setInput(val);
    setCursorPos(pos);

    // Check for slash command at start of line
    const beforeCursor = val.slice(0, pos);
    const lineStart = beforeCursor.lastIndexOf("\n") + 1;
    const currentLine = beforeCursor.slice(lineStart);

    // Check for @file mention (word boundary before @)
    const atIdx = beforeCursor.lastIndexOf("@");
    if (atIdx >= 0 && atIdx >= lineStart && !beforeCursor.slice(lineStart, atIdx).includes(" ")) {
      const query = beforeCursor.slice(atIdx + 1);
      // Only trigger if @ is at word start (preceded by space or start)
      const charBefore = atIdx > 0 ? val[atIdx - 1] : " ";
      if (charBefore === " " || charBefore === "\n" || atIdx === 0) {
        setShowSlashMenu(false);
        setFileIndex(0);
        if (!showFileMenu) {
          setShowFileMenu(true);
          fetchFiles(query);
        } else {
          // Debounce fetch
          if (fileFetchRef.current) clearTimeout(fileFetchRef.current);
          fileFetchRef.current = setTimeout(() => fetchFiles(query), 150);
        }
        return;
      }
    }

    // Check for slash command
    if (currentLine.startsWith("/")) {
      const query = currentLine.slice(1);
      setShowFileMenu(false);
      setSlashQuery(query);
      setSlashIndex(0);
      setShowSlashMenu(true);
      return;
    }

    // Close menus if no trigger
    closeMenus();
  };

  const closeMenus = () => {
    setShowSlashMenu(false);
    setShowFileMenu(false);
    setSlashQuery("");
  };

  const executeSlashCommand = (cmd: string) => {
    setInput("");
    closeMenus();
    switch (cmd) {
      case "/new":
        createSession();
        break;
      case "/compact":
        handleCompact();
        break;
      case "/clear":
        handleClear();
        break;
      case "/help": {
        const helpMsg: Message = {
          id: `help-${Date.now()}`,
          role: "assistant",
          parts: [{ id: "help", kind: "text", text: HELP_TEXT }],
          timestamp: Date.now(),
        };
        setMessages((prev) => [...prev, helpMsg]);
        break;
      }
    }
  };

  const insertFileMention = (file: AutocompleteItem) => {
    const before = input.slice(0, cursorPos);
    const atIdx = before.lastIndexOf("@");
    const after = input.slice(cursorPos);
    const newVal = before.slice(0, atIdx) + `@${file.value} ` + after;
    setInput(newVal);
    closeMenus();
    // Focus back on textarea
    textareaRef.current?.focus();
  };

  const handleSlashSelect = (item: AutocompleteItem) => {
    executeSlashCommand(item.value);
  };

  const handleFileSelect = (item: AutocompleteItem) => {
    insertFileMention(item);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    // Autocomplete navigation
    if (showSlashMenu) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setSlashIndex((i) => (i + 1) % slashCommands.length);
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setSlashIndex(
          (i) => (i - 1 + slashCommands.length) % slashCommands.length,
        );
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        const filtered = getFilteredSlashCommands();
        if (filtered[slashIndex]) {
          executeSlashCommand(filtered[slashIndex].value);
        }
        return;
      }
      if (e.key === "Escape") {
        closeMenus();
        return;
      }
    }

    if (showFileMenu) {
      if (e.key === "ArrowDown") {
        e.preventDefault();
        setFileIndex((i) => Math.min(i + 1, fileItems.length - 1));
        return;
      }
      if (e.key === "ArrowUp") {
        e.preventDefault();
        setFileIndex((i) => Math.max(i - 1, 0));
        return;
      }
      if (e.key === "Enter" || e.key === "Tab") {
        e.preventDefault();
        if (fileItems[fileIndex]) {
          insertFileMention(fileItems[fileIndex]);
        }
        return;
      }
      if (e.key === "Escape") {
        closeMenus();
        return;
      }
    }

    // Normal Enter to send
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  const getFilteredSlashCommands = () => {
    if (!slashQuery) return slashCommands;
    return slashCommands.filter((c) =>
      c.value.toLowerCase().includes(slashQuery.toLowerCase()),
    );
  };

  const sortedSessions = [...sessions]
    .filter((s) => !workspaceFilter || s.directory === workspaceFilter)
    .sort((a, b) => {
      const pa = pinned.includes(a.id) ? 1 : 0;
      const pb = pinned.includes(b.id) ? 1 : 0;
      if (pa !== pb) return pb - pa;
      return (b.time?.created || 0) - (a.time?.created || 0);
    });

  // Group sessions by workspace (directory), preserving the sorted order so each
  // group's first entry is its most-recent/pinned session.
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

  const activeTitle =
    sessions.find((s) => s.id === activeSession)?.title || "New conversation";
  const showWelcome = !!activeSession && messages.length === 0 && !isStreaming;

  return (
    <div className="chat-container">
      {/* Session sidebar */}
      <aside className="chat-sessions">
        <div className="chat-sessions-header">
          <span className="chat-brand">
            <Sparkles size={16} /> Chat
          </span>
          <button
            className="btn-new-session"
            onClick={createSession}
            title="New conversation"
          >
            <Plus size={16} /> New
          </button>
        </div>
        {projects.length > 0 && (
          <div className="workspace-switcher">
            <select
              className="workspace-select"
              value={workspaceFilter}
              onChange={(e) => setWorkspaceFilter(e.target.value)}
              title="Filter by workspace (also targets new chats)"
            >
              <option value="">All workspaces</option>
              {projects.map((p) => (
                <option key={p.id} value={p.path}>
                  {p.name}
                </option>
              ))}
            </select>
          </div>
        )}
        <div className="chat-sessions-list">
          {groupedSessions.map(([dir, items]) => (
            <div className="session-group" key={dir || "none"}>
              <div className="session-group-header" title={dir}>
                {workspaceLabel(dir)}
              </div>
              {items.map((s) => (
                <div
                  key={s.id}
                  className={`session-item ${s.id === activeSession ? "active" : ""}`}
                  onClick={() => {
                    setActiveSession(s.id);
                    setMessages([]);
                    loadMessages(s.id);
                  }}
                >
                  <MessageSquarePlus size={15} className="session-icon" />
                  <span className="session-title">
                    {s.title || `Session ${s.id.slice(0, 8)}`}
                  </span>
                  <span className="session-time">
                    {formatTime(s.time?.created)}
                  </span>
                  <button
                    className={`btn-pin ${pinned.includes(s.id) ? "pinned" : ""}`}
                    title={pinned.includes(s.id) ? "Unpin" : "Pin"}
                    onClick={(e) => togglePin(s.id, e)}
                  >
                    <Star
                      size={14}
                      fill={pinned.includes(s.id) ? "currentColor" : "none"}
                    />
                  </button>
                </div>
              ))}
            </div>
          ))}
          {sessions.length === 0 && (
            <div className="chat-empty">
              No conversations yet.
              <br />
              Tap <strong>New</strong> to start.
            </div>
          )}
        </div>
      </aside>

      {/* Main chat area */}
      <main className="chat-main">
        <header className="chat-header">
          <div className="chat-header-title">{activeTitle}</div>
          <div className="chat-header-right">
            <span
              className={`chat-status-dot ${connected ? "on" : "off"}`}
              title={connected ? "Connected" : "Disconnected"}
            />
          </div>
        </header>

        {activeSession && children.length > 0 && (
          <div className="subagents-strip">
            <span className="subagents-label">
              <Users size={13} /> Subagents
            </span>
            {children.map((c) => {
              const running = c.time?.completed == null;
              return (
                <button
                  key={c.id}
                  className={`subagent-chip ${running ? "running" : "done"}`}
                  title={c.title}
                  onClick={() => {
                    setActiveSession(c.id);
                    setMessages([]);
                    loadMessages(c.id);
                  }}
                >
                  <span className="subagent-dot" />
                  {c.title || c.id.slice(0, 8)}
                  <span className="subagent-mode">
                    {running ? "async · running" : "done"}
                  </span>
                </button>
              );
            })}
          </div>
        )}

        <div className="chat-messages">
          {!activeSession && (
            <div className="chat-placeholder">
              <Bot size={40} />
              <h2>How can I help you today?</h2>
              <p>Pick a conversation or create a new one to get started.</p>
              <button className="btn-new-session big" onClick={createSession}>
                <Plus size={16} /> New conversation
              </button>
            </div>
          )}

          {showWelcome && (
            <div className="chat-placeholder">
              <Sparkles size={36} />
              <h2>How can I help?</h2>
              <div className="chat-suggestions">
                {SUGGESTIONS.map((s) => (
                  <button
                    key={s}
                    className="suggestion-card"
                    onClick={() => {
                      setInput(s);
                      textareaRef.current?.focus();
                    }}
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>
          )}

          {messages.map((msg) => (
            <div key={msg.id} className={`chat-message ${msg.role}`}>
              <div className="message-avatar">
                {msg.role === "user" ? <User size={18} /> : <Bot size={18} />}
              </div>
              <div className="message-body">
                <div className="message-role">
                  {msg.role === "user" ? "You" : "Assistant"}
                </div>
                <div className="message-content">
                  {msg.parts.map((part) => {
                    if (part.kind === "reasoning")
                      return <ThinkingBlock key={part.id} text={part.text} />;
                    if (part.kind === "subtask")
                      return (
                        <SubagentBlock
                          key={part.id}
                          agent={part.agent}
                          description={part.description}
                        />
                      );
                    if (part.kind === "tool")
                      return (
                        <ToolBlock
                          key={part.id}
                          tool={part.tool}
                          status={part.status}
                          title={part.title}
                          output={part.output}
                        />
                      );
                    return msg.role === "user" ? (
                      <div key={part.id} className="text-part">
                        {part.text}
                      </div>
                    ) : (
                      <Markdown key={part.id} content={part.text} />
                    );
                  })}
                </div>
              </div>
            </div>
          ))}
          {isStreaming && messages[messages.length - 1]?.role !== "assistant" && (
            <div className="chat-message assistant">
              <div className="message-avatar">
                <Bot size={18} />
              </div>
              <div className="message-body">
                <div className="message-role">Assistant</div>
                <div className="message-content">
                  <span className="typing">
                    <span></span>
                    <span></span>
                    <span></span>
                  </span>
                </div>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {error && <div className="chat-error">{error}</div>}

        {/* Composer */}
        <div className="chat-composer">
          <div className="composer-card">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder={
                activeSession
                  ? "Type your message…  (Enter to send, Shift+Enter for a new line)"
                  : "Create or pick a conversation to start typing"
              }
              rows={1}
              disabled={!activeSession}
            />
            <div className="composer-toolbar">
              {agents.length > 0 && (
                <select
                  className="composer-select"
                  title="Agent"
                  value={selectedAgent}
                  onChange={(e) => setSelectedAgent(e.target.value)}
                >
                  {agents.map((a) => (
                    <option key={a.name} value={a.name} title={a.description}>
                      @{a.name}
                    </option>
                  ))}
                </select>
              )}
              {providers.length > 0 && (
                <select
                  className="composer-select"
                  title="Model"
                  value={
                    selectedModel
                      ? `${selectedModel.providerID}::${selectedModel.modelID}`
                      : ""
                  }
                  onChange={(e) => {
                    const [providerID, modelID] = e.target.value.split("::");
                    setSelectedModel(
                      providerID && modelID ? { providerID, modelID } : null,
                    );
                  }}
                >
                  {providers.map((p) =>
                    p.models.map((m) => (
                      <option key={`${p.id}::${m}`} value={`${p.id}::${m}`}>
                        {p.name} · {m}
                      </option>
                    )),
                  )}
                </select>
              )}
              <div className="composer-actions">
                <button
                  className="composer-icon"
                  title="Templates"
                  onClick={() => setShowTemplates((v) => !v)}
                  disabled={!activeSession}
                >
                  <FileText size={18} />
                </button>
                <button
                  className="btn-send"
                  onClick={sendMessage}
                  disabled={!input.trim() || !activeSession || isStreaming}
                  title="Send"
                >
                  <Send size={18} />
                </button>
              </div>
            </div>

            <Autocomplete
              items={getFilteredSlashCommands()}
              selectedIndex={slashIndex}
              onSelect={handleSlashSelect}
              onClose={closeMenus}
              visible={showSlashMenu}
              anchorEl={textareaRef.current}
            />
            <Autocomplete
              items={fileItems}
              selectedIndex={fileIndex}
              onSelect={handleFileSelect}
              onClose={closeMenus}
              visible={showFileMenu}
              anchorEl={textareaRef.current}
            />

            {showTemplates && (
              <div className="templates-menu">
                <div className="templates-menu-header">
                  <span>Templates</span>
                  <button
                    className="btn-save-template"
                    onClick={saveTemplate}
                    disabled={!input.trim()}
                  >
                    <Plus size={13} /> Save current
                  </button>
                </div>
                {templates.length === 0 && (
                  <div className="templates-empty">
                    No templates yet. Type something and save it here.
                  </div>
                )}
                {templates.map((t) => (
                  <div
                    key={t.id}
                    className="template-item"
                    onClick={() => insertTemplate(t.content)}
                  >
                    <span className="template-name">{t.name}</span>
                    <button
                      className="btn-template-delete"
                      title="Delete"
                      onClick={(e) => deleteTemplate(t.id, e)}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      </main>
    </div>
  );
}
