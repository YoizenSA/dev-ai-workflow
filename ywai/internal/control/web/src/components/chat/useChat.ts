import { useState, useEffect, useRef, useCallback } from "react";
import type { Session, Message, MsgPart, StoredMessage, PartKind } from "./types";
import { getEventStreamURL, getMessagesURL } from "../../api/chat";

const API_BASE = "/api/chat";

export interface UseChatReturn {
  sessions: Session[];
  activeSession: string | null;
  setActiveSession: (id: string | null) => void;
  messages: Message[];
  isStreaming: boolean;
  connected: boolean;
  error: string;
  setError: (err: string) => void;
  retryLastMessage: () => void;
  providers: { id: string; name: string; models: string[] }[];
  selectedModel: { providerID: string; modelID: string } | null;
  setSelectedModel: (m: { providerID: string; modelID: string } | null) => void;
  agents: { name: string; description: string; mode: string }[];
  selectedAgent: string;
  setSelectedAgent: (a: string) => void;
  projects: { id: string; path: string; name: string }[];
  workspaceFilter: string;
  setWorkspaceFilter: (f: string) => void;
  children: Session[];
  questions: any[];
  permissions: any[];
  setPermissions: (p: any[] | ((prev: any[]) => any[])) => void;
  pinned: string[];
  templates: { id: string; name: string; content: string }[];
  createSession: () => Promise<void>;
  loadSessions: () => Promise<void>;
  togglePin: (id: string, e: React.MouseEvent) => Promise<void>;
  saveTemplate: () => Promise<void>;
  deleteTemplate: (id: string, e: React.MouseEvent) => Promise<void>;
  handleCompact: () => Promise<void>;
  handleClear: () => void;
  sendMessage: (text: string) => Promise<void>;
  replyQuestion: (requestID: string, answers: string[][]) => Promise<void>;
  rejectQuestion: (requestID: string) => Promise<void>;
  handleSlashCommand: (cmd: string) => void;
}

export function useChat(): UseChatReturn {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [connected, setConnected] = useState(false);
  const [error, setError] = useState("");
  const [providers, setProviders] = useState<
    { id: string; name: string; models: string[] }[]
  >([]);
  const [selectedModel, setSelectedModel] = useState<{
    providerID: string;
    modelID: string;
  } | null>(null);
  const [agents, setAgents] = useState<
    { name: string; description: string; mode: string }[]
  >([]);
  const [selectedAgent, setSelectedAgent] = useState<string>("");
  const [projects, setProjects] = useState<
    { id: string; path: string; name: string }[]
  >([]);
  const [workspaceFilter, setWorkspaceFilter] = useState<string>("");
  const [children, setChildren] = useState<Session[]>([]);
  const [questions, setQuestions] = useState<any[]>([]);
  const [permissions, setPermissions] = useState<any[]>([]);
  const [pinned, setPinned] = useState<string[]>([]);
  const [templates, setTemplates] = useState<
    { id: string; name: string; content: string }[]
  >([]);

  const eventSourceRef = useRef<EventSource | null>(null);
  const msgStore = useRef<Map<string, StoredMessage>>(new Map());

  // ---- Helpers ----

  const ensureMsg = useCallback(
    (id: string, role?: string, created?: number): StoredMessage => {
      let m = msgStore.current.get(id);
      if (!m) {
        m = { id, role: role || "assistant", created: created || Date.now(), order: [], parts: new Map() };
        msgStore.current.set(id, m);
      }
      if (role) m.role = role;
      if (created) m.created = created;
      return m;
    },
    [],
  );

  const ensurePart = useCallback(
    (msg: StoredMessage, partID: string, kind: PartKind): MsgPart => {
      let p = msg.parts.get(partID);
      if (!p) {
        p = { id: partID, kind };
        msg.parts.set(partID, p);
        msg.order.push(partID);
      }
      return p;
    },
    [],
  );

  const setPart = useCallback(
    (msgID: string, partID: string, fields: Partial<MsgPart> & { kind: PartKind }) => {
      const p = ensurePart(ensureMsg(msgID), partID, fields.kind);
      Object.assign(p, fields);
    },
    [ensureMsg, ensurePart],
  );

  const appendPart = useCallback(
    (msgID: string, partID: string, delta: string) => {
      const p = ensurePart(ensureMsg(msgID), partID, "text");
      p.text = (p.text || "") + delta;
    },
    [ensureMsg, ensurePart],
  );

  const rebuildMessages = useCallback(() => {
    const arr: Message[] = [];
    msgStore.current.forEach((m) => {
      arr.push({
        id: m.id,
        role: m.role,
        parts: m.order.map((id) => m.parts.get(id)!).filter(Boolean),
        timestamp: m.created,
      });
    });
    setMessages(arr);
  }, []);

  const disconnectSSE = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  }, []);

  // ---- SSE ----

  const handleSSEEvent = useCallback(
    (data: any) => {
      const p = data.payload || {};
      switch (data.type || data.event) {
        case "connected":
          setConnected(true);
          break;
        case "disconnected":
          setConnected(false);
          break;
        case "session.status":
          setIsStreaming(p.status?.type === "busy");
          break;
        case "session.idle":
          setIsStreaming(false);
          break;
        case "message.created":
        case "message.part.created":
          ensureMsg(p.info?.id || p.id, p.info?.role, p.info?.time?.created);
          rebuildMessages();
          break;
        case "message.part.updated": {
          const part = p.part;
          if (!part) break;
          if (part.type === "text") {
            setPart(part.messageID, part.id, { kind: "text", text: part.text || "" });
          } else if (part.type === "reasoning") {
            setPart(part.messageID, part.id, { kind: "reasoning", text: part.text || "" });
          } else if (part.type === "tool") {
            const st = part.state || {};
            const out = st.output
              ? typeof st.output === "string" ? st.output : JSON.stringify(st.output, null, 2)
              : "";
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
        case "permission.updated":
          setPermissions((prev) => {
            const exists = prev.find((pr: any) => pr.id === p.id);
            if (exists) return prev.map((pr: any) => pr.id === p.id ? { ...pr, ...p } : pr);
            return [...prev, { id: p.id, sessionID: p.sessionID || p.session_id || activeSession, tool: p.tool, description: p.description || "" }];
          });
          break;
        case "question":
          setQuestions((prev) => {
            const exists = prev.find((q: any) => q.id === p.id || q.id === p.requestID);
            if (exists) return prev;
            return [...prev, { id: p.id || p.requestID, sessionID: p.sessionID || activeSession, questions: p.questions || [{ label: p.question || p.text || "", multiple: false }] }];
          });
          break;
        case "error":
          setError(p.message || "An error occurred");
          setIsStreaming(false);
          break;
      }
    },
    [setPart, ensureMsg, appendPart, rebuildMessages],
  );

  const connectSSE = useCallback(
    (sessionId: string) => {
      disconnectSSE();
      const es = new EventSource(getEventStreamURL(sessionId));
      eventSourceRef.current = es;
      es.onmessage = (event) => {
        try { handleSSEEvent(JSON.parse(event.data)); }
        catch { /* ignore parse errors */ }
      };
      es.onerror = () => setConnected(false);
    },
    [disconnectSSE, handleSSEEvent],
  );

  // ---- API ----

  const loadSessions = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`);
      if (resp.ok) setSessions((await resp.json()).sessions || []);
    } catch { /* ignore */ }
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
      const def = data.default || {};
      const firstProviderID = Object.keys(def)[0];
      if (firstProviderID && def[firstProviderID]) {
        setSelectedModel({ providerID: firstProviderID, modelID: def[firstProviderID] });
      }
    } catch { /* ignore */ }
  };

  const loadAgents = async () => {
    try {
      const resp = await fetch(`${API_BASE}/agents`);
      if (resp.ok) setAgents((await resp.json()).agents || []);
    } catch { /* ignore */ }
  };

  const loadProjects = async () => {
    try {
      const resp = await fetch(`${API_BASE}/projects`);
      if (resp.ok) setProjects((await resp.json()).projects || []);
    } catch { /* ignore */ }
  };

  const loadPins = async () => {
    try {
      const resp = await fetch(`${API_BASE}/pins`);
      if (resp.ok) setPinned((await resp.json()).pins || []);
    } catch { /* ignore */ }
  };

  const loadChildren = async (sessionId: string) => {
    try {
      const resp = await fetch(`${API_BASE}/sessions/${sessionId}/children`);
      if (resp.ok) setChildren((await resp.json()).children || []);
    } catch { /* ignore */ }
  };

  const loadQuestions = async (sessionId: string) => {
    try {
      const resp = await fetch(`${API_BASE}/sessions/${sessionId}/questions`);
      if (resp.ok) setQuestions((await resp.json()).questions || []);
    } catch { /* ignore */ }
  };

  const loadTemplates = async () => {
    try {
      const resp = await fetch(`${API_BASE}/templates`);
      if (resp.ok) setTemplates((await resp.json()).templates || []);
    } catch { /* ignore */ }
  };

  const loadMessages = async (sessionId: string) => {
    try {
      const resp = await fetch(getMessagesURL(sessionId));
      if (resp.ok) {
        const data = await resp.json();
        msgStore.current = new Map();
        (data.messages || []).forEach((m: any, i: number) => {
          ensureMsg(m.id, m.role, m.created_at || i + 1);
          (m.parts || []).forEach((p: any) => {
            const part: MsgPart = {
              id: p.id, kind: p.type || "text", text: p.text || "",
              tool: p.tool, status: p.state?.status, output: p.state?.output,
            };
            setPart(m.id, p.id, part);
          });
        });
        rebuildMessages();
      }
    } catch { /* ignore */ }
  };

  // ---- Actions ----

  const createSession = async () => {
    try {
      const qs = workspaceFilter ? `?directory=${encodeURIComponent(workspaceFilter)}` : "";
      const resp = await fetch(`${API_BASE}/sessions${qs}`, { method: "POST" });
      if (resp.ok) {
        const data = await resp.json();
        setActiveSession(data.id);
        setMessages([]);
        loadSessions();
      }
    } catch { /* ignore */ }
  };

  const togglePin = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const isPinned = pinned.includes(id);
    try {
      const resp = await fetch(`${API_BASE}/pins/${id}`, {
        method: isPinned ? "DELETE" : "POST",
      });
      if (resp.ok) setPinned((await resp.json()).pins || []);
    } catch { /* ignore */ }
  };

  const saveTemplate = async () => {
    const content = prompt("Template content:");
    if (!content) return;
    const name = prompt("Template name:");
    if (!name) return;
    try {
      await fetch(`${API_BASE}/templates`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, content }),
      });
      loadTemplates();
    } catch { /* ignore */ }
  };

  const deleteTemplate = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await fetch(`${API_BASE}/templates/${id}`, { method: "DELETE" });
      loadTemplates();
    } catch { /* ignore */ }
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
    } catch { /* ignore */ }
  };

  const handleClear = () => setMessages([]);

  const lastSentText = useRef("");

  const sendMessage = async (text: string) => {
    if (!activeSession || isStreaming) return;
    lastSentText.current = text;
    setError("");
    setIsStreaming(true);
    try {
      await fetch(`${API_BASE}/sessions/${activeSession}/messages`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: text, model: selectedModel || undefined, agent: selectedAgent || undefined }),
      });
    } catch { /* ignore */ }
  };

  const retryLastMessage = () => {
    if (lastSentText.current) {
      sendMessage(lastSentText.current);
    }
  };

  const replyQuestion = async (requestID: string, answers: string[][]) => {
    try {
      await fetch(`${API_BASE}/questions/${requestID}/reply`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ answers }),
      });
    } catch { /* ignore */ }
    setQuestions((prev) => prev.filter((q) => q.id !== requestID));
    if (activeSession) loadQuestions(activeSession);
  };

  const rejectQuestion = async (requestID: string) => {
    try {
      await fetch(`${API_BASE}/questions/${requestID}/reject`, { method: "POST" });
    } catch { /* ignore */ }
    setQuestions((prev) => prev.filter((q) => q.id !== requestID));
    if (activeSession) loadQuestions(activeSession);
  };

  const handleSlashCommand = useCallback(
    (cmd: string) => {
      switch (cmd) {
        case "/new": createSession(); break;
        case "/compact": handleCompact(); break;
        case "/clear": handleClear(); break;
        case "/help": {
          setMessages((prev) => [...prev, {
            id: `help-${Date.now()}`,
            role: "assistant",
            parts: [{ id: "help", kind: "text" as const, text: HELP_TEXT }],
            timestamp: Date.now(),
          }]);
          break;
        }
      }
    },
    // ponytail: dependencies omitted — no stale closure risk
    [],
  );

  // ---- Effects ----

  useEffect(() => {
    loadSessions();
    loadProviders();
    loadAgents();
    loadProjects();
    loadPins();
    loadTemplates();
  }, []);

  useEffect(() => {
    if (activeSession) {
      connectSSE(activeSession);
      loadMessages(activeSession);
    }
    return () => disconnectSSE();
    // ponytail: reconnect only when session changes
  }, [activeSession, connectSSE, disconnectSSE]);

  useEffect(() => {
    if (activeSession) {
      loadChildren(activeSession);
      loadQuestions(activeSession);
    } else {
      setChildren([]);
      setQuestions([]);
    }
  }, [activeSession, isStreaming]);

  return {
    sessions, activeSession, setActiveSession, messages,
    isStreaming, connected, error, setError,
    providers, selectedModel, setSelectedModel,
    agents, selectedAgent, setSelectedAgent,
    projects, workspaceFilter, setWorkspaceFilter,
    children, questions, permissions, setPermissions, pinned, templates,
    createSession, loadSessions, togglePin,
    saveTemplate, deleteTemplate, handleCompact, handleClear,
    sendMessage, retryLastMessage, replyQuestion, rejectQuestion, handleSlashCommand,
  };
}

const HELP_TEXT = `**Available commands:**
- \`/new\` — Start a new chat session
- \`/compact\` — Compact the current session (sends to OpenCode)
- \`/clear\` — Clear messages (UI only)
- \`/help\` — Show this help

**@file mentions**
Type \`@\` followed by a filename to search and insert file references.`;
