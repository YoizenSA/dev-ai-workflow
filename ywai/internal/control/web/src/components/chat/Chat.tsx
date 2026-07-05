import { useState, useEffect, useRef, useCallback } from "react";
import { useSearchParams } from "react-router-dom";
import {
  Plus,
  Star,
  Send,
  Square,
  User,
  Bot,
  FileText,
  Sparkles,
  Trash2,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRight,
  Menu,
  X,
  Folder,
  ChevronRight,
} from "lucide-react";
import Autocomplete, { type AutocompleteItem } from "./Autocomplete";
import Markdown from "./Markdown";
import { ThinkingBlock, ToolBlock, SubagentBlock } from "./PartBlock";
import RightPanel, { type RightTab } from "./RightPanel";
import ModelSearchSelect from "./ModelSearchSelect";
import AgentSearchSelect from "./AgentSearchSelect";
import WorkspaceSearchSelect from "./WorkspaceSearchSelect";
import "./Chat.css";
import { API_BASE, getEventStreamURL, getMessagesURL, deleteMessage, revertToMessage, sendQuestionReply, startOpencode, getSessionInfo, abortSession, fetchContextUsage } from "../../api/chat";
import TodoDisplay from "./TodoDisplay";
import MessageActions from "./MessageActions";
import PermissionDialog, { type PermissionRequest } from "./PermissionDialog";
import QuestionPrompt, { type QuestionRequest } from "./QuestionPrompt";

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
  parentID?: string;
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

const HELP_TEXT = `**Available commands:**
- \`/new\` — Start a new chat session
- \`/compact\` — Compact the current session (sends to OpenCode)
- \`/clear\` — Clear messages (UI only)
- \`/help\` — Show this help

**@file mentions**
Type \`@\` followed by a filename to search and insert file references.`;

export default function Chat() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const [activeSession, setActiveSession] = useState<string | null>(
    () => searchParams.get("session")
  );
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [ctxUsage, setCtxUsage] = useState<{ used: number; total: number; pct: number } | null>(null);
  // Session IDs currently busy, tracked from live status events. OpenCode never
  // sets time.completed on subagent sessions, so this is the only reliable
  // "running" signal for the subagents panel.
  const [busySessions, setBusySessions] = useState<Set<string>>(new Set());
  // opencodeDown is set when the chat backend returns 503 (no opencode server
  // reachable). The banner offers a button to spawn `opencode serve` via ywai.
  const [opencodeDown, setOpencodeDown] = useState(false);
  const [startingOpencode, setStartingOpencode] = useState(false);
  const [todoRefreshKey, setTodoRefreshKey] = useState(0);

  // Pending permission / question dialogs from OpenCode events.
  const [pendingPermission, setPendingPermission] = useState<PermissionRequest | null>(null);
  const [pendingQuestion, setPendingQuestion] = useState<QuestionRequest | null>(null);

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
  // Family navigation: parent of the active session (null for roots) and its
  // siblings (other subagents of the same parent).
  const [parentSession, setParentSession] = useState<Session | null>(null);
  const [siblings, setSiblings] = useState<Session[]>([]);

  // Session pins and prompt templates (persisted server-side in ~/.ywai).
  const [pinned, setPinned] = useState<string[]>([]);
  const [templates, setTemplates] = useState<
    { id: string; name: string; content: string }[]
  >([]);
  const [showTemplates, setShowTemplates] = useState(false);

  // Collapsible sidebar (desktop rail) + mobile off-canvas drawer.
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  // Collapsible right-side panel for subagent (child session) details.
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());
  // Agent (root session) IDs whose subagent children are expanded. Empty by
  // default → subagents stay collapsed under their parent until clicked open.
  const [expandedAgents, setExpandedAgents] = useState<Set<string>>(new Set());
  const [rightPanelOpen, setRightPanelOpen] = useState(false);
  const [rightTab, setRightTab] = useState<RightTab>("subagents");

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

  const scrollToBottom = useCallback((behavior: ScrollBehavior = "smooth") => {
    messagesEndRef.current?.scrollIntoView({ behavior });
  }, []);

  // New messages (streaming tokens, replies) scroll smoothly to the bottom.
  useEffect(() => {
    scrollToBottom("smooth");
  }, [messages, scrollToBottom]);

  // Switching sessions should land at the bottom right away, without the
  // animated scroll-from-top that smooth scrolling produces on a fresh load.
  const prevSessionRef = useRef<string | null>(null);
  useEffect(() => {
    if (prevSessionRef.current !== activeSession) {
      prevSessionRef.current = activeSession;
      // Defer one frame so the freshly loaded messages are in the DOM.
      requestAnimationFrame(() => scrollToBottom("instant"));
    }
  }, [activeSession, scrollToBottom]);

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

  // Keep ?session=<id> in the URL in sync with the active session so a page
  // reload (F5) or shared link reopens the same session. Mirrors Kanban/Missions.
  useEffect(() => {
    const next = new URLSearchParams(searchParams);
    if (activeSession) next.set("session", activeSession);
    else next.delete("session");
    setSearchParams(next, { replace: true });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeSession]);

  const loadSessions = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`);
      if (resp.status === 503) {
        // Backend has no opencode server to proxy to — surface the start button.
        setOpencodeDown(true);
        return;
      }
      if (resp.ok) {
        setOpencodeDown(false);
        const data = await resp.json();
        setSessions(data.sessions || []);
      }
    } catch {
      // ignore network errors
    }
  };

  const handleStartOpencode = async () => {
    setStartingOpencode(true);
    setError("");
    try {
      const res = await startOpencode();
      // Give the server a moment to bind, then re-check sessions.
      await new Promise((r) => setTimeout(r, 2500));
      await loadSessions();
      if (res.status === "already_running" || res.status === "started") {
        // Best-effort: if sessions now load, opencodeDown is cleared by loadSessions.
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : "Could not start opencode");
    } finally {
      setStartingOpencode(false);
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
    if (!activeSession) {
      setChildren([]);
      setParentSession(null);
      setSiblings([]);
      return;
    }
    loadChildren(activeSession);
    // Resolve the active session's family for the subagents panel: parent
    // (for "go up") and siblings (other subagents of the same parent).
    let cancelled = false;
    (async () => {
      try {
        const info = await getSessionInfo(activeSession);
        if (cancelled) return;
        const pid = info.parentID;
        if (!pid) {
          setParentSession(null);
          setSiblings([]);
          return;
        }
        // Parent: prefer the already-loaded list, fall back to a metadata fetch.
        const fromList = sessions.find((s) => s.id === pid);
        if (fromList) {
          setParentSession(fromList);
        } else {
          try {
            const pInfo = await getSessionInfo(pid);
            if (!cancelled) {
              setParentSession({
                id: pInfo.id,
                title: pInfo.title,
                time: { created: pInfo.time?.created },
                directory: pInfo.directory,
              });
            }
          } catch {
            setParentSession(null);
          }
        }
        // Siblings: the parent's other children.
        try {
          const resp = await fetch(`${API_BASE}/sessions/${pid}/children`);
          if (cancelled) return;
          if (resp.ok) {
            const data = await resp.json();
            setSiblings((data.children || []) as Session[]);
          }
        } catch {
          setSiblings([]);
        }
      } catch {
        if (!cancelled) {
          setParentSession(null);
          setSiblings([]);
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [activeSession, isStreaming, sessions]);

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
      case "session.created":
      case "session.updated":
        // A child session of the active one — surface it in the subagents panel.
        if (p.info?.id && p.info?.parentID === activeSession) {
          setChildren((prev) => {
            const next = prev.filter((c) => c.id !== p.info.id);
            return [
              ...next,
              {
                id: p.info.id,
                title: p.info.title,
                time: p.info.time,
                directory: p.info.directory,
              },
            ];
          });
        }
        break;
      case "session.status": {
        const busy = p.status?.type === "busy";
        setIsStreaming(busy);
        if (p.sessionID) {
          setBusySessions((prev) => {
            const next = new Set(prev);
            if (busy) next.add(p.sessionID);
            else next.delete(p.sessionID);
            return next;
          });
        }
        break;
      }
      case "session.idle":
        setIsStreaming(false);
        if (p.sessionID) {
          setBusySessions((prev) => {
            const next = new Set(prev);
            next.delete(p.sessionID);
            return next;
          });
        }
        break;
      case "todo.updated":
        setTodoRefreshKey((k) => k + 1);
        break;
      case "permission.updated":
        setPendingPermission({
          id: p.id,
          sessionID: p.sessionID || p.session_id || activeSession || "",
          tool: p.tool || "",
          description: p.description || "",
        });
        break;
      case "question":
        setPendingQuestion({
          id: p.id || p.requestID,
          sessionID: p.sessionID || activeSession || "",
          questions: p.questions || [],
        });
        break;
    }
  };

  // Called when the user dismisses a permission dialog (allow/deny).
  const handlePermissionDone = () => setPendingPermission(null);

  // Called when the user submits an answer to a question.
  const handleQuestionReply = async (answers: string[][]) => {
    if (!pendingQuestion) return;
    await sendQuestionReply(pendingQuestion.sessionID, pendingQuestion.id, answers);
    setPendingQuestion(null);
  };

  // Called when the user skips a question.
  const handleQuestionReject = () => setPendingQuestion(null);

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

  // Refresh context usage when the session changes or a turn finishes.
  useEffect(() => {
    if (!activeSession) {
      setCtxUsage(null);
      return;
    }
    if (isStreaming) return;
    fetchContextUsage(activeSession).then(setCtxUsage);
  }, [activeSession, isStreaming]);

  const handleAbort = async () => {
    await abortSession();
    setIsStreaming(false);
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

  const handleEditMessage = (text: string) => {
    setInput(text);
    textareaRef.current?.focus();
  };

  const handleDeleteMessage = async (messageId: string) => {
    if (!activeSession) return;
    const ok = await deleteMessage(activeSession, messageId);
    if (ok) {
      msgStore.current.delete(messageId);
      rebuildMessages();
    }
  };

  const handleRevertToMessage = async (messageId: string) => {
    if (!activeSession) return;
    const ok = await revertToMessage(activeSession, messageId);
    if (ok) {
      loadMessages(activeSession);
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

  // Nest subagents under their parent within a workspace group. Roots keep the
  // group's sort order; orphan subagents (parent absent from the list) surface
  // as roots so they're never lost.
  const buildAgentTree = (items: Session[]) => {
    const byId = new Set(items.map((s) => s.id));
    const kidsOf = new Map<string, Session[]>();
    const roots: Session[] = [];
    for (const s of items) {
      if (s.parentID && byId.has(s.parentID)) {
        const arr = kidsOf.get(s.parentID) || [];
        arr.push(s);
        kidsOf.set(s.parentID, arr);
      } else {
        roots.push(s);
      }
    }
    return roots.map((root) => ({ root, kids: kidsOf.get(root.id) || [] }));
  };

  const renderSessionItem = (
    s: Session,
    isChild: boolean,
    childCount: number,
    open: boolean,
  ) => (
    <div
      key={s.id}
      className={`session-item ${s.id === activeSession ? "active" : ""} ${isChild ? "is-subagent" : ""}`}
      onClick={() => {
        setActiveSession(s.id);
        setMessages([]);
        loadMessages(s.id);
        setMobileDrawerOpen(false);
      }}
    >
      {childCount > 0 ? (
        <button
          className={`session-agent-toggle ${open ? "open" : ""}`}
          data-tip={open ? "Collapse subagents" : "Expand subagents"}
          aria-label={open ? "Collapse subagents" : "Expand subagents"}
          onClick={(e) => {
            e.stopPropagation();
            setExpandedAgents((prev) => {
              const next = new Set(prev);
              if (next.has(s.id)) next.delete(s.id);
              else next.add(s.id);
              return next;
            });
          }}
        >
          <ChevronRight size={13} />
        </button>
      ) : isChild ? (
        <Bot size={13} className="session-subagent-icon" data-tip="Subagent session" />
      ) : null}
      <span className={`session-title ${s.title ? "" : "is-placeholder"}`}>
        {s.title || `Session ${s.id.slice(0, 8)}`}
      </span>
      {childCount > 0 && (
        <span className="session-subagent-count" data-tip="Subagents">
          {childCount}
        </span>
      )}
      <span className="session-time">{formatTime(s.time?.created)}</span>
      <button
        className={`btn-pin ${pinned.includes(s.id) ? "pinned" : ""}`}
        data-tip={pinned.includes(s.id) ? "Unpin" : "Pin"}
        aria-label={pinned.includes(s.id) ? "Unpin session" : "Pin session"}
        onClick={(e) => togglePin(s.id, e)}
      >
        <Star size={16} fill={pinned.includes(s.id) ? "currentColor" : "none"} />
      </button>
    </div>
  );

  const activeTitle =
    sessions.find((s) => s.id === activeSession)?.title || "New conversation";
  const showWelcome = !!activeSession && messages.length === 0 && !isStreaming;

  return (
    <div
      className={`chat-container ${sidebarCollapsed ? "sidebar-collapsed" : ""} ${mobileDrawerOpen ? "drawer-open" : ""}`}
    >
      {/* Mobile drawer scrim (blurs + dims the conversation behind the drawer) */}
      {mobileDrawerOpen && (
        <div
          className="chat-drawer-scrim"
          onClick={() => setMobileDrawerOpen(false)}
          aria-hidden="true"
        />
      )}

      {/* Session sidebar */}
      <aside className={`chat-sessions ${mobileDrawerOpen ? "drawer-open" : ""}`}>
        <div className="chat-sessions-header">
          <span className="chat-brand">
            <Sparkles size={20} /> Chat
          </span>
          <button
            className="btn-new-session"
            onClick={createSession}
            data-tip="New conversation"
            aria-label="New conversation"
          >
            <Plus size={16} /> <span className="btn-new-session-text">New</span>
          </button>
          {/* Mobile close (only visible inside the drawer) */}
          <button
            className="chat-drawer-close"
            onClick={() => setMobileDrawerOpen(false)}
            aria-label="Close menu"
          >
            <X size={20} />
          </button>
        </div>
        {projects.length > 0 && (
          <div className="workspace-switcher">
            <WorkspaceSearchSelect
              workspaces={projects}
              value={workspaceFilter}
              onChange={setWorkspaceFilter}
            />
          </div>
        )}
        <div className="chat-sessions-list">
          {groupedSessions.map(([dir, items]) => {
            const groupKey = dir || "none";
            const collapsed = collapsedGroups.has(groupKey);
            return (
            <div className="session-group" key={groupKey}>
              <button
                className={`session-group-header ${collapsed ? "collapsed" : ""}`}
                data-tip={dir}
                onClick={() =>
                  setCollapsedGroups((prev) => {
                    const next = new Set(prev);
                    if (next.has(groupKey)) next.delete(groupKey);
                    else next.add(groupKey);
                    return next;
                  })
                }
              >
                <ChevronRight size={14} className="session-group-chevron" />
                <Folder size={16} className="session-group-folder" />
                <span className="session-group-name">{workspaceLabel(dir)}</span>
                <span className="session-group-count">{items.length}</span>
              </button>
              {!collapsed && buildAgentTree(items).map(({ root, kids }) => {
                const agentOpen = expandedAgents.has(root.id);
                return (
                  <div className="agent-node" key={root.id}>
                    {renderSessionItem(root, false, kids.length, agentOpen)}
                    {agentOpen &&
                      kids.map((k) => renderSessionItem(k, true, 0, false))}
                  </div>
                );
              })}
            </div>
            );
          })}
          {sessions.length === 0 && (
            <div className="chat-empty">
              No conversations yet.
              <br />
              Tap <strong>New</strong> to start.
            </div>
          )}
        </div>
        {/* Desktop collapse toggle (rail pattern) */}
        <button
          className="chat-collapse-toggle"
          onClick={() => setSidebarCollapsed((v) => !v)}
          data-tip={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
          aria-expanded={!sidebarCollapsed}
        >
          {sidebarCollapsed ? <PanelLeftOpen size={20} /> : <PanelLeftClose size={20} />}
        </button>
      </aside>

      {/* Main chat area */}
      <main className="chat-main">
        <header className="chat-header">
          {/* Mobile hamburger (only visible below 760px) */}
          <button
            className="chat-drawer-toggle"
            onClick={() => setMobileDrawerOpen((v) => !v)}
            aria-label={mobileDrawerOpen ? "Close menu" : "Open menu"}
            aria-expanded={mobileDrawerOpen}
          >
            {mobileDrawerOpen ? <X size={20} /> : <Menu size={20} />}
          </button>
          <div className="chat-header-title">{activeTitle}</div>
          <div className="chat-header-right">
            {activeSession && (
              <button
                className={`chat-subagents-toggle ${rightPanelOpen ? "active" : ""}`}
                onClick={() => setRightPanelOpen((v) => !v)}
                data-tip="Session tools (subagents, diff, files)"
                aria-label="Toggle session tools panel"
              >
                <PanelRight size={16} />
                {siblings.length + children.length > 0 && (
                  <span className="chat-subagents-badge">
                    {siblings.length + children.length}
                  </span>
                )}
              </button>
            )}
            {ctxUsage && ctxUsage.total > 0 && (
              <div
                className={`chat-context-meter ${ctxUsage.pct >= 80 ? "warn" : ""}`}
                data-tip={`${ctxUsage.used.toLocaleString()} / ${ctxUsage.total.toLocaleString()} tokens`}
                aria-label={`Context usage ${ctxUsage.pct}%`}
              >
                <span className="chat-context-bar">
                  <span
                    className="chat-context-fill"
                    style={{ width: `${Math.min(ctxUsage.pct, 100)}%` }}
                  />
                </span>
                <span className="chat-context-pct">{ctxUsage.pct}%</span>
              </div>
            )}
            <span
              className={`chat-status-dot ${connected ? "on" : "off"}`}
              data-tip={connected ? "Connected" : "Disconnected"}
              aria-label={connected ? "Connected" : "Disconnected"}
            />
          </div>
        </header>

        <div className="chat-content-row">
        <div className="chat-body">
        <div className="chat-messages">
          {!activeSession && (
            <div className="chat-placeholder">
              <Sparkles size={24} className="chat-placeholder-icon" />
              <h2>How can I help you today?</h2>
              <p>Pick a conversation or create a new one to get started.</p>
              <button className="btn-new-session big" onClick={createSession}>
                <Plus size={16} /> New conversation
              </button>
            </div>
          )}

          {showWelcome && (
            <div className="chat-placeholder">
              <Sparkles size={24} className="chat-placeholder-icon" />
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

          {activeSession && (
            <TodoDisplay sessionId={activeSession} refreshKey={todoRefreshKey} />
          )}

          {messages.map((msg, i) => {
            const prev = messages[i - 1];
            const startsNewTurn = !prev || prev.role !== msg.role;
            const msgText = msg.parts
              .filter((p) => p.kind === "text" || !p.kind)
              .map((p) => p.text || "")
              .join("\n");
            return (
              <div
                key={msg.id}
                className={`chat-message ${msg.role} ${startsNewTurn ? "turn-start" : ""}`}
              >
                {msg.role === "user" && (
                  <MessageActions
                    text={msgText}
                    messageId={msg.id}
                    isFirstUser={i === 0}
                    onEdit={handleEditMessage}
                    onDelete={handleDeleteMessage}
                    onRevert={handleRevertToMessage}
                  />
                )}
                <div className="message-avatar">
                  {msg.role === "user" ? <User size={20} /> : <Bot size={20} />}
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
            );
          })}
          {isStreaming && messages[messages.length - 1]?.role !== "assistant" && (
            <div className="chat-message assistant turn-start">
              <div className="message-avatar">
                <Bot size={20} />
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

        {opencodeDown && (
          <div className="chat-banner chat-banner-warn" role="status">
            <div className="chat-banner-text">
              <strong>OpenCode server is not running.</strong>
              <span>The chat needs an opencode server. Start one now or run <code>opencode serve</code> in a terminal.</span>
            </div>
            <button
              className="btn-new-session"
              onClick={handleStartOpencode}
              disabled={startingOpencode}
              data-tip={startingOpencode ? "Starting…" : "Run `opencode serve` via ywai"}
              aria-label="Start opencode server"
            >
              {startingOpencode ? "Starting…" : "Start opencode"}
            </button>
          </div>
        )}

        {error && <div className="chat-error">{error}</div>}

        {/* Permission and question dialogs from OpenCode */}
        {pendingPermission && (
          <div className="dialog-overlay">
            <PermissionDialog request={pendingPermission} onDone={handlePermissionDone} />
          </div>
        )}
        {pendingQuestion && (
          <div className="dialog-overlay">
            <QuestionPrompt
              request={pendingQuestion}
              onReply={handleQuestionReply}
              onReject={handleQuestionReject}
            />
          </div>
        )}

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
                <AgentSearchSelect
                  agents={agents}
                  value={selectedAgent}
                  onChange={setSelectedAgent}
                  disabled={!activeSession}
                />
              )}
              {providers.length > 0 && (
                <ModelSearchSelect
                  providers={providers}
                  value={
                    selectedModel
                      ? `${selectedModel.providerID}::${selectedModel.modelID}`
                      : ""
                  }
                  onChange={(providerID, modelID) =>
                    setSelectedModel({ providerID, modelID })
                  }
                  disabled={!activeSession}
                />
              )}
              <div className="composer-actions">
                <button
                  className="composer-icon"
                  data-tip="Templates"
                  aria-label="Templates"
                  onClick={() => setShowTemplates((v) => !v)}
                  disabled={!activeSession}
                >
                  <FileText size={20} />
                </button>
                {isStreaming ? (
                  <button
                    className="btn-stop"
                    onClick={handleAbort}
                    data-tip="Stop"
                    aria-label="Stop generation"
                  >
                    <Square size={20} />
                  </button>
                ) : (
                  <button
                    className="btn-send"
                    onClick={sendMessage}
                    disabled={!input.trim() || !activeSession}
                    data-tip="Send"
                    aria-label="Send message"
                  >
                    <Send size={20} />
                  </button>
                )}
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
                    data-tip="Save current message as template"
                    aria-label="Save template"
                  >
                    <Plus size={16} /> Save current
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
                      data-tip="Delete template"
                      aria-label={`Delete template ${t.name}`}
                      onClick={(e) => deleteTemplate(t.id, e)}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
        </div>

        <RightPanel
          open={rightPanelOpen && !!activeSession}
          onClose={() => setRightPanelOpen(false)}
          tab={rightTab}
          onTabChange={setRightTab}
          activeSessionId={activeSession}
          workspaceDir={
            sessions.find((s) => s.id === activeSession)?.directory ||
            workspaceFilter
          }
          parent={parentSession}
          siblings={siblings}
          children={children}
          busySessions={busySessions}
          onSelectSession={(id) => {
            setActiveSession(id);
            setMessages([]);
            loadMessages(id);
          }}
          onInsertFile={(f) => {
            setInput((prev) => `${prev}\`${f}\` `);
            textareaRef.current?.focus();
          }}
        />
        </div>
      </main>
    </div>
  );
}
