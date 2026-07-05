// ponytail: URL builders extracted from Chat.tsx for testability.
// Backend expects sessionID (capital D) — see chat_proxy.go

export const API_BASE = "/api/chat";

export function getEventStreamURL(sessionId: string): string {
  return `${API_BASE}/events?sessionID=${sessionId}`;
}

export function getMessagesURL(sessionId: string): string {
  return `${API_BASE}/sessions/${sessionId}`;
}

export function getContextURL(sessionId: string): string {
  return `${API_BASE}/sessions/${sessionId}/context`;
}

export function getAbortURL(): string {
  return `${API_BASE}/abort`;
}

export function getPermissionURL(sessionId: string, permissionId: string): string {
  return `${API_BASE}/sessions/${sessionId}/permissions/${permissionId}`;
}

export function getQuestionReplyURL(sessionId: string, requestId: string): string {
  return `${API_BASE}/sessions/${sessionId}/question/${requestId}/reply`;
}

// startOpencode asks the ywai backend to spawn `opencode serve` (it resolves
// the binary via agent.FindBinary, so nvm/asdf installs work). Mirrors
// missionsApi.startOpencode but lives here so the chat view can import it from
// the same module as its other endpoints.
export async function startOpencode(): Promise<{ status: string; message: string; pid?: number }> {
  const resp = await fetch("/missions/api/opencode/start", { method: "POST" });
  if (!resp.ok) {
    throw new Error(`Failed to start opencode (HTTP ${resp.status})`);
  }
  return resp.json();
}

// getSessionInfoURL returns the metadata endpoint for a single session.
export function getSessionInfoURL(sessionId: string): string {
  return `${API_BASE}/sessions/${sessionId}/info`;
}

// SessionInfo is the metadata returned by opencode's GET /session/{id}
// (passthrough via /api/chat/sessions/{id}/info). Only the fields the chat UI
// consumes are declared; opencode returns more.
export interface SessionInfo {
  id: string;
  title?: string;
  parentID?: string;
  directory?: string;
  time?: { created?: number; updated?: number; completed?: number };
}

// getSessionInfo fetches a single session's metadata (id, title, parentID,
// time). Used to resolve a subagent's parent for family navigation.
export async function getSessionInfo(sessionId: string): Promise<SessionInfo> {
  const resp = await fetch(getSessionInfoURL(sessionId));
  if (!resp.ok) {
    throw new Error(`Failed to load session info (HTTP ${resp.status})`);
  }
  return resp.json();
}

export function getQuestionRejectURL(sessionId: string, requestId: string): string {
  return `${API_BASE}/sessions/${sessionId}/question/${requestId}/reject`;
}

export function getTodoURL(sessionId: string): string {
  return `${API_BASE}/sessions/${sessionId}/todo`;
}

export function getDeleteMessageURL(sessionId: string, messageID: string): string {
  return `${API_BASE}/sessions/${sessionId}/message/${messageID}`;
}

export function getRevertURL(sessionId: string): string {
  return `${API_BASE}/sessions/${sessionId}/revert`;
}

// ─── API callers ───────────────────────────────────────────────────────────

interface ContextUsage {
  used: number;
  total: number;
  pct: number;
}

export async function fetchContextUsage(sessionId: string): Promise<ContextUsage | null> {
  try {
    const resp = await fetch(getContextURL(sessionId));
    if (!resp.ok) return null;
    return resp.json();
  } catch {
    return null;
  }
}

export async function abortSession(): Promise<void> {
  try {
    await fetch(getAbortURL(), { method: "POST" });
  } catch { /* ignore */ }
}

export async function renameSession(sessionId: string, title: string): Promise<boolean> {
  try {
    const resp = await fetch(`${API_BASE}/sessions/${sessionId}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ title }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

export async function deleteSession(sessionId: string): Promise<boolean> {
  try {
    const resp = await fetch(`${API_BASE}/sessions/${sessionId}`, {
      method: "DELETE",
    });
    return resp.ok;
  } catch {
    return false;
  }
}

export async function sendPermissionAction(
  sessionId: string,
  permissionId: string,
  action: "allow" | "deny",
  always: boolean,
): Promise<boolean> {
  try {
    const resp = await fetch(getPermissionURL(sessionId, permissionId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ action, always }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

export async function sendQuestionReply(
  sessionId: string,
  requestId: string,
  answers: string[][],
): Promise<boolean> {
  try {
    const resp = await fetch(getQuestionReplyURL(sessionId, requestId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ answers }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

export interface TodoItem {
  text: string;
  done: boolean;
}

export async function getTodo(sessionId: string): Promise<TodoItem[]> {
  try {
    const resp = await fetch(getTodoURL(sessionId));
    if (!resp.ok) return [];
    const data = await resp.json();
    return data.items || [];
  } catch {
    return [];
  }
}

export async function deleteMessage(
  sessionId: string,
  messageID: string,
): Promise<boolean> {
  try {
    const resp = await fetch(getDeleteMessageURL(sessionId, messageID), {
      method: "DELETE",
    });
    return resp.ok;
  } catch {
    return false;
  }
}

export async function revertToMessage(
  sessionId: string,
  messageID: string,
): Promise<boolean> {
  try {
    const resp = await fetch(getRevertURL(sessionId), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ messageID }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}

// ─── Slash commands ────────────────────────────────────────────────────────

export interface CommandItem {
  label: string;
  description: string;
  value: string;
}

const FALLBACK_COMMANDS: CommandItem[] = [
  { label: "/new", description: "Start a new chat session", value: "/new" },
  { label: "/compact", description: "Compact the current session", value: "/compact" },
  { label: "/clear", description: "Clear messages (UI only)", value: "/clear" },
  { label: "/help", description: "Show available commands", value: "/help" },
  { label: "/context", description: "Show session context", value: "/context" },
  { label: "/init", description: "Initialize a new project", value: "/init" },
  { label: "/doctor", description: "Run health checks", value: "/doctor" },
  { label: "/status", description: "Show session status", value: "/status" },
  { label: "/exit", description: "End the current session", value: "/exit" },
];

export async function getCommands(): Promise<CommandItem[]> {
  try {
    const resp = await fetch(`${API_BASE}/commands`);
    if (resp.ok) {
      const data = await resp.json();
      if (Array.isArray(data.commands)) return data.commands;
      if (Array.isArray(data)) return data;
    }
  } catch { /* ignore */ }
  return FALLBACK_COMMANDS;
}

export async function executeCommand(sessionId: string, cmd: string): Promise<boolean> {
  try {
    const resp = await fetch(`${API_BASE}/sessions/${sessionId}/command`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ command: cmd }),
    });
    return resp.ok;
  } catch {
    return false;
  }
}
