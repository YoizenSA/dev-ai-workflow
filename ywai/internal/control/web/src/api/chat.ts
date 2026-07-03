// ponytail: URL builders extracted from Chat.tsx for testability.
// Backend expects sessionID (capital D) — see chat_proxy.go

const API_BASE = "/api/chat";

export function getEventStreamURL(sessionId: string): string {
  return `${API_BASE}/events?sessionID=${sessionId}`;
}

export function getMessagesURL(sessionId: string): string {
  return `${API_BASE}/messages?sessionID=${sessionId}`;
}
