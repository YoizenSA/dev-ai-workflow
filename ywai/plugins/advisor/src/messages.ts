/**
 * Flattening OpenCode messages into the shape the cursor works with. Separate
 * from the plugin module for the same reason as `config.ts`: only the plugin
 * itself may be exported from `index.ts`.
 */

import type { TrackedMessage } from "./delta"
import { hasFailedTool, hasMutatingTool, renderText, renderTools } from "./render"

/** Flattens an OpenCode message into the shape the cursor works with. */
export function toTracked(entry: any): TrackedMessage {
  const info = entry?.info ?? entry
  const parts = entry?.parts ?? []
  return {
    id: info?.id,
    role: info?.role,
    text: renderText(parts),
    tools: renderTools(parts),
    mutating: hasMutatingTool(parts),
    failed: hasFailedTool(parts),
  }
}

export async function fetchMessages(session: any, sessionID: string): Promise<TrackedMessage[]> {
  try {
    let raw: unknown
    if (typeof session?.context === "function") {
      raw = await session.context({ sessionID })
    } else if (typeof session?.messages === "function") {
      const res = await session.messages({ sessionID, path: { id: sessionID } })
      raw = res?.data ?? res
    } else {
      raw = []
    }
    return (Array.isArray(raw) ? raw : []).map(toTracked)
  } catch {
    return []
  }
}


export function extractText(parts: any[]): string {
  return (Array.isArray(parts) ? parts : [])
    .filter((p) => p?.type === "text" && typeof p.text === "string")
    .map((p) => p.text.trim())
    .filter(Boolean)
    .join("\n")
}
