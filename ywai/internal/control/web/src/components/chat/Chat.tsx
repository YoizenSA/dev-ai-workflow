import { useState, useEffect, useRef, useCallback } from "react";
import Autocomplete, { type AutocompleteItem } from "./Autocomplete";
import "./Chat.css";
import { getEventStreamURL, getMessagesURL } from "../../api/chat";

interface Session {
  id: string;
  title?: string;
  time?: { created?: number };
}

interface Message {
  id: string;
  role: string;
  content: string;
  timestamp: number;
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
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Accumulates messages by id and their text parts (by partID) so streaming
  // deltas and full-part updates from OpenCode reconstruct in order.
  type StoredMessage = {
    id: string;
    role: string;
    created: number;
    order: string[];
    parts: Map<string, string>;
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

  const setPart = (msgID: string, partID: string, text: string) => {
    const m = ensureMsg(msgID);
    if (!m.parts.has(partID)) m.order.push(partID);
    m.parts.set(partID, text);
  };

  const appendPart = (msgID: string, partID: string, delta: string) => {
    const m = ensureMsg(msgID);
    if (!m.parts.has(partID)) {
      m.order.push(partID);
      m.parts.set(partID, "");
    }
    m.parts.set(partID, (m.parts.get(partID) || "") + delta);
  };

  const rebuildMessages = () => {
    const arr = Array.from(msgStore.current.values())
      .sort((a, b) => a.created - b.created)
      .map((m) => ({
        id: m.id,
        role: m.role,
        content: m.order.map((p) => m.parts.get(p) || "").join(""),
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

  useEffect(() => {
    loadSessions();
  }, []);

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
      case "message.part.updated":
        if (p.part?.type === "text") {
          setPart(p.part.messageID, p.part.id, p.part.text || "");
          rebuildMessages();
        }
        break;
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
        body: JSON.stringify({ content: msgText }),
      });
    } catch {
      setError("Send failed — network error");
      setIsStreaming(false);
    }
  };

  const createSession = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`, {
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
          e.order = ["_full"];
          e.parts = new Map([["_full", m.content || ""]]);
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
          content: HELP_TEXT,
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

  return (
    <div className="chat-container">
      {/* Session sidebar */}
      <div className="chat-sessions">
        <div className="chat-sessions-header">
          <span>Sessions</span>
          <button className="btn-new-session" onClick={createSession}>
            + New
          </button>
        </div>
        <div className="chat-sessions-list">
          {sessions.map((s) => (
            <button
              key={s.id}
              className={`session-item ${s.id === activeSession ? "active" : ""}`}
              onClick={() => {
                setActiveSession(s.id);
                setMessages([]);
                loadMessages(s.id);
              }}
            >
              <span className="session-title">
                {s.title || `Session ${s.id.slice(0, 8)}`}
              </span>
            </button>
          ))}
          {sessions.length === 0 && (
            <div className="chat-empty">No sessions yet</div>
          )}
        </div>
      </div>

      {/* Main chat area */}
      <div className="chat-main">
        <div className="chat-header">
          <span className="chat-connection-status">
            {connected ? "● Connected" : "○ Disconnected"}
          </span>
        </div>

        <div className="chat-messages">
          {messages.map((msg) => (
            <div
              key={msg.id}
              className={`chat-message ${msg.role} ${isStreaming && messages[messages.length - 1]?.id === msg.id && msg.role === "assistant" ? "streaming" : ""}`}
            >
              <div className="message-role">
                {msg.role === "user" ? "You" : "AI"}
              </div>
              <div className="message-content">{msg.content}</div>
            </div>
          ))}
          {isStreaming && (
            <div className="chat-message assistant streaming">
              <div className="message-role">AI</div>
              <div className="message-content">
                <span className="streaming-cursor">▊</span>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        {error && <div className="chat-error">{error}</div>}

        <div className="chat-input-area">
          <div className="input-wrapper">
            <textarea
              ref={textareaRef}
              value={input}
              onChange={handleInputChange}
              onKeyDown={handleKeyDown}
              placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
              rows={2}
              disabled={!activeSession}
            />
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
          </div>
          <button
            onClick={sendMessage}
            disabled={!input.trim() || !activeSession || isStreaming}
            className="btn-send"
          >
            Send
          </button>
        </div>
      </div>
    </div>
  );
}
