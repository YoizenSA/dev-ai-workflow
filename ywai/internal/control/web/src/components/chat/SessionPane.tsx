import { useState, useEffect, useRef, useCallback } from "react";
import {
  Send,
  Square,
  Sparkles,
  User,
  Bot,
  FileText,
} from "lucide-react";
import Autocomplete, { type AutocompleteItem } from "./Autocomplete";
import Markdown from "./Markdown";
import { ThinkingBlock, ToolBlock, SubagentBlock } from "./PartBlock";
import MessageActions from "./MessageActions";
import TodoDisplay from "./TodoDisplay";
import RightPanel from "./RightPanel";
import LayoutSash from "./LayoutSash";
import ModelSearchSelect from "./ModelSearchSelect";
import AgentSearchSelect from "./AgentSearchSelect";
import PermissionDialog, { type PermissionRequest } from "./PermissionDialog";
import QuestionPrompt, { type QuestionRequest } from "./QuestionPrompt";
import type { Session, StoredMessage, MsgPart, PartKind } from "./types";
import { configApi } from "../../api/client";
import {
  API_BASE,
  getEventStreamURL,
  getMessagesURL,
  deleteMessage,
  revertToMessage,
  sendQuestionReply,
  abortSession,
  fetchContextUsage,
  executeCommand,
} from "../../api/chat";

type RightTab = "subagents" | "diff" | "files";

interface ContextUsage {
  used: number;
  total: number;
  pct: number;
}

/** Actions Chat can invoke on the active pane (templates, etc.). */
export interface PaneActions {
  insertText(text: string): void;
  focus(): void;
  toggleRightPanel(): void;
  refresh(): void;
}

/** Header state for the focused pane — rendered by Chat in the tab strip. */
export interface PaneHeaderState {
  subagentCount: number;
  rightPanelOpen: boolean;
  ctxPct: number | null;
}

interface SessionPaneProps {
  sessionId: string;
  sessions: Session[];
  workspaceFilter: string;
  busySessions: Set<string>;
  providers: { id: string; name: string; models: string[] }[];
  agents: { name: string; description?: string }[];
  onOpenSession: (id: string) => void;
  onToggleTemplates: () => void;
  paneActionsRef: React.MutableRefObject<PaneActions | null>;
  // Reports this session's live busy state (from SSE) up to Chat, which owns the
  // shared busySessions set. OpenCode session objects carry no status field, so
  // SSE is the only reliable busy signal.
  onBusyChange: (sessionId: string, busy: boolean) => void;
  // Reports the header state (subagent count, right panel, context) up to Chat
  // so it can render the trailing actions inside the tab strip of the focused
  // leaf. Only the focused leaf reports (Chat ignores others).
  isFocused: boolean;
  onHeaderStateChange?: (state: PaneHeaderState) => void;
}

const SUGGESTIONS = [
  "Explain what this project does",
  "Help me write a message",
  "Summarize a text I paste",
  "Give me ideas to get started",
];

export default function SessionPane({
  sessionId,
  sessions,
  workspaceFilter,
  busySessions,
  providers,
  agents,
  onOpenSession,
  onToggleTemplates,
  paneActionsRef,
  onBusyChange,
  isFocused,
  onHeaderStateChange,
}: SessionPaneProps) {
  // --- Message state ---
  const [messages, setMessages] = useState<
    { id: string; role: string; parts: MsgPart[]; timestamp: number }[]
  >([]);
  const msgStore = useRef<Map<string, StoredMessage>>(new Map());
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  // --- Streaming / connection ---
  const [isStreaming, setIsStreaming] = useState(false);
  const [ctxUsage, setCtxUsage] = useState<ContextUsage | null>(null);

  // --- Composer ---
  const [input, setInput] = useState("");
  const [sendError, setSendError] = useState<string | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // --- Model selection ---
  const [selectedModel, setSelectedModel] = useState<{
    providerID: string;
    modelID: string;
  } | null>(null);

  // --- Agent selection (empty = default agent) ---
  const [selectedAgent, setSelectedAgent] = useState("");

  // --- Family (subagent tree) ---
  const [children, setChildren] = useState<Session[]>([]);
  const [parentSession, setParentSession] = useState<Session | null>(null);
  const [siblings, setSiblings] = useState<Session[]>([]);

  // --- Right panel ---
  const [rightPanelOpen, setRightPanelOpen] = useState(false);
  const [rightTab, setRightTab] = useState<RightTab>("subagents");
  // Resizable right-panel width, persisted to localStorage (chat-right-panel-w).
  const [rightPanelWidth, setRightPanelWidth] = useState<number>(() => {
    try {
      const saved = localStorage.getItem("chat-right-panel-w");
      if (saved) return Math.max(240, Math.min(640, parseInt(saved, 10) || 340));
    } catch { /* private mode */ }
    return 340;
  });
  // Persist on every change.
  useEffect(() => {
    try { localStorage.setItem("chat-right-panel-w", String(rightPanelWidth)); }
    catch { /* ignore */ }
  }, [rightPanelWidth]);

  // --- Dialogs ---
  const [pendingPermission, setPendingPermission] =
    useState<PermissionRequest | null>(null);
  const [pendingQuestion, setPendingQuestion] =
    useState<QuestionRequest | null>(null);

  // --- Todo refresh ---
  const [todoRefreshKey, setTodoRefreshKey] = useState(0);

  // --- Autocomplete state ---
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [showFileMenu, setShowFileMenu] = useState(false);
  const [slashQuery, setSlashQuery] = useState("");
  const [slashIndex, setSlashIndex] = useState(0);
  const [fileIndex, setFileIndex] = useState(0);
  const [fileItems, setFileItems] = useState<AutocompleteItem[]>([]);
  const fileFetchRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const slashCommands: AutocompleteItem[] = [
    { label: "/new", description: "New session", value: "/new" },
    { label: "/compact", description: "Compact session", value: "/compact" },
    { label: "/clear", description: "Clear messages", value: "/clear" },
    { label: "/help", description: "Show commands", value: "/help" },
  ];

  // ── Register pane actions for Chat to call ──────────────────────────────
  useEffect(() => {
    paneActionsRef.current = {
      insertText: (text: string) => {
        setInput((prev) => (prev ? `${prev}\n${text}` : text));
        textareaRef.current?.focus();
      },
      focus: () => textareaRef.current?.focus(),
      toggleRightPanel: () => setRightPanelOpen((o) => !o),
      refresh: () => {
        msgStore.current.clear();
        setMessages([]);
        loadMessages(sessionId);
        loadChildren(sessionId);
        loadContextUsage(sessionId);
      },
    };
    return () => {
      paneActionsRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [paneActionsRef, sessionId]);

  // ── Report header state to Chat (for the tab strip trailing actions) ───
  // Only the focused leaf reports; Chat ignores the rest.
  useEffect(() => {
    if (!isFocused || !onHeaderStateChange) return;
    onHeaderStateChange({
      subagentCount: siblings.length + children.length,
      rightPanelOpen,
      ctxPct: ctxUsage && ctxUsage.total > 0 ? ctxUsage.pct : null,
    });
  }, [isFocused, siblings.length, children.length, rightPanelOpen, ctxUsage, onHeaderStateChange]);

  // ── Message store helpers ───────────────────────────────────────────────
  const ensureMsg = (id: string, role?: string, created?: number) => {
    let m = msgStore.current.get(id);
    if (!m) {
      m = {
        id,
        role: role || "assistant",
        created: created || Date.now(),
        order: [],
        parts: new Map(),
      };
      msgStore.current.set(id, m);
    }
    if (role) m.role = role;
    if (created) m.created = created;
    return m;
  };

  const ensurePart = (
    m: StoredMessage,
    partID: string,
    kind: PartKind,
  ): MsgPart => {
    let p = m.parts.get(partID);
    if (!p) {
      p = { id: partID, kind, text: "" };
      m.parts.set(partID, p);
    }
    return p;
  };

  const setPart = (msgID: string, partID: string, patch: Partial<MsgPart>) => {
    const m = msgStore.current.get(msgID);
    if (!m) return;
    const p = m.parts.get(partID);
    if (p) Object.assign(p, patch);
  };

  const rebuildMessages = useCallback(() => {
    const list: {
      id: string;
      role: string;
      parts: MsgPart[];
      timestamp: number;
    }[] = [];
    for (const [, m] of msgStore.current) {
      const parts: MsgPart[] = [];
      for (const pid of m.order) {
        const p = m.parts.get(pid);
        if (p) parts.push(p);
      }
      list.push({ id: m.id, role: m.role, parts, timestamp: m.created });
    }
    list.sort((a, b) => a.timestamp - b.timestamp);
    setMessages(list);
    // ponytail: scroll on next frame so the DOM has updated. Scroll the messages
    // container directly instead of scrollIntoView — the latter walks up and
    // scrolls EVERY scrollable ancestor (incl. the shell's .content), which
    // pushes the pane header off-screen when opening a long session.
    requestAnimationFrame(() => {
      const c = messagesEndRef.current?.parentElement;
      if (c) c.scrollTop = c.scrollHeight;
    });
  }, []);

  // ── SSE connection ─────────────────────────────────────────────────────
  const connectSSE = useCallback(
    (sid: string) => {
      disconnectSSE();
      const es = new EventSource(getEventStreamURL(sid));
      eventSourceRef.current = es;

      es.onmessage = (ev) => {
        try {
          const data = JSON.parse(ev.data);
          handleSSEEvent(data);
        } catch {
          // ignore parse errors
        }
      };

      es.onerror = () => {
        // Reconnect after a delay
        setTimeout(() => {
          if (eventSourceRef.current === es) {
            connectSSE(sid);
          }
        }, 3000);
      };

      es.onopen = () => {};
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [],
  );

  const disconnectSSE = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  };

  // SSE event handler
  const handleSSEEvent = useCallback(
    (data: any) => {
      const p = data.properties;
      if (!p) return;

      // Permission request
      if (data.type === "permission.updated" && p.permission) {
        const perm = p.permission;
        setPendingPermission({
          id: perm.id,
          sessionID: perm.sessionID || sessionId,
          tool: perm.tool || perm.title || "Permission required",
          description: perm.description || "",
        });
        return;
      }

      // Question request
      if (data.type === "question.updated" && p.question) {
        const q = p.question;
        setPendingQuestion({
          id: q.id,
          sessionID: q.sessionID || sessionId,
          questions: q.questions || [],
        });
        return;
      }

      // Session status. The event may target this pane's session OR one of its
      // subagents, so key the busy report by the event's own session id and only
      // flip this pane's streaming flag when it's about this session.
      if (data.type === "session.status" || data.type === "session.idle") {
        const evId = p.sessionID || sessionId;
        // status can be a string ("running") or OpenCode's object ({type:"busy"}).
        const st = p.status;
        const running =
          data.type !== "session.idle" &&
          (st === "running" || st === "busy" ||
            st?.type === "busy" || st?.type === "running");
        onBusyChange(evId, running);
        if (evId === sessionId) setIsStreaming(running);
        return;
      }

      // Todo update
      if (data.type === "todo.updated") {
        setTodoRefreshKey((k) => k + 1);
        return;
      }

      // OpenCode nests event payloads: message.updated → p.info,
      // message.part.updated → p.part, message.part.delta →
      // {messageID, partID, field, delta}. Older/flat emitters put the fields
      // directly on properties — support both shapes.
      const info = p.info || p;
      const part = p.part || p;
      const msgID = part.messageID || info.id || p.messageID || p.id;
      if (!msgID) return;

      if (data.type === "message.updated") {
        const created = info.time?.created ?? info.created_at;
        const m = ensureMsg(msgID, info.role, created);
        if (info.role) m.role = info.role;
        if (created) m.created = created;
        rebuildMessages();
        return;
      }

      if (data.type === "message.part.delta") {
        const partID = part.partID || part.id || `${msgID}-0`;
        const m = ensureMsg(msgID);
        const kind = part.field === "reasoning" ? "reasoning" : "text";
        const existing = ensurePart(m, partID, kind);
        existing.text = (existing.text || "") + (part.delta ?? part.text ?? "");
        if (!m.order.includes(partID)) m.order.push(partID);
        rebuildMessages();
        return;
      }

      if (data.type === "message.part.updated") {
        const m = ensureMsg(msgID);
        const partID = part.id || `${msgID}-0`;

        if (part.type === "text" || !part.type) {
          ensurePart(m, partID, "text");
          setPart(msgID, partID, { text: part.text || "" });
          // Providers that don't send an explicit part `order` (e.g. pi) still
          // need the part registered so rebuildMessages renders it.
          if (!m.order.includes(partID)) m.order.push(partID);
        } else if (part.type === "reasoning") {
          ensurePart(m, partID, "reasoning");
          setPart(msgID, partID, { text: part.text || "" });
          if (!m.order.includes(partID)) m.order.push(partID);
        } else if (part.type === "tool") {
          const st = part.state || {};
          const out =
            st.error ||
            (typeof st.output === "string"
              ? st.output
              : st.output
                ? JSON.stringify(st.output, null, 2)
                : "");
          ensurePart(m, partID, "tool");
          setPart(msgID, partID, {
            kind: "tool",
            tool: part.tool,
            title: st.title || part.tool,
            status: st.status,
            output: out,
            agent: st.agent,
            description: st.description,
          });
          if (!m.order.includes(partID)) {
            m.order.push(partID);
          }
        } else if (part.type === "subtask") {
          ensurePart(m, partID, "subtask");
          setPart(msgID, partID, {
            kind: "subtask",
            agent: part.agent,
            title: part.title,
            status: part.status,
            description: part.description,
          });
          if (!m.order.includes(partID)) {
            m.order.push(partID);
          }
        }

        // Reorder if needed
        if (part.order !== undefined) {
          m.order = part.order;
        }

        rebuildMessages();
        return;
      }

      // Fallback: treat as message update
      if (data.type === "message" || data.type === "message.part") {
        const m = ensureMsg(msgID, p.role, p.created_at);
        if (p.parts) {
          m.order = [];
          m.parts.clear();
          (p.parts || []).forEach((pt: any, idx: number) => {
            const pid = pt.id || `${msgID}-${idx}`;
            m!.order.push(pid);
            m!.parts.set(pid, {
              id: pid,
              kind: pt.kind || pt.type || "text",
              text: pt.text || "",
              tool: pt.tool,
              status: pt.status,
              title: pt.title,
              output: pt.output,
              agent: pt.agent,
              description: pt.description,
            });
          });
        }
        rebuildMessages();
      }
    },
    [sessionId, ensureMsg, ensurePart, setPart, rebuildMessages, onBusyChange],
  );

  // ── Mount: connect SSE + load messages ──────────────────────────────────
  useEffect(() => {
    if (sessionId) {
      msgStore.current.clear();
      setMessages([]);
      connectSSE(sessionId);
      loadMessages(sessionId);
      loadChildren(sessionId);
      loadFamily(sessionId);
      loadContextUsage(sessionId);
    }
    return () => {
      disconnectSSE();
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // ── Load messages ──────────────────────────────────────────────────────
  const loadMessages = async (sid: string) => {
    try {
      const resp = await fetch(getMessagesURL(sid));
      if (!resp.ok) return;
      const data = await resp.json();
      const msgs = data.messages || [];
      msgs.forEach((m: any, i: number) => {
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
    } catch {
      // ignore
    }
  };

  // ── Load family (parent, siblings, children) ───────────────────────────
  const loadChildren = async (sid: string) => {
    try {
      const resp = await fetch(`${API_BASE}/sessions/${sid}/children`);
      if (resp.ok) setChildren((await resp.json()).children || []);
    } catch {
      // ignore
    }
  };

  const loadFamily = async (sid: string) => {
    try {
      const resp = await fetch(`${API_BASE}/sessions/${sid}`);
      if (!resp.ok) return;
      const info = await resp.json();
      const pid = info.parentID;
      if (!pid) {
        setParentSession(null);
        setSiblings([]);
        return;
      }
      // Parent: prefer already-loaded list, fall back to metadata fetch.
      const fromList = sessions.find((s) => s.id === pid);
      if (fromList) {
        setParentSession(fromList);
      } else {
        try {
          const pResp = await fetch(`${API_BASE}/sessions/${pid}`);
          if (pResp.ok) {
            const pInfo = await pResp.json();
            setParentSession({
              id: pInfo.id,
              title: pInfo.title,
              time: { created: pInfo.time?.created },
              directory: pInfo.directory,
              parentID: pInfo.parentID,
            });
          }
        } catch {
          // ignore
        }
      }
      // Siblings
      try {
        const sResp = await fetch(`${API_BASE}/sessions/${pid}/children`);
        if (sResp.ok) {
          const kids = (await sResp.json()).children || [];
          setSiblings(kids.filter((c: Session) => c.id !== sid));
        }
      } catch {
        // ignore
      }
    } catch {
      // ignore
    }
  };

  // ── Context usage ──────────────────────────────────────────────────────
  const loadContextUsage = async (sid: string) => {
    try {
      const data = await fetchContextUsage(sid);
      if (data) setCtxUsage(data);
    } catch {
      // ignore
    }
  };

  // ── Send / Abort ───────────────────────────────────────────────────────
  const sendMessage = async () => {
    if (!input.trim() || isStreaming) return;

    const text = input.trim();
    setInput("");

    // Add user message to store
    const userMsg = ensureMsg(`user-${Date.now()}`, "user", Date.now());
    userMsg.order = ["user-text"];
    userMsg.parts = new Map([
      ["user-text", { id: "user-text", kind: "text", text }],
    ]);
    rebuildMessages();

    try {
      const model = selectedModel
        ? `${selectedModel.providerID}/${selectedModel.modelID}`
        : undefined;
      const resp = await fetch(`${API_BASE}/sessions/${sessionId}/message`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          message: text,
          model,
          agent: selectedAgent || undefined,
        }),
      });
      if (!resp.ok) {
        const detail = (await resp.text().catch(() => "")).trim();
        setSendError(detail || `Failed to send message (HTTP ${resp.status})`);
        console.error("Failed to send message:", resp.status);
        return;
      }
      setSendError(null);
      setIsStreaming(true);
    } catch (err) {
      setSendError("Failed to send message — the chat backend is unreachable.");
      console.error("Failed to send message:", err);
    }
  };

  const handleAbort = async () => {
    try {
      await abortSession();
      setIsStreaming(false);
    } catch {
      // ignore
    }
  };

  // ── Message actions ────────────────────────────────────────────────────
  const handleEditMessage = (text: string) => {
    setInput(text);
    textareaRef.current?.focus();
  };

  const handleDeleteMessage = async (messageId: string) => {
    const ok = await deleteMessage(sessionId, messageId);
    if (ok) {
      msgStore.current.delete(messageId);
      rebuildMessages();
    }
  };

  const handleRevertMessage = async (messageId: string) => {
    const ok = await revertToMessage(sessionId, messageId);
    if (ok) {
      loadMessages(sessionId);
    }
  };

  // ── Permission / Question handlers ─────────────────────────────────────
  const handlePermissionDone = () => setPendingPermission(null);

  const handleQuestionReply = async (answers: string[][]) => {
    if (!pendingQuestion) return;
    await sendQuestionReply(
      pendingQuestion.sessionID,
      pendingQuestion.id,
      answers,
    );
    setPendingQuestion(null);
  };

  const handleQuestionReject = () => setPendingQuestion(null);

  // ── Slash commands / file mentions ──────────────────────────────────────
  const closeMenus = () => {
    setShowSlashMenu(false);
    setShowFileMenu(false);
    setSlashQuery("");
  };

  const executeSlashCommand = async (cmd: string) => {
    closeMenus();
    if (cmd === "/new") {
      // Handled by Chat via onOpenSession after creating
      try {
        const resp = await fetch(`${API_BASE}/sessions`, { method: "POST" });
        if (resp.ok) {
          const data = await resp.json();
          const newId = data.id || data.sessionID;
          if (newId) onOpenSession(newId);
        }
      } catch {
        // ignore
      }
      return;
    }
    if (cmd === "/clear") {
      msgStore.current.clear();
      setMessages([]);
      return;
    }
    if (cmd === "/compact") {
      try {
        await executeCommand(sessionId, "/compact");
      } catch {
        // ignore
      }
      return;
    }
    if (cmd === "/help") {
      // Show help as a system message
      const helpMsg = ensureMsg("help-" + Date.now(), "system", Date.now());
      helpMsg.order = ["help-text"];
      helpMsg.parts = new Map([
        [
          "help-text",
          {
            id: "help-text",
            kind: "text",
            text: `**Available commands:**\n- \`/new\` — Start a new chat session\n- \`/compact\` — Compact the current session\n- \`/clear\` — Clear messages (UI only)\n- \`/help\` — Show this help\n\n**@file mentions**\nType \`@\` followed by a filename to search and insert file references.`,
          },
        ],
      ]);
      rebuildMessages();
      return;
    }
    // Other commands: send to server
    try {
      await executeCommand(sessionId, cmd);
    } catch {
      // ignore
    }
  };

  const getFilteredSlashCommands = (): AutocompleteItem[] => {
    if (!slashQuery) return slashCommands;
    return slashCommands.filter((c) =>
      c.value.toLowerCase().includes(slashQuery.toLowerCase()),
    );
  };

  const insertFileMention = (file: AutocompleteItem) => {
    const ta = textareaRef.current;
    if (!ta) return;
    const val = input;
    const atIdx = val.lastIndexOf("@");
    const before = atIdx >= 0 ? val.slice(0, atIdx) : val;
    const after = val.slice(ta.selectionStart);
    const newVal = before.slice(0, atIdx) + `@${file.value} ` + after;
    setInput(newVal);
    closeMenus();
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
        setSlashIndex((i) => (i - 1 + slashCommands.length) % slashCommands.length);
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

  const handleInput = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setInput(val);

    const ta = e.target;
    const textBeforeCursor = val.slice(0, ta.selectionStart);

    // Check for @ file mention
    const atMatch = textBeforeCursor.match(/@(\S*)$/);
    if (atMatch) {
      const query = atMatch[1];
      setShowSlashMenu(false);
      if (query.length > 0) {
        // Fetch files
        if (fileFetchRef.current) clearTimeout(fileFetchRef.current);
        fileFetchRef.current = setTimeout(() => fetchFiles(query), 150);
      }
      return;
    }

    // Check for slash command
    if (textBeforeCursor.startsWith("/")) {
      const query = textBeforeCursor.slice(1);
      setShowFileMenu(false);
      setSlashQuery(query);
      setSlashIndex(0);
      setShowSlashMenu(true);
      return;
    }

    // Close menus if no trigger
    closeMenus();
  };

  const fetchFiles = async (query: string) => {
    try {
      const resp = await fetch(
        `${API_BASE}/files?q=${encodeURIComponent(query)}`,
      );
      if (resp.ok) {
        const data = await resp.json();
        setFileItems(data.files || []);
        setShowFileMenu(data.files?.length > 0);
        setFileIndex(0);
      }
    } catch {
      // ignore
    }
  };

  // ── Provider/model + agent seed from /settings defaults ────────────────
  // Track whether we've applied the saved defaults so a user change isn't
  // clobbered if providers re-loads. Only seeds once per pane.
  const seededDefaults = useRef(false);
  useEffect(() => {
    if (seededDefaults.current || providers.length === 0 || selectedModel) return;
    seededDefaults.current = true;

    const applyFallback = () => {
      const first = providers[0];
      if (first.models.length > 0) {
        setSelectedModel({ providerID: first.id, modelID: first.models[0] });
      }
    };

    configApi
      .getConfig()
      .then((cfg) => {
        // opencode.json stores default_agent as a flat string or {id: ...}.
        const agent =
          typeof cfg.defaultAgent === "string"
            ? cfg.defaultAgent
            : cfg.defaultAgent && typeof cfg.defaultAgent === "object"
              ? Object.keys(cfg.defaultAgent)[0] ?? ""
              : "";
        if (agent) setSelectedAgent(agent);

        // config.model is "providerID/modelID"; only apply if it exists in
        // the loaded providers, otherwise fall back to the first provider.
        const slash = cfg.model?.indexOf("/") ?? -1;
        if (cfg.model && slash > 0) {
          const providerID = cfg.model.slice(0, slash);
          const modelID = cfg.model.slice(slash + 1);
          const prov = providers.find((p) => p.id === providerID);
          if (prov?.models.includes(modelID)) {
            setSelectedModel({ providerID, modelID });
            return;
          }
        }
        applyFallback();
      })
      .catch(applyFallback);
  }, [providers, selectedModel]);

  // ── Context usage polling ──────────────────────────────────────────────
  useEffect(() => {
    if (!sessionId) return;
    const id = setInterval(() => loadContextUsage(sessionId), 30000);
    return () => clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  // ── Render ─────────────────────────────────────────────────────────────
  return (
    <>
      {/* Body: message list + right panel side by side */}
      <div className="chat-content-row">
        <div className="chat-body">
          <div className="chat-messages">
            {messages.length === 0 ? (
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
            ) : (
              <>
                <TodoDisplay sessionId={sessionId} refreshKey={todoRefreshKey} />
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
                          onEdit={() => handleEditMessage(msgText)}
                          onDelete={() => handleDeleteMessage(msg.id)}
                          onRevert={() => handleRevertMessage(msg.id)}
                        />
                      )}
                      <div className="message-avatar">
                        {msg.role === "user" ? (
                          <User size={20} />
                        ) : (
                          <Bot size={20} />
                        )}
                      </div>
                      <div className="message-body">
                        <div className="message-role">
                          {msg.role === "user" ? "You" : "Assistant"}
                        </div>
                        <div className="message-content">
                          {msg.parts.map((part) => {
                            if (part.kind === "reasoning")
                              return (
                                <ThinkingBlock
                                  key={part.id}
                                  text={part.text || ""}
                                />
                              );
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
                                  tool={part.tool || part.title || ""}
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
                              <Markdown key={part.id} content={part.text || ""} />
                            );
                          })}
                        </div>
                      </div>
                    </div>
                  );
                })}
                {isStreaming &&
                  messages[messages.length - 1]?.role !== "assistant" && (
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
              </>
            )}
          </div>

          {pendingPermission && (
            <div className="dialog-overlay">
              <PermissionDialog
                request={pendingPermission}
                onDone={handlePermissionDone}
              />
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
            {sendError && (
              <div className="chat-banner chat-banner-warn" role="alert">
                <div className="chat-banner-text">
                  <strong>Message not sent.</strong>
                  <span>{sendError}</span>
                </div>
                <button
                  className="btn-new-session"
                  onClick={() => setSendError(null)}
                >
                  Dismiss
                </button>
              </div>
            )}
            <div className="composer-card">
              <textarea
                ref={textareaRef}
                value={input}
                onChange={handleInput}
                onKeyDown={handleKeyDown}
                placeholder="Type your message…  (Enter to send, Shift+Enter for a new line)"
                rows={1}
              />
              <div className="composer-toolbar">
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
                  />
                )}
                {agents.length > 0 && (
                  <AgentSearchSelect
                    agents={agents}
                    value={selectedAgent}
                    onChange={setSelectedAgent}
                  />
                )}
                <div className="composer-actions">
                  <button
                    className="composer-icon"
                    data-tip="Templates"
                    aria-label="Templates"
                    onClick={onToggleTemplates}
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
                      disabled={!input.trim()}
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
                visible={showSlashMenu}
                selectedIndex={slashIndex}
                onSelect={handleSlashSelect}
                onClose={closeMenus}
                anchorEl={textareaRef.current}
              />
              <Autocomplete
                items={fileItems}
                visible={showFileMenu}
                selectedIndex={fileIndex}
                onSelect={handleFileSelect}
                onClose={closeMenus}
                anchorEl={textareaRef.current}
              />
            </div>
          </div>
        </div>

        {/* Right panel + resize sash. The sash sits between the chat body and
            the panel; it resizes the panel (next sibling), dragging left grows
            it (inverted delta). Only rendered when the panel is open. */}
        {rightPanelOpen && (
          <LayoutSash
            axis="x"
            resizeTarget="next"
            invertDelta
            min={240}
            onResizeEnd={(w) => setRightPanelWidth(w)}
          />
        )}
        <RightPanel
          open={rightPanelOpen}
          onClose={() => setRightPanelOpen(false)}
          tab={rightTab}
          onTabChange={setRightTab}
          activeSessionId={sessionId}
          workspaceDir={
            sessions.find((s) => s.id === sessionId)?.directory ||
            workspaceFilter
          }
          parent={parentSession}
          siblings={siblings}
          children={children}
          busySessions={busySessions}
          onSelectSession={(id) => onOpenSession(id)}
          onInsertFile={(f) => {
            setInput((prev) => `${prev}\`${f}\` `);
            textareaRef.current?.focus();
          }}
          width={rightPanelWidth}
        />
      </div>
    </>
  );
}


