import { useState, useEffect, useRef, useCallback } from "react";
import "./Chat.css";

interface Message {
  id: string;
  role: "user" | "assistant";
  content: string;
  timestamp: number;
}

interface Session {
  id: string;
  title?: string;
  created: number;
}

const API_BASE = "/api/chat";

export default function Chat() {
  const [sessions, setSessions] = useState<Session[]>([]);
  const [activeSession, setActiveSession] = useState<string | null>(null);
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isStreaming, setIsStreaming] = useState(false);
  const [connected, setConnected] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Auto-scroll to bottom
  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  // Load sessions on mount
  useEffect(() => {
    loadSessions();
  }, []);

  // Connect SSE when session changes
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
        if (data.sessions?.length > 0 && !activeSession) {
          setActiveSession(data.sessions[0].id);
        }
      }
    } catch {
      setConnected(false);
    }
  };

  const loadMessages = async (sessionId: string) => {
    try {
      const resp = await fetch(`${API_BASE}/messages?sessionID=${sessionId}`);
      if (resp.ok) {
        const data = await resp.json();
        setMessages(
          (data.messages || []).map((m: any) => ({
            id: m.id,
            role: m.role,
            content: extractTextContent(m),
            timestamp: m.time?.created || Date.now(),
          }))
        );
      }
    } catch {
      // ignore
    }
  };

  const extractTextContent = (message: any): string => {
    if (typeof message.content === "string") return message.content;
    if (Array.isArray(message.content)) {
      return message.content
        .filter((p: any) => p.type === "text")
        .map((p: any) => p.text)
        .join("");
    }
    return "";
  };

  const connectSSE = (sessionId: string) => {
    disconnectSSE();
    const url = `${API_BASE}/events?sessionID=${sessionId}`;
    const es = new EventSource(url);
    eventSourceRef.current = es;

    es.onopen = () => setConnected(true);

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
        if (activeSession === sessionId) {
          connectSSE(sessionId);
        }
      }, 3000);
    };
  };

  const disconnectSSE = () => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
    }
  };

  const handleSSEEvent = (data: any) => {
    if (data.type === "message.part" || data.type === "message") {
      const part = data.part || data.message;
      if (!part) return;

      const text = part.text || part.content || "";
      if (!text) return;

      setMessages((prev) => {
        const existing = prev.find((m) => m.id === part.messageID);
        if (existing) {
          // Update existing message (streaming)
          return prev.map((m) =>
            m.id === part.messageID
              ? { ...m, content: m.content + text }
              : m
          );
        }
        // New assistant message
        if (part.type === "text" && data.role === "assistant") {
          return [
            ...prev,
            {
              id: part.messageID || `msg-${Date.now()}`,
              role: "assistant",
              content: text,
              timestamp: Date.now(),
            },
          ];
        }
        return prev;
      });
    }

    if (data.type === "message.complete") {
      setIsStreaming(false);
    }

    if (data.type === "session.status") {
      setIsStreaming(data.status === "running" || data.status === "streaming");
    }
  };

  const sendMessage = async () => {
    if (!input.trim() || !activeSession || isStreaming) return;

    const userMessage: Message = {
      id: `user-${Date.now()}`,
      role: "user",
      content: input.trim(),
      timestamp: Date.now(),
    };

    setMessages((prev) => [...prev, userMessage]);
    setInput("");
    setIsStreaming(true);

    try {
      await fetch(`${API_BASE}/send`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sessionID: activeSession,
          content: userMessage.content,
        }),
      });
    } catch {
      setIsStreaming(false);
    }
  };

  const createSession = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      });
      if (resp.ok) {
        const data = await resp.json();
        const newSession = data.session || data;
        setSessions((prev) => [newSession, ...prev]);
        setActiveSession(newSession.id);
        setMessages([]);
      }
    } catch {
      // ignore
    }
  };

  const abortSession = async () => {
    if (!activeSession) return;
    try {
      await fetch(`${API_BASE}/abort`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ sessionID: activeSession }),
      });
      setIsStreaming(false);
    } catch {
      // ignore
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  return (
    <div className="chat-container">
      {/* Session sidebar */}
      <div className="chat-sessions">
        <div className="chat-sessions-header">
          <h3>Sessions</h3>
          <button onClick={createSession} className="btn-new-session" title="New session">
            +
          </button>
        </div>
        <div className="chat-sessions-list">
          {sessions.map((s) => (
            <button
              key={s.id}
              className={`chat-session-item ${activeSession === s.id ? "active" : ""}`}
              onClick={() => setActiveSession(s.id)}
            >
              <span className="session-title">
                {s.title || `Session ${s.id.slice(0, 8)}`}
              </span>
            </button>
          ))}
          {sessions.length === 0 && (
            <div className="chat-empty">No sessions</div>
          )}
        </div>
      </div>

      {/* Chat area */}
      <div className="chat-main">
        <div className="chat-header">
          <div className="chat-status">
            <span className={`status-dot ${connected ? "connected" : "disconnected"}`} />
            <span>{connected ? "Connected" : "Disconnected"}</span>
          </div>
          {isStreaming && (
            <button onClick={abortSession} className="btn-abort">
              Stop
            </button>
          )}
        </div>

        <div className="chat-messages">
          {messages.map((msg) => (
            <div key={msg.id} className={`chat-message ${msg.role}`}>
              <div className="message-role">{msg.role === "user" ? "You" : "AI"}</div>
              <div className="message-content">
                <pre>{msg.content}</pre>
              </div>
            </div>
          ))}
          {isStreaming && (
            <div className="chat-message assistant streaming">
              <div className="message-role">AI</div>
              <div className="message-content">
                <span className="typing-indicator">...</span>
              </div>
            </div>
          )}
          <div ref={messagesEndRef} />
        </div>

        <div className="chat-input-area">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => setInput(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message... (Enter to send, Shift+Enter for newline)"
            rows={2}
            disabled={!activeSession}
          />
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
