import { useState, useEffect, useRef, useCallback } from "react";
import { useSearchParams } from "react-router-dom";
import {
  Plus,
  Star,
  Bot,
  Trash2,
  PanelLeftClose,
  PanelLeftOpen,
  PanelRight,
  X,
  Folder,
  ChevronRight,
} from "lucide-react";
import WorkspaceSearchSelect from "./WorkspaceSearchSelect";
import TabStrip from "./TabStrip";
import SessionPane, { type PaneActions } from "./SessionPane";
import LayoutSash from "./LayoutSash";
import {
  type LayoutNode,
  type SplitDirection,
  type LeafNode,
  createEmptyLeaf,
  newLeafId,
  findLeaf,
  firstLeaf,
  collectLeaves,
  hasOpenTabs,
  updateLeaf,
  splitLeaf,
  closeTabInLeaf,
  moveTabBetweenLeaves,
  removeSessionFromTree,
} from "./layoutTree";
import "./Chat.css";
import {
  API_BASE,
  startOpencode,
  deleteSession,
} from "../../api/chat";
import type { Session } from "./types";

function formatTime(ms?: number): string {
  if (!ms) return "";
  const d = new Date(ms);
  const now = new Date();
  const sameDay = d.toDateString() === now.toDateString();
  return sameDay
    ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })
    : d.toLocaleDateString([], { day: "2-digit", month: "short" });
}

function workspaceLabel(dir?: string): string {
  if (!dir) return "No workspace";
  const parts = dir.replace(/\/+$/, "").split("/");
  return parts[parts.length - 1] || dir;
}

export default function Chat() {
  // ── Session list (sidebar) ─────────────────────────────────────────────
  const [sessions, setSessions] = useState<Session[]>([]);
  const [searchParams, setSearchParams] = useSearchParams();
  const [workspaceFilter, setWorkspaceFilter] = useState<string>("");
  const [projects, setProjects] = useState<
    { id: string; path: string; name: string }[]
  >([]);

  // ── Pane management — VS Code-style split tree ─────────────────────────
  const [layout, setLayout] = useState<LayoutNode>(() => createEmptyLeaf());
  const [focusedLeaf, setFocusedLeaf] = useState<string>("");

  // Initialize focusedLeaf to the root leaf once layout is set/changed by
  // the URL restore effect.
  useEffect(() => {
    if (!focusedLeaf) setFocusedLeaf(firstLeaf(layout).id);
  }, [layout, focusedLeaf]);

  // Derive activeTab for sidebar rendering
  const activeTab = findLeaf(layout, focusedLeaf)?.active ?? null;

  // ── Pins & templates ──────────────────────────────────────────────────
  const [pinned, setPinned] = useState<string[]>([]);
  const [templates, setTemplates] = useState<
    { id: string; name: string; content: string }[]
  >([]);
  const [showTemplates, setShowTemplates] = useState(false);

  // ── Sidebar chrome ────────────────────────────────────────────────────
  // Auto-collapse the session sidebar on entry to chat (the dashboard nav
  // rail is also collapsed on /chat). The user can still expand it manually
  // via the expand button; that preference is persisted below.
  const [sidebarCollapsed, setSidebarCollapsed] = useState(() => {
    try { return localStorage.getItem("chat-sidebar-collapsed") !== "0"; }
    catch { return true; }
  });
  useEffect(() => {
    try { localStorage.setItem("chat-sidebar-collapsed", sidebarCollapsed ? "1" : "0"); }
    catch { /* ignore */ }
  }, [sidebarCollapsed]);
  const [mobileDrawerOpen, setMobileDrawerOpen] = useState(false);
  // Resizable sidebar width, persisted to localStorage (chat-sidebar-w).
  const [sidebarWidth, setSidebarWidth] = useState<number>(() => {
    try {
      const saved = localStorage.getItem("chat-sidebar-w");
      if (saved) return Math.max(200, Math.min(480, parseInt(saved, 10) || 264));
    } catch { /* private mode */ }
    return 264;
  });
  useEffect(() => {
    try { localStorage.setItem("chat-sidebar-w", String(sidebarWidth)); }
    catch { /* ignore */ }
  }, [sidebarWidth]);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(
    new Set(),
  );
  const [expandedAgents, setExpandedAgents] = useState<Set<string>>(
    new Set(),
  );
  const [searchQuery, setSearchQuery] = useState("");

  // ── Shared state ──────────────────────────────────────────────────────
  const [providers, setProviders] = useState<
    { id: string; name: string; models: string[] }[]
  >([]);
  const [agents, setAgents] = useState<
    { name: string; description?: string }[]
  >([]);
  const [busySessions, setBusySessions] = useState<Set<string>>(new Set());
  const [opencodeDown, setOpencodeDown] = useState(false);
  const [startingOpencode, setStartingOpencode] = useState(false);
  // ── Ref for pane actions (template insertion, focus) ───────────────────
  const paneActionsRef = useRef<PaneActions | null>(null);

  // ── Drag-over highlight: which leaf is currently being dragged over ────
  const [dragOverLeaf, setDragOverLeaf] = useState<string | null>(null);

  // ── Header state reported by the focused leaf (rendered in its tab strip) ─
  const [focusedHeader, setFocusedHeader] = useState<{
    subagentCount: number;
    rightPanelOpen: boolean;
    ctxPct: number | null;
  } | null>(null);

  // ── Restore layout from URL on mount ───────────────────────────────────
  useEffect(() => {
    // New canonical: ?layout=<tree-json>
    const layoutParam = searchParams.get("layout");
    if (layoutParam) {
      try {
        const parsed = JSON.parse(layoutParam) as LayoutNode;
        if (parsed && typeof parsed === "object" && "kind" in parsed) {
          setLayout(parsed);
          setFocusedLeaf(firstLeaf(parsed).id);
          return;
        }
      } catch { /* fall through */ }
    }

    // Legacy ?panes=<flat-array> → convert to a row-split of leaves.
    const panesParam = searchParams.get("panes");
    if (panesParam) {
      try {
        const parsed = JSON.parse(panesParam) as {
          id: string;
          tabs: string[];
          active: string | null;
        }[];
        if (Array.isArray(parsed) && parsed.length > 0) {
          const leaves: LeafNode[] = parsed.map((p) => ({
            kind: "leaf" as const,
            id: p.id,
            tabs: p.tabs,
            active: p.active,
          }));
          const tree: LayoutNode =
            leaves.length === 1
              ? leaves[0]
              : {
                  kind: "split",
                  id: `split-${Date.now().toString(36)}`,
                  direction: "row",
                  children: leaves,
                };
          setLayout(tree);
          setFocusedLeaf(leaves[0].id);
          return;
        }
      } catch { /* fall through */ }
    }

    // Older legacy: ?tabs= & ?active= → single leaf.
    const tabsParam = searchParams.get("tabs");
    const activeParam = searchParams.get("active");
    const sessionParam = searchParams.get("session");

    if (tabsParam) {
      const ids = tabsParam.split(",").filter(Boolean);
      if (ids.length > 0) {
        const active = activeParam && ids.includes(activeParam) ? activeParam : ids[0];
        const leaf: LeafNode = { kind: "leaf", id: newLeafId(), tabs: ids, active };
        setLayout(leaf);
        setFocusedLeaf(leaf.id);
        return;
      }
    }
    if (sessionParam) {
      const leaf: LeafNode = { kind: "leaf", id: newLeafId(), tabs: [sessionParam], active: sessionParam };
      setLayout(leaf);
      setFocusedLeaf(leaf.id);
    }
  }, []); // mount only

  // ── Persist layout to URL ─────────────────────────────────────────────
  useEffect(() => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set("layout", JSON.stringify(layout));
      // Mirror the focused leaf into legacy params for backward-compat links.
      const focused = findLeaf(layout, focusedLeaf);
      if (focused && focused.tabs.length > 0) {
        next.set("tabs", focused.tabs.join(","));
        if (focused.active) next.set("active", focused.active);
      } else {
        next.delete("tabs");
        next.delete("active");
      }
      next.delete("panes");
      next.delete("session");
      return next;
    }, { replace: true });
  }, [layout, focusedLeaf]);

  // ── Open a session in the focused leaf ─────────────────────────────────
  const openSession = useCallback(
    (id: string) => {
      const leaf = findLeaf(layout, focusedLeaf);
      if (!leaf) return;
      const nextTabs = leaf.tabs.includes(id)
        ? leaf.tabs
        : [...leaf.tabs, id];
      setLayout((prev) => updateLeaf(prev, focusedLeaf, { tabs: nextTabs, active: id }));
    },
    [layout, focusedLeaf],
  );

  // ── Close a tab in a leaf ─────────────────────────────────────────────
  const closeTab = useCallback(
    (leafId: string, sessionId: string) => {
      setLayout((prev) => {
        const { tree, nextFocusLeafId } = closeTabInLeaf(prev, leafId, sessionId);
        if (nextFocusLeafId) setFocusedLeaf(nextFocusLeafId);
        return tree;
      });
    },
    [],
  );

  // ── Move a session tab between leaves (DnD) ──────────────────────────
  const moveTab = useCallback(
    (sessionId: string, fromLeafId: string, toLeafId: string, insertIndex?: number) => {
      setLayout((prev) =>
        moveTabBetweenLeaves(prev, sessionId, fromLeafId, toLeafId, insertIndex),
      );
      setFocusedLeaf(toLeafId);
    },
    [],
  );

  // ── Split a session into a new leaf ──────────────────────────────────
  const splitSession = useCallback(
    (sessionId: string, direction: SplitDirection) => {
      setLayout((prev) => {
        // Find the leaf that owns this session; split IT (not just the
        // focused leaf) so the context menu matches the right-clicked tab.
        const ownerLeaf = collectLeaves(prev).find((l) => l.tabs.includes(sessionId));
        const targetLeafId = ownerLeaf?.id ?? focusedLeaf;
        const newLeaf: LeafNode = {
          kind: "leaf",
          id: newLeafId(),
          tabs: [sessionId],
          active: sessionId,
        };
        const { tree, newLeafId: newId } = splitLeaf(prev, targetLeafId, direction, newLeaf);
        setFocusedLeaf(newId);
        return tree;
      });
    },
    [focusedLeaf],
  );

  // ── Select a tab (also focuses its leaf) ─────────────────────────────
  const selectTab = useCallback(
    (leafId: string, sessionId: string) => {
      setLayout((prev) => updateLeaf(prev, leafId, { active: sessionId }));
      setFocusedLeaf(leafId);
    },
    [],
  );

  // ── Close a session from the whole tree (for delete) ─────────────────
  const closeSessionEverywhere = useCallback((id: string) => {
    setLayout((prev) => removeSessionFromTree(prev, id));
  }, []);

  // ── Data loading ──────────────────────────────────────────────────────
  const loadSessions = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`);
      // The proxy returns 502/503 when no opencode server is reachable; use that
      // as the "opencode down" signal (there is no /api/chat/health route).
      if (resp.status === 502 || resp.status === 503) {
        setOpencodeDown(true);
        return;
      }
      if (resp.ok) {
        setOpencodeDown(false);
        const data = await resp.json();
        setSessions(data.sessions || []);
      }
    } catch {
      // ignore
    }
  };

  const loadAgents = async () => {
    try {
      const resp = await fetch(`${API_BASE}/agents`);
      if (!resp.ok) return;
      const data = await resp.json();
      setAgents(
        (data.agents || []).map((a: any) => ({
          name: a.name,
          description: a.description,
        })),
      );
    } catch {
      // ignore
    }
  };

  const loadProviders = async () => {
    try {
      const resp = await fetch(`${API_BASE}/providers`);
      if (!resp.ok) return;
      const data = await resp.json();
      const list = (data.providers || []).map((p: any) => ({
        id: p.id,
        name: p.name || p.id,
        models: Object.keys(p.models || {}).sort(),
      }));
      setProviders(list);
    } catch {
      // ignore
    }
  };

  const loadPins = async () => {
    try {
      const resp = await fetch(`${API_BASE}/pins`);
      if (resp.ok) setPinned((await resp.json()).pins || []);
    } catch {
      // ignore
    }
  };

  const togglePin = async (id: string) => {
    const isPinned = pinned.includes(id);
    try {
      if (isPinned) {
        await fetch(`${API_BASE}/pins/${id}`, { method: "DELETE" });
        setPinned((p) => p.filter((x) => x !== id));
      } else {
        await fetch(`${API_BASE}/pins`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ id }),
        });
        setPinned((p) => [...p, id]);
      }
    } catch {
      // ignore
    }
  };

  const loadTemplates = async () => {
    try {
      const resp = await fetch(`${API_BASE}/templates`);
      if (resp.ok) setTemplates((await resp.json()).templates || []);
    } catch {
      // ignore
    }
  };

  const loadProjects = async () => {
    try {
      const resp = await fetch(`${API_BASE}/projects`);
      if (resp.ok) setProjects((await resp.json()).projects || []);
    } catch {
      // ignore
    }
  };

  const createTemplate = async (name: string, content: string) => {
    try {
      const resp = await fetch(`${API_BASE}/templates`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name, content }),
      });
      if (resp.ok) loadTemplates();
    } catch {
      // ignore
    }
  };

  const deleteTemplate = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await fetch(`${API_BASE}/templates/${id}`, { method: "DELETE" });
      loadTemplates();
    } catch {
      // ignore
    }
  };

  const insertTemplate = (content: string) => {
    paneActionsRef.current?.insertText(content);
    setShowTemplates(false);
  };

  // ── Create session ────────────────────────────────────────────────────
  const createSession = async () => {
    try {
      const resp = await fetch(`${API_BASE}/sessions`, { method: "POST" });
      if (!resp.ok) return;
      const data = await resp.json();
      const newId = data.id || data.sessionID;
      if (newId) {
        openSession(newId);
        loadSessions();
      }
    } catch {
      // ignore
    }
  };

  // ── Start OpenCode server ─────────────────────────────────────────────
  const handleStartOpencode = async () => {
    setStartingOpencode(true);
    try {
      // The endpoint returns 202 with status:"starting" immediately — the
      // opencode server takes a few seconds to bind its port. Poll the
      // sessions endpoint until it responds 200 (or we give up after ~30s),
      // otherwise loadSessions' 5s interval keeps flipping opencodeDown back
      // on while the server is mid-boot, re-showing the banner.
      await startOpencode();
      const startedAt = Date.now();
      const TIMEOUT_MS = 30_000;
      const POLL_MS = 1_000;
      let up = false;
      while (Date.now() - startedAt < TIMEOUT_MS) {
        await new Promise((r) => setTimeout(r, POLL_MS));
        try {
          const resp = await fetch(`${API_BASE}/sessions`, { method: "GET" });
          if (resp.ok) { up = true; break; }
        } catch { /* keep polling */ }
      }
      if (up) {
        setOpencodeDown(false);
        loadSessions();
        loadProviders();
        loadAgents();
      }
    } catch {
      // ignore — the banner stays; the user can retry
    }
    setStartingOpencode(false);
  };

  // ── Init effects ──────────────────────────────────────────────────────
  useEffect(() => {
    loadSessions();
    loadProviders();
    loadAgents();
    loadPins();
    loadProjects();
  }, []);

  // Poll session list periodically
  useEffect(() => {
    const id = setInterval(loadSessions, 5000);
    return () => clearInterval(id);
  }, []);

  // Busy state comes from each SessionPane's SSE stream (OpenCode session
  // objects carry no status field, so polling can't see it). Panes report per
  // session id; we keep the shared set here.
  const handleBusyChange = useCallback((id: string, busy: boolean) => {
    setBusySessions((prev) => {
      if (busy === prev.has(id)) return prev;
      const next = new Set(prev);
      if (busy) next.add(id);
      else next.delete(id);
      return next;
    });
  }, []);

  // opencode reachability is derived from loadSessions' status code (see above);
  // there is no dedicated /api/chat/health route.


  // ── Sidebar: rename / delete ──────────────────────────────────────────
  const handleDeleteSession = async (id: string) => {
    if (!confirm("Delete this session?")) return;
    await deleteSession(id);
    closeSessionEverywhere(id);
    loadSessions();
  };

  // ── Sidebar: render session item ──────────────────────────────────────
  const renderSessionItem = (
    s: Session,
    isChild: boolean,
    childCount: number,
    agentOpen: boolean,
  ) => {
    const isActive = s.id === activeTab;
    const isPinned = pinned.includes(s.id);
    const isBusy = busySessions.has(s.id);

    return (
      <div
        key={s.id}
        className={`session-item ${isActive ? "active" : ""} ${isChild ? "is-subagent" : ""}`}
        onClick={() => openSession(s.id)}
      >
        {isBusy && <span className="session-busy-indicator" />}
        {!isChild && childCount > 0 ? (
          <button
            className={`session-agent-toggle ${agentOpen ? "open" : ""}`}
            onClick={(e) => {
              e.stopPropagation();
              setExpandedAgents((prev) => {
                const next = new Set(prev);
                if (next.has(s.id)) next.delete(s.id);
                else next.add(s.id);
                return next;
              });
            }}
          >
            <ChevronRight size={13} />
          </button>
        ) : isChild ? (
          <Bot
            size={13}
            className="session-subagent-icon"
            data-tip="Subagent session"
          />
        ) : null}
        <span
          className={`session-title ${s.title ? "" : "is-placeholder"}`}
        >
          {s.title || `Session ${s.id.slice(0, 8)}`}
        </span>
        {childCount > 0 && (
          <span className="session-subagent-count" data-tip="Subagents">
            {childCount}
          </span>
        )}
        <span className="session-time">{formatTime(s.time?.created)}</span>
        <button
          className={`btn-pin ${isPinned ? "pinned" : ""}`}
          onClick={(e) => {
            e.stopPropagation();
            togglePin(s.id);
          }}
          data-tip={isPinned ? "Unpin" : "Pin"}
        >
          <Star size={12} fill={isPinned ? "currentColor" : "none"} />
        </button>
        <button
          className="btn-delete-session"
          onClick={(e) => {
            e.stopPropagation();
            handleDeleteSession(s.id);
          }}
          data-tip="Delete"
        >
          <Trash2 size={12} />
        </button>
      </div>
    );
  };

  // ── Sidebar: group sessions by workspace ──────────────────────────────
  const pinnedSessions = sessions.filter((s) => pinned.includes(s.id));
  const unpinnedSessions = sessions.filter((s) => !pinned.includes(s.id));

  // Group unpinned by workspace (directory)
  const workspaceGroups = new Map<string, Session[]>();
  for (const s of unpinnedSessions) {
    const ws = workspaceLabel(s.directory);
    if (!workspaceGroups.has(ws)) workspaceGroups.set(ws, []);
    workspaceGroups.get(ws)!.push(s);
  }

  // Filter by search query
  const filterSessions = (list: Session[]) => {
    if (!searchQuery) return list;
    const q = searchQuery.toLowerCase();
    return list.filter(
      (s) =>
        (s.title || "").toLowerCase().includes(q) ||
        s.id.toLowerCase().includes(q),
    );
  };

  // Build agent tree: root sessions (no parentID) with their children
  const buildAgentTrees = (list: Session[]) => {
    const roots: Session[] = [];
    const childrenMap = new Map<string, Session[]>();
    for (const s of list) {
      if (s.parentID) {
        if (!childrenMap.has(s.parentID))
          childrenMap.set(s.parentID, []);
        childrenMap.get(s.parentID)!.push(s);
      } else {
        roots.push(s);
      }
    }
    return { roots, childrenMap };
  };

  // ── Recursive layout renderer ────────────────────────────────────────
  // A leaf renders its TabStrip + SessionPane; a split renders its children
  // in a row/column with LayoutSash dividers between them. This closure lets
  // every node share the same callbacks without prop-drilling depth.
  const renderNode = (node: LayoutNode): React.ReactNode => {
    if (node.kind === "leaf") {
      const isFocused = node.id === focusedLeaf;
      return (
        <div
          key={node.id}
          className={`layout-leaf ${isFocused ? "layout-leaf-focused" : ""} ${dragOverLeaf === node.id ? "layout-leaf-drop-target" : ""}`}
          onMouseDown={() => {
            if (!isFocused) setFocusedLeaf(node.id);
          }}
          onDragOver={(e) => {
            if (e.dataTransfer.types.includes("application/x-session-id")) {
              e.preventDefault();
              setDragOverLeaf(node.id);
            }
          }}
          onDragLeave={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node)) {
              setDragOverLeaf((cur) => (cur === node.id ? null : cur));
            }
          }}
          onDrop={() => setDragOverLeaf(null)}
        >
          <TabStrip
            leafId={node.id}
            openSessions={node.tabs}
            activeTab={node.active}
            sessions={sessions}
            onSelect={(id) => selectTab(node.id, id)}
            onClose={(id) => closeTab(node.id, id)}
            onMoveSession={moveTab}
            onSplitRight={(id) => splitSession(id, "row")}
            onSplitDown={(id) => splitSession(id, "column")}
            trailing={
              isFocused ? (
                <div className="tab-strip-actions">
                  {focusedHeader && focusedHeader.ctxPct !== null && (
                    <span
                      className={`tab-context-meter ${focusedHeader.ctxPct >= 80 ? "warn" : ""}`}
                      data-tip={`${focusedHeader.ctxPct}% context`}
                    >
                      {focusedHeader.ctxPct}%
                    </span>
                  )}
                  <button
                    className={`btn-icon sm ${focusedHeader?.rightPanelOpen ? "active" : ""}`}
                    onClick={() => paneActionsRef.current?.toggleRightPanel()}
                    aria-label="Toggle subagents panel"
                    disabled={!paneActionsRef.current}
                    data-tip="Subagents"
                  >
                    <PanelRight size={16} />
                  </button>
                  {focusedHeader && focusedHeader.subagentCount > 0 && (
                    <span className="tab-subagents-count" data-tip="Subagents">
                      {focusedHeader.subagentCount}
                    </span>
                  )}
                </div>
              ) : null
            }
          />
          {node.active ? (
            <div className="session-pane-container">
              <SessionPane
                key={node.active}
                sessionId={node.active}
                sessions={sessions}
                workspaceFilter={workspaceFilter}
                busySessions={busySessions}
                providers={providers}
                agents={agents}
                onOpenSession={openSession}
                onToggleTemplates={() => {
                  setShowTemplates(!showTemplates);
                  if (!showTemplates) loadTemplates();
                }}
                paneActionsRef={isFocused ? paneActionsRef : { current: null }}
                onBusyChange={handleBusyChange}
                isFocused={isFocused}
                onHeaderStateChange={setFocusedHeader}
              />
            </div>
          ) : (
            <div className="pane-empty">
              <Bot size={32} />
              <span>No session open</span>
            </div>
          )}
        </div>
      );
    }

    // Split node: lay out children in the requested direction with sashes.
    const axis = node.direction === "row" ? "x" : "y";
    return (
      <div
        key={node.id}
        className={`layout-split layout-${node.direction}`}
      >
        {node.children.map((child, i) => (
          <div key={child.kind === "leaf" ? child.id : child.id} className="layout-child">
            {renderNode(child)}
            {i < node.children.length - 1 && <LayoutSash axis={axis} />}
          </div>
        ))}
      </div>
    );
  };

  // ── Render ────────────────────────────────────────────────────────────
  return (
    <div className="chat-container">
      {/* Sidebar */}
      <aside
        className={`chat-sidebar ${sidebarCollapsed ? "collapsed" : ""} ${mobileDrawerOpen ? "open" : ""}`}
        style={!sidebarCollapsed ? { flex: `0 0 ${sidebarWidth}px`, width: sidebarWidth } : undefined}
      >
        <div className="sidebar-header">
          {!sidebarCollapsed && (
            <>
              <h3 className="sidebar-title">Sessions</h3>
              <div className="sidebar-actions">
                <button
                  className="btn-icon"
                  onClick={createSession}
                  data-tip="New session"
                  data-tip-pos="bottom"
                >
                  <Plus size={16} />
                </button>
                <button
                  className="btn-icon"
                  onClick={() => setSidebarCollapsed(true)}
                  data-tip="Collapse sidebar"
                  data-tip-pos="bottom"
                >
                  <PanelLeftClose size={16} />
                </button>
              </div>
            </>
          )}
          {sidebarCollapsed && (
            <button
              className="btn-icon"
              onClick={() => setSidebarCollapsed(false)}
              data-tip="Expand sidebar"
              data-tip-pos="right"
            >
              <PanelLeftOpen size={16} />
            </button>
          )}
        </div>

        {!sidebarCollapsed && (
          <>
            {/* Workspace filter */}
            <div className="sidebar-filter">
              <WorkspaceSearchSelect
                workspaces={projects}
                value={workspaceFilter}
                onChange={setWorkspaceFilter}
              />
            </div>

            {/* Search */}
            <input
              className="sidebar-search"
              placeholder="Search sessions…"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />

            {/* Session list */}
            <div className="session-list">
              {/* Pinned */}
              {pinnedSessions.length > 0 && (
                <div className="session-group">
                  <div className="session-group-header">
                    <Star size={12} /> Pinned
                  </div>
                  {filterSessions(pinnedSessions).map((s) =>
                    renderSessionItem(s, false, 0, false),
                  )}
                </div>
              )}

              {/* Workspace groups */}
              {Array.from(workspaceGroups.entries()).map(
                ([ws, groupSessions]) => {
                  const filtered = filterSessions(groupSessions);
                  if (filtered.length === 0) return null;
                  const isCollapsed = collapsedGroups.has(ws);
                  return (
                    <div className="session-group" key={ws}>
                      <div
                        className="session-group-header"
                        onClick={() =>
                          setCollapsedGroups((prev) => {
                            const next = new Set(prev);
                            if (next.has(ws)) next.delete(ws);
                            else next.add(ws);
                            return next;
                          })
                        }
                      >
                        <Folder size={12} />
                        <span>{ws}</span>
                        <span className="session-group-count">
                          {filtered.length}
                        </span>
                      </div>
                      {!isCollapsed &&
                        (() => {
                          const { roots, childrenMap } =
                            buildAgentTrees(filtered);
                          return (
                            <div className="session-group-items">
                              {roots.map((root) => {
                                const kids =
                                  childrenMap.get(root.id) || [];
                                const agentOpen =
                                  expandedAgents.has(root.id);
                                return (
                                  <div
                                    className="agent-node"
                                    key={root.id}
                                  >
                                    {renderSessionItem(
                                      root,
                                      false,
                                      kids.length,
                                      agentOpen,
                                    )}
                                    {agentOpen &&
                                      kids.map((k) =>
                                        renderSessionItem(
                                          k,
                                          true,
                                          0,
                                          false,
                                        ),
                                      )}
                                  </div>
                                );
                              })}
                            </div>
                          );
                        })()}
                    </div>
                  );
                },
              )}
              {sessions.length === 0 && (
                <div className="chat-empty">
                  No conversations yet.
                  <br />
                  Tap <strong>New</strong> to start.
                </div>
              )}
            </div>
          </>
        )}
      </aside>

      {/* Sidebar resize sash — hidden when collapsed or on mobile (where the
          sidebar becomes an off-canvas drawer). */}
      {!sidebarCollapsed && (
        <div className="sidebar-sash-wrapper">
          <LayoutSash
            axis="x"
            min={200}
            onResizeEnd={(w) => setSidebarWidth(w)}
          />
        </div>
      )}

      {/* Mobile overlay */}
      {mobileDrawerOpen && (
        <div
          className="sidebar-overlay"
          onClick={() => setMobileDrawerOpen(false)}
        />
      )}

      {/* Main area */}
      <main className="chat-main">
        {/* Agent selector bar */}
        {/* VS Code-style split grid. renderNode recurses the layout tree. */}
        {hasOpenTabs(layout) ? (
          <div className="layout-root">{renderNode(layout)}</div>
        ) : (
          <div className="chat-placeholder">
            <div className="chat-placeholder-inner">
              <Bot size={48} className="chat-placeholder-icon" />
              <h2>Welcome to ywai</h2>
              <p>Select a session or create a new one.</p>
              <button
                className="btn-new-session"
                onClick={createSession}
              >
                <Plus size={16} /> New session
              </button>
            </div>
          </div>
        )}

        {/* OpenCode down banner */}
        {opencodeDown && (
          <div className="chat-banner chat-banner-warn" role="status">
            <div className="chat-banner-text">
              <strong>OpenCode server is not running.</strong>
              <span>
                The chat needs an opencode server. Start one now or run{" "}
                <code>opencode serve</code> in a terminal.
              </span>
            </div>
            <button
              className="btn-new-session"
              onClick={handleStartOpencode}
              disabled={startingOpencode}
            >
              {startingOpencode ? "Starting…" : "Start OpenCode"}
            </button>
          </div>
        )}

        {/* Templates panel (overlay) */}
        {showTemplates && (
          <div
            className="templates-backdrop"
            onClick={() => setShowTemplates(false)}
          />
        )}
        {showTemplates && (
          <div className="templates-panel">
            <div className="templates-header">
              <h3>Prompt Templates</h3>
              <button
                className="btn-icon"
                onClick={() => setShowTemplates(false)}
              >
                <X size={16} />
              </button>
            </div>
            <div className="templates-list">
              {templates.map((t) => (
                <div
                  key={t.id}
                  className="template-item"
                  onClick={() => insertTemplate(t.content)}
                >
                  <div className="template-name">{t.name}</div>
                  <button
                    className="btn-icon"
                    onClick={(e) => deleteTemplate(t.id, e)}
                  >
                    <Trash2 size={12} />
                  </button>
                </div>
              ))}
              {templates.length === 0 && (
                <div className="templates-empty">No templates yet.</div>
              )}
            </div>
            <div className="templates-create">
              <input
                className="template-input"
                placeholder="Template name…"
                id="template-name"
              />
              <textarea
                className="template-textarea"
                placeholder="Template content…"
                id="template-content"
                rows={3}
              />
              <button
                className="btn-new-session"
                onClick={() => {
                  const nameEl = document.getElementById(
                    "template-name",
                  ) as HTMLInputElement;
                  const contentEl = document.getElementById(
                    "template-content",
                  ) as HTMLTextAreaElement;
                  if (nameEl?.value && contentEl?.value) {
                    createTemplate(nameEl.value, contentEl.value);
                    nameEl.value = "";
                    contentEl.value = "";
                  }
                }}
              >
                Save
              </button>
            </div>
          </div>
        )}
      </main>
    </div>
  );
}
