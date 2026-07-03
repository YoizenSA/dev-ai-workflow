import { describe, it, expect } from "vitest";
import { getEventStreamURL, getMessagesURL } from "./chat";

describe("chat API URL construction", () => {
  it("should use sessionID (capital D) in event stream URL", () => {
    const url = getEventStreamURL("session-abc");
    // The backend expects sessionID (capital D) — see chat_proxy.go handleChatSSE
    expect(url).toContain("sessionID=session-abc");
  });

  it("should use sessionID (capital D) in messages URL", () => {
    const url = getMessagesURL("session-abc");
    // The backend expects sessionID (capital D) — see chat_proxy.go handleChatMessages
    expect(url).toContain("sessionID=session-abc");
  });
});
