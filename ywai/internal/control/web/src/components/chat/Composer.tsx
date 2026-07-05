import { useState, useRef, useEffect, useCallback, forwardRef, useImperativeHandle } from "react";
import { Send, Square, Trash2, FileText } from "lucide-react";
import Autocomplete from "./Autocomplete";
import CommandSuggestions from "./CommandSuggestions";
import type { AutocompleteItem } from "./types";
import { getCommands } from "../../api/chat";

export interface ComposerHandle {
  setInput: (text: string) => void;
  focus: () => void;
}

interface ComposerProps {
  onSend: (text: string) => void;
  onSlashCommand: (cmd: string) => void;
  disabled?: boolean;
  isStreaming?: boolean;
  onStop?: () => void;
  providers: { id: string; name: string; models: string[] }[];
  selectedModel: { providerID: string; modelID: string } | null;
  onModelChange: (model: { providerID: string; modelID: string } | null) => void;
  agents: { name: string; description?: string; mode?: string }[];
  selectedAgent: string;
  onAgentChange: (agent: string) => void;
  templates: { id: string; name: string; content: string }[];
  onSaveTemplate: () => void;
  onDeleteTemplate: (id: string, e: React.MouseEvent) => void;
  editText?: string;
}

const Composer = forwardRef<ComposerHandle, ComposerProps>(function Composer({
  onSend,
  onSlashCommand,
  disabled,
  isStreaming,
  onStop,
  providers,
  selectedModel,
  onModelChange,
  agents,
  selectedAgent,
  onAgentChange,
  templates,
  onSaveTemplate,
  onDeleteTemplate,
  editText,
}, ref) {
  useImperativeHandle(ref, () => ({
    setInput: (text: string) => {
      setInput(text);
    },
    focus: () => {
      textareaRef.current?.focus();
    },
  }));
  const [input, setInput] = useState("");
  useEffect(() => {
    if (editText !== undefined) setInput(editText);
  }, [editText]);
  const [cursorPos, setCursorPos] = useState(0);
  const [showFileMenu, setShowFileMenu] = useState(false);
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [fileItems, setFileItems] = useState<AutocompleteItem[]>([]);
  const [fileIndex, setFileIndex] = useState(0);
  const [slashQuery, setSlashQuery] = useState("");
  const [slashIndex, setSlashIndex] = useState(0);
  const [commands, setCommands] = useState<AutocompleteItem[]>([]);
  const [showTemplates, setShowTemplates] = useState(false);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileFetchRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  // Load slash commands from server on mount
  useEffect(() => {
    getCommands().then((cmds) =>
      setCommands(cmds.map((c) => ({ label: c.label, description: c.description, value: c.value }))),
    );
  }, []);

  // Auto-grow textarea to fit content (capped by CSS max-height).
  useEffect(() => {
    const ta = textareaRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = `${Math.min(ta.scrollHeight, 200)}px`;
  }, [input]);

  const sendMessage = () => {
    if (!input.trim() || disabled) return;
    onSend(input);
    setInput("");
  };

  const closeMenus = useCallback(() => {
    setShowSlashMenu(false);
    setShowFileMenu(false);
    setSlashQuery("");
  }, []);

  const fetchFiles = useCallback(async (prefix: string) => {
    try {
      const resp = await fetch(`/api/files?q=${encodeURIComponent(prefix)}`);
      if (resp.ok) {
        const data = await resp.json();
        setFileItems(
          (data.files || []).map((f: string) => ({ label: f, value: f })),
        );
      }
    } catch {
      // ignore
    }
  }, []);

  const insertFileMention = useCallback(
    (file: AutocompleteItem) => {
      const before = input.slice(0, cursorPos);
      const atIdx = before.lastIndexOf("@");
      const after = input.slice(cursorPos);
      const mention = `\`${file.value}\` `;
      setInput(before.slice(0, atIdx) + mention + after);
      const newPos = (before.slice(0, atIdx) + mention).length;
      setCursorPos(newPos);
      closeMenus();
      textareaRef.current?.focus();
    },
    [input, cursorPos, closeMenus],
  );

  const executeSlashCommand = useCallback(
    (cmd: string) => {
      setInput("");
      closeMenus();
      onSlashCommand(cmd);
    },
    [closeMenus, onSlashCommand],
  );

  const handleFileSelect = useCallback(
    (item: AutocompleteItem) => {
      insertFileMention(item);
    },
    [insertFileMention],
  );

  const getFilteredSlashCommands = useCallback(() => {
    if (!slashQuery) return commands;
    return commands.filter((c) => c.label.includes(slashQuery));
  }, [commands, slashQuery]);

  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      const val = e.target.value;
      const pos = e.target.selectionStart || 0;
      setInput(val);
      setCursorPos(pos);

      // Check for slash command at start of line
      const beforeCursor = val.slice(0, pos);
      const lineStart = beforeCursor.lastIndexOf("\n") + 1;
      const currentLine = beforeCursor.slice(lineStart);

      // Check for @file mention (word boundary before @)
      const atIdx = beforeCursor.lastIndexOf("@");
      if (
        atIdx >= 0 &&
        atIdx >= lineStart &&
        !beforeCursor.slice(lineStart, atIdx).includes(" ")
      ) {
        const query = beforeCursor.slice(atIdx + 1);
        const charBefore = atIdx > 0 ? val[atIdx - 1] : " ";
        if (charBefore === " " || charBefore === "\n" || charBefore === "") {
          setShowSlashMenu(false);
          setShowFileMenu(true);
          setFileIndex(0);
          if (query.length === 0) {
            fetchFiles("");
          } else {
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
    },
    [closeMenus, fetchFiles],
  );

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (showSlashMenu) {
        if (e.key === "ArrowDown") {
          e.preventDefault();
          setSlashIndex((i) => Math.min(i + 1, getFilteredSlashCommands().length - 1));
          return;
        }
        if (e.key === "ArrowUp") {
          e.preventDefault();
          setSlashIndex((i) => Math.max(i - 1, 0));
          return;
        }
        if (e.key === "Enter" || e.key === "Tab") {
          e.preventDefault();
          const items = getFilteredSlashCommands();
          if (items[slashIndex]) {
            executeSlashCommand(items[slashIndex].value);
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

      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        sendMessage();
      }
    },
    [
      showSlashMenu,
      showFileMenu,
      getFilteredSlashCommands,
      slashIndex,
      fileItems,
      fileIndex,
      executeSlashCommand,
      closeMenus,
      insertFileMention,
      sendMessage,
    ],
  );

  const insertTemplate = (content: string) => {
    setInput((prev) => (prev ? `${prev}\n${content}` : content));
    setShowTemplates(false);
    textareaRef.current?.focus();
  };

  return (
    <div className="chat-composer">
      <div className="composer-card">
        <div className="composer-toolbar">
          {providers.length > 0 && (
            <select
              className="composer-model-select"
              value={
                selectedModel
                  ? `${selectedModel.providerID}::${selectedModel.modelID}`
                  : ""
              }
              onChange={(e) => {
                const [providerID, modelID] = e.target.value.split("::");
                onModelChange(
                  providerID && modelID ? { providerID, modelID } : null,
                );
              }}
            >
              {providers.map((p) =>
                p.models.map((m) => (
                  <option key={`${p.id}::${m}`} value={`${p.id}::${m}`}>
                    {p.name} · {m}
                  </option>
                )),
              )}
            </select>
          )}

          {agents.length > 0 && (
            <select
              className="composer-agent-select"
              value={selectedAgent}
              onChange={(e) => onAgentChange(e.target.value)}
            >
              <option value="">Default agent</option>
              {agents.map((a) => (
                <option key={a.name} value={a.name}>
                  {a.name}
                  {a.description ? ` — ${a.description}` : ""}
                </option>
              ))}
            </select>
          )}

          <button
            className="btn-templates"
            onClick={() => setShowTemplates((v) => !v)}
            data-tip="Templates"
            aria-label="Templates"
            type="button"
          >
            <FileText size={16} />
          </button>
        </div>

        <div className="composer-input-row">
          <textarea
            ref={textareaRef}
            className="composer-input"
            placeholder="Type a message…"
            rows={1}
            value={input}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            disabled={disabled}
          />
          <div className="composer-actions">
            {isStreaming ? (
            <button
              className="btn-stop"
              onClick={onStop}
              aria-label="Stop generation"
            >
              <Square size={20} />
            </button>
          ) : (
            <button
              className="btn-send"
              onClick={sendMessage}
              disabled={!input.trim() || disabled}
              aria-label="Send message"
            >
              <Send size={20} />
            </button>
          )}
          </div>
        </div>

        <CommandSuggestions
          visible={showSlashMenu}
          anchorEl={textareaRef.current}
          onClose={closeMenus}
          onCommand={executeSlashCommand}
        />
        <Autocomplete
          items={fileItems}
          selectedIndex={fileIndex}
          onSelect={handleFileSelect}
          onClose={closeMenus}
          visible={showFileMenu}
          anchorEl={textareaRef.current}
        />

        {showTemplates && (
          <div className="templates-menu">
            <div className="templates-menu-header">
              <span>Templates</span>
              <button
                className="btn-save-template"
                onClick={onSaveTemplate}
                data-tip="Save current message as template"
                aria-label="Save template"
              >
                + Save current
              </button>
            </div>
            {templates.map((t) => (
              <div
                key={t.id}
                className="template-item"
                onClick={() => insertTemplate(t.content)}
              >
                <span className="template-name">{t.name}</span>
                <button
                  className="btn-template-delete"
                  data-tip="Delete template"
                  aria-label={`Delete template ${t.name}`}
                  onClick={(e) => onDeleteTemplate(t.id, e)}
                >
                  <Trash2 size={16} />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
});

export default Composer;
