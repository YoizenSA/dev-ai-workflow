import { useRef, useEffect } from "react";
import { User, Bot, Sparkles, Plus } from "lucide-react";
import Markdown from "./Markdown";
import { ThinkingBlock, ToolBlock, SubagentBlock } from "./PartBlock";
import type { Message } from "./types";

const SUGGESTIONS = [
  "Explain what this project does",
  "Help me write a message",
  "Summarize a text I paste",
  "Give me ideas to get started",
];

interface MessageListProps {
  messages: Message[];
  isStreaming: boolean;
  activeSession: string | null;
  showWelcome: boolean;
  onSuggestionClick: (text: string) => void;
  onCreateSession: () => void;
}

export default function MessageList({
  messages,
  isStreaming,
  activeSession,
  showWelcome,
  onSuggestionClick,
  onCreateSession,
}: MessageListProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  return (
    <div className="chat-messages">
      {!activeSession && (
        <div className="chat-placeholder">
          <Sparkles size={24} className="chat-placeholder-icon" />
          <h2>How can I help you today?</h2>
          <p>Pick a conversation or create a new one to get started.</p>
          <button className="btn-new-session big" onClick={onCreateSession}>
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
                className="chat-suggestion"
                onClick={() => onSuggestionClick(s)}
              >
                {s}
              </button>
            ))}
          </div>
        </div>
      )}

      {messages.map((msg, i) => {
        const prev = messages[i - 1];
        const startsNewTurn = !prev || prev.role !== msg.role;
        return (
          <div
            key={msg.id}
            className={`chat-message ${msg.role} ${startsNewTurn ? "turn-start" : ""}`}
          >
            <div className="message-avatar">
              {msg.role === "user" ? <User size={20} /> : <Bot size={20} />}
            </div>
            <div className="message-body">
              <div className="message-role">
                {msg.role === "user" ? "You" : "Assistant"}
              </div>
              <div className="message-content">
                {msg.parts.map((part) => {
                  if (part.kind === "text") {
                    return <Markdown key={part.id} content={part.text || ""} />;
                  }
                  if (part.kind === "reasoning") {
                    return <ThinkingBlock key={part.id} text={part.text || ""} />;
                  }
                  if (part.kind === "tool") {
                    return (
                      <ToolBlock
                        key={part.id}
                        tool={part.tool || part.toolName}
                        status={part.status}
                        title={part.title}
                        output={part.output}
                      />
                    );
                  }
                  if (part.kind === "subtask") {
                    return (
                      <SubagentBlock
                        key={part.id}
                        agent={part.agent}
                        description={part.description}
                      />
                    );
                  }
                  return null;
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
  );
}
