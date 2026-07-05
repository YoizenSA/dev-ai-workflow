export interface Session {
  id: string;
  title?: string;
  time?: { created?: number; completed?: number };
  directory?: string;
  // opencode sessions carry a parentID when they are subagent forks. Present in
  // /api/chat/sessions and /api/chat/sessions/{id}/children responses (raw
  // passthrough from opencode). Undefined for root sessions.
  parentID?: string;
}

export type PartKind = "text" | "reasoning" | "tool" | "subtask";

export interface MsgPart {
  id: string;
  kind: PartKind;
  text?: string;
  toolName?: string;
  tool?: string;
  title?: string;
  status?: string;
  output?: string;
  messageID?: string;
  agent?: string;
  description?: string;
}

export interface Message {
  id: string;
  role: string;
  parts: MsgPart[];
  timestamp: number;
}

export interface StoredMessage {
  id: string;
  role: string;
  created: number;
  order: string[];
  parts: Map<string, MsgPart>;
}

export interface AutocompleteItem {
  label: string;
  description?: string;
  value: string;
}
