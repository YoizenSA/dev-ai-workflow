import { Bot, X } from "lucide-react";
import { useRef, useState, useEffect, useCallback } from "react";
import type { Session } from "./types";

interface TabStripProps {
  leafId: string;
  openSessions: string[];
  activeTab: string | null;
  sessions: Session[];
  onSelect: (id: string) => void;
  onClose: (id: string) => void;
  onMoveSession: (
    sessionId: string,
    fromLeafId: string,
    toLeafId: string,
    insertIndex?: number,
  ) => void;
  /** Context-menu actions. Optional so a minimal host still works. */
  onSplitRight?: (sessionId: string) => void;
  onSplitDown?: (sessionId: string) => void;
  /** Trailing actions rendered at the right end of the strip (e.g. subagents toggle). */
  trailing?: React.ReactNode;
}

/**
 * TabStrip — always renders (even with a single tab), VS Code style.
 *
 * Hosts a right-click context menu with Split Right / Split Down / Close /
 * Close Others when the split callbacks are provided.
 */
export default function TabStrip({
  leafId,
  openSessions,
  activeTab,
  sessions,
  onSelect,
  onClose,
  onMoveSession,
  onSplitRight,
  onSplitDown,
  trailing,
}: TabStripProps) {
  const stripRef = useRef<HTMLDivElement>(null);
  const [dropIndex, setDropIndex] = useState<number | null>(null);
  const [draggingId, setDraggingId] = useState<string | null>(null);

  // Context menu
  const [ctxMenu, setCtxMenu] = useState<{
    x: number;
    y: number;
    sessionId: string;
  } | null>(null);

  const sessionMap = new Map(sessions.map((s) => [s.id, s]));

  const closeMenu = useCallback(() => setCtxMenu(null), []);

  // Close the context menu on any outside click / escape / scroll.
  useEffect(() => {
    if (!ctxMenu) return;
    const onDoc = () => closeMenu();
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") closeMenu();
    };
    // Defer one tick so the opening contextmenu event doesn't close it.
    const id = setTimeout(() => {
      document.addEventListener("click", onDoc);
      document.addEventListener("contextmenu", onDoc);
    }, 0);
    document.addEventListener("keydown", onKey);
    return () => {
      clearTimeout(id);
      document.removeEventListener("click", onDoc);
      document.removeEventListener("contextmenu", onDoc);
      document.removeEventListener("keydown", onKey);
    };
  }, [ctxMenu, closeMenu]);

  const handleDragStart = (e: React.DragEvent, sessionId: string) => {
    e.dataTransfer.setData("application/x-session-id", sessionId);
    e.dataTransfer.setData("application/x-leaf-id", leafId);
    e.dataTransfer.effectAllowed = "move";
    setDraggingId(sessionId);
    if (e.dataTransfer.setDragImage) {
      const el = e.currentTarget as HTMLElement;
      e.dataTransfer.setDragImage(el, el.offsetWidth / 2, el.offsetHeight / 2);
    }
  };

  const handleDragEnd = () => {
    setDraggingId(null);
    setDropIndex(null);
  };

  const getDropIndex = (e: React.DragEvent): number => {
    const strip = stripRef.current;
    if (!strip) return openSessions.length;
    const tabs = [...strip.querySelectorAll<HTMLElement>(".tab-item")];
    for (let i = 0; i < tabs.length; i++) {
      const rect = tabs[i].getBoundingClientRect();
      if (e.clientX < rect.left + rect.width / 2) return i;
    }
    return tabs.length;
  };

  const handleDragOver = (e: React.DragEvent) => {
    if (!e.dataTransfer.types.includes("application/x-session-id")) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    setDropIndex(getDropIndex(e));
  };

  const handleDragLeave = (e: React.DragEvent) => {
    if (!stripRef.current?.contains(e.relatedTarget as Node)) {
      setDropIndex(null);
    }
  };

  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setDropIndex(null);
    const sessionId = e.dataTransfer.getData("application/x-session-id");
    const fromLeafId = e.dataTransfer.getData("application/x-leaf-id") || leafId;
    if (!sessionId) return;
    const idx = getDropIndex(e);
    onMoveSession(sessionId, fromLeafId, leafId, idx);
  };

  const handleContextMenu = (e: React.MouseEvent, sessionId: string) => {
    e.preventDefault();
    e.stopPropagation();
    setCtxMenu({ x: e.clientX, y: e.clientY, sessionId });
  };

  const closeOthers = (keepId: string) => {
    for (const id of openSessions) {
      if (id !== keepId) onClose(id);
    }
  };

  return (
    <>
      <div
        ref={stripRef}
        className={`tab-strip${dropIndex !== null ? " tab-strip-drop-active" : ""}`}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        onDrop={handleDrop}
      >
        {openSessions.map((id, i) => {
          const s = sessionMap.get(id);
          const isSubagent = !!s?.parentID;
          const title = s?.title || `Session ${id.slice(0, 8)}`;
          return (
            <div
              key={id}
              className={`tab-item${id === activeTab ? " tab-active" : ""}${dropIndex === i ? " tab-drop-before" : ""}`}
              draggable
              data-dragging={id === draggingId ? "true" : undefined}
              onClick={() => onSelect(id)}
              onContextMenu={(e) => handleContextMenu(e, id)}
              onMouseDown={(e) => {
                if (e.button === 1) {
                  e.preventDefault();
                  onClose(id);
                }
              }}
              onDragStart={(e) => handleDragStart(e, id)}
              onDragEnd={handleDragEnd}
            >
              {isSubagent && <Bot size={12} className="tab-subagent-icon" />}
              <span className="tab-title">{title}</span>
              <button
                className="tab-close"
                onClick={(e) => {
                  e.stopPropagation();
                  onClose(id);
                }}
                aria-label={`Close ${title}`}
              >
                <X size={12} />
              </button>
            </div>
          );
        })}
        {dropIndex === openSessions.length && (
          <div className="tab-drop-indicator-end" />
        )}
        {trailing && <div className="tab-strip-trailing">{trailing}</div>}
      </div>

      {ctxMenu && (
        <div
          className="tab-context-menu"
          style={{ left: ctxMenu.x, top: ctxMenu.y }}
          onClick={(e) => e.stopPropagation()}
          onContextMenu={(e) => e.preventDefault()}
        >
          {onSplitRight && (
            <button
              className="tab-context-item"
              onClick={() => {
                onSplitRight(ctxMenu.sessionId);
                closeMenu();
              }}
            >
              Split Right
            </button>
          )}
          {onSplitDown && (
            <button
              className="tab-context-item"
              onClick={() => {
                onSplitDown(ctxMenu.sessionId);
                closeMenu();
              }}
            >
              Split Down
            </button>
          )}
          <div className="tab-context-sep" />
          <button
            className="tab-context-item"
            onClick={() => {
              onClose(ctxMenu.sessionId);
              closeMenu();
            }}
          >
            Close
          </button>
          {openSessions.length > 1 && (
            <button
              className="tab-context-item"
              onClick={() => {
                closeOthers(ctxMenu.sessionId);
                closeMenu();
              }}
            >
              Close Others
            </button>
          )}
        </div>
      )}
    </>
  );
}
