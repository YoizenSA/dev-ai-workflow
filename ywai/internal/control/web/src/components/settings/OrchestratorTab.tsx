import { useCallback, useEffect, useMemo, useState } from "react";
import {
	ReactFlow,
	ReactFlowProvider,
	useReactFlow,
	Background,
	Controls,
	MiniMap,
	type Node,
	type Edge,
	type Connection,
	type NodeProps,
	Handle,
	Position,
	MarkerType,
	BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import {
	Download,
	Plus,
	RefreshCw,
	Upload,
	Trash2,
	X,
	Save,
} from "lucide-react";
import { useAgentsDiagramStore } from "../../stores/agentsDiagramStore";
import { configApi, missionsApi } from "../../api/client";
import type {
	AgentGraphNode,
	AgentGraphEdge,
	DelegationRule,
	DelegationTrigger,
	ModelInfo,
} from "../../api/types";
import Modal from "../shared/Modal";
import ImportAgentsModal from "./ImportAgentsModal";
import ModelCombobox from "../missions/ModelCombobox";
import "./Orchestrator.css";

/**
 * Orchestrator tab: visual editor for OpenCode agents and the static
 * delegation graph between them (derived from each agent's permission.task).
 *
 * Three editing surfaces:
 *  1. Canvas: agents grouped by `group` into draggable lanes (xyflow v12
 *     parentId + extent:'parent'); sidebar toggles group visibility.
 *  2. permission.task matrix (opencode.json) — allow/ask/deny per subagent.
 *  3. "Delegation Rules" + "Mandatory Delegation Triggers" sections of the
 *     agent's prompt body (markdown), edited as structured data.
 */

type AgentNodeData = {
	name: string;
	mode?: string;
	model?: string;
	group?: string;
	hasWildcard?: boolean;
	wildcardValue?: string;
	ghost?: boolean;
};

type GroupNodeData = { label: string };

// Horizontal spacing between topological layers (delegation hops) inside a lane.
const COL_W = 280;
const ROW_H = 110;
const LANE_PAD_X = 24;
const LANE_PAD = 48; // top padding inside a lane (room for the header)
const LANE_GAP = 64; // vertical gap between lanes

const UNGROUPED = "ungrouped";

function groupOf(node: AgentGraphNode): string {
	return node.group || UNGROUPED;
}

/**
 * Compute the topological layer of each node within a group's delegation
 * sub-graph: roots (nothing delegates to them) are layer 0, and each node's
 * layer = 1 + max(layer of its inbound delegators). Edges that cross group
 * boundaries are ignored so each lane is laid out by its own internal chain.
 *
 * This produces a clean left→right flow (orchestrator → roles → leafs) instead
 * of the old mode-based columns where every `mode: all` node stacked together.
 */
function topoLayers(
	nodes: AgentGraphNode[],
	edges: { source: string; target: string }[],
): { layer: Map<string, number>; maxLayer: number } {
	const ids = new Set(nodes.map((n) => n.id));
	// inbound edges within this group only.
	const inbound = new Map<string, string[]>();
	for (const id of ids) inbound.set(id, []);
	for (const e of edges) {
		if (ids.has(e.source) && ids.has(e.target)) {
			inbound.get(e.target)!.push(e.source);
		}
	}
	// Longest path from any root: layer(n) = 1 + max(layer(p) for p in inbound).
	const layer = new Map<string, number>();
	const cache = new Map<string, number>();
	const compute = (id: string, stack: Set<string>): number => {
		if (cache.has(id)) return cache.get(id)!;
		if (stack.has(id)) return 0; // cycle guard
		stack.add(id);
		const deps = inbound.get(id) ?? [];
		let best = 0;
		for (const d of deps) best = Math.max(best, compute(d, stack) + 1);
		stack.delete(id);
		cache.set(id, best);
		return best;
	};
	let maxLayer = 0;
	for (const id of ids) {
		const l = compute(id, new Set());
		layer.set(id, l);
		if (l > maxLayer) maxLayer = l;
	}
	return { layer, maxLayer };
}

/** Deterministic group ordering (insertion order, ungrouped always last). */
function orderedGroups(nodes: AgentGraphNode[]): string[] {
	const seen = new Set<string>();
	const order: string[] = [];
	for (const n of nodes) {
		const g = groupOf(n);
		if (!seen.has(g)) {
			seen.add(g);
			if (g !== UNGROUPED) order.push(g);
		}
	}
	if (seen.has(UNGROUPED)) order.push(UNGROUPED);
	return order;
}

type LaneLayout = {
	groupNodes: Node<GroupNodeData>[];
	agentNodes: Node<AgentNodeData>[];
	laneOf: Record<string, number>;
};

/**
 * Build group-lane container nodes + agent nodes parented to their lane.
 * Agent positions are relative to the lane (v12 semantics with parentId) and
 * assigned a column by their topological layer within the group's delegation
 * sub-graph, so delegation chains read left→right.
 */
function buildGroupedNodes(graph: {
	nodes: AgentGraphNode[];
	edges: { source: string; target: string }[];
}): LaneLayout {
	const groups = orderedGroups(graph.nodes);
	const laneOf: Record<string, number> = {};
	groups.forEach((g, i) => (laneOf[g] = i));

	const agentNodes: Node<AgentNodeData>[] = [];
	const groupNodes: Node<GroupNodeData>[] = [];

	for (const g of groups) {
		const groupNodesRaw = graph.nodes.filter((n) => groupOf(n) === g);
		const { layer, maxLayer } = topoLayers(groupNodesRaw, graph.edges);

		// rows per layer for vertical stacking.
		const rowsIn: Record<number, number> = {};
		for (const n of groupNodesRaw) {
			const l = layer.get(n.id) ?? 0;
			const row = rowsIn[l] ?? 0;
			rowsIn[l] = row + 1;
			agentNodes.push({
				id: n.id,
				type: "agent",
				parentId: `group:${g}`,
				extent: "parent",
				expandParent: false,
				position: {
					x: LANE_PAD_X + l * COL_W,
					y: LANE_PAD + row * ROW_H,
				},
				data: {
					name: n.name,
					mode: n.mode,
					model: n.model,
					group: n.group,
					hasWildcard: n.hasWildcard,
					wildcardValue: n.wildcardValue,
					ghost: n.ghost,
				},
			});
		}

		const maxRows = Math.max(...Object.values(rowsIn), 0);
		const laneH = LANE_PAD + (maxRows + 1) * ROW_H;
		const laneW = LANE_PAD_X * 2 + (maxLayer + 1) * COL_W;
		groupNodes.push({
			id: `group:${g}`,
			type: "groupNode",
			position: { x: 0, y: laneOf[g] * (laneH + LANE_GAP) },
			data: { label: g === UNGROUPED ? "(sin grupo)" : g },
			style: { width: laneW, height: laneH },
		});
	}

	return { groupNodes, agentNodes, laneOf };
}

function buildEdges(graph: {
	nodes: AgentGraphNode[];
	edges: AgentGraphEdge[];
}): Edge[] {
	return graph.edges.map((e) => ({
		id: e.id,
		source: e.source,
		target: e.target,
		animated: e.value === "ask",
		className: `agent-edge agent-edge-${e.value}`,
		markerEnd: { type: MarkerType.ArrowClosed, width: 18, height: 18 },
		label: e.value,
		labelStyle: { fontSize: 10, fill: "var(--text-muted)" },
		labelBgStyle: { fill: "var(--surface)" },
	}));
}

function AgentNode({ data, selected }: NodeProps) {
	const d = data as AgentNodeData;
	const modeClass = `agent-mode-${d.ghost ? "ghost" : d.mode || "all"}`;
	return (
		<div className={`agent-card ${selected ? "is-selected" : ""} ${modeClass}`}>
			<Handle type="target" position={Position.Left} />
			<div className="agent-card-head">
				<span className="agent-card-name">{d.name}</span>
				<span className={`pill pill-sm agent-mode-pill ${modeClass}`}>
					{d.ghost ? "ghost" : d.mode || "all"}
				</span>
			</div>
			{d.model && <div className="agent-card-model">{d.model}</div>}
			{d.hasWildcard && (
				<div className="agent-card-wildcard">
					task{" "}
					<span className={`pill pill-sm wildcard-${d.wildcardValue}`}>
						* = {d.wildcardValue}
					</span>
				</div>
			)}
			<Handle type="source" position={Position.Right} />
		</div>
	);
}

function GroupNode({ data }: NodeProps) {
	const d = data as GroupNodeData;
	return (
		<div className="group-zone">
			<div className="group-zone-header">{d.label}</div>
			<div className="group-zone-body nodrag" />
		</div>
	);
}

const nodeTypes = { agent: AgentNode, groupNode: GroupNode };

// Default delegation rules used when seeding a new section (mirrors gentle).
const DEFAULT_RULES: DelegationRule[] = [
	{ action: "Read to decide/verify (1-3 files)", inline: "Yes", delegate: "No" },
	{ action: "Read to explore/understand (4+ files)", inline: "No", delegate: "Yes" },
	{ action: "Write with analysis (multiple files, new logic)", inline: "No", delegate: "Yes" },
	{ action: "Bash for execution (test, install, external tooling)", inline: "No", delegate: "Yes" },
];
const DEFAULT_TRIGGERS: DelegationTrigger[] = [
	{ name: "4-file rule", description: "if understanding requires reading 4+ files, delegate exploration" },
	{ name: "Multi-file write rule", description: "if implementation touches 2+ non-trivial files, delegate one writer" },
	{ name: "PR rule", description: "before commit/push/PR after code changes, run a fresh-context review" },
];

/**
 * AutoFit re-runs fitView whenever the set of visible groups changes, so the
 * canvas always recenters on what's actually shown instead of staying zoomed
 * on the initial layout. Must live inside a <ReactFlowProvider>.
 */
function AutoFit({ trigger }: { trigger: unknown }) {
	const { fitView } = useReactFlow();
	useEffect(() => {
		// Defer one tick so the new hidden/visible nodes are committed before fit.
		const t = setTimeout(() => fitView({ padding: 0.2, duration: 200 }), 50);
		return () => clearTimeout(t);
	}, [trigger, fitView]);
	return null;
}

export default function OrchestratorTab() {
	const graph = useAgentsDiagramStore((s) => s.graph);
	const loading = useAgentsDiagramStore((s) => s.loading);
	const error = useAgentsDiagramStore((s) => s.error);
	const selected = useAgentsDiagramStore((s) => s.selected);
	const load = useAgentsDiagramStore((s) => s.load);
	const selectAgent = useAgentsDiagramStore((s) => s.selectAgent);
	const cycleEdge = useAgentsDiagramStore((s) => s.cycleEdge);
	const setEdge = useAgentsDiagramStore((s) => s.setEdge);
	const removeEdge = useAgentsDiagramStore((s) => s.removeEdge);
	const setAgentModel = useAgentsDiagramStore((s) => s.setAgentModel);

	const [models, setModels] = useState<ModelInfo[]>([]);
	const [importOpen, setImportOpen] = useState(false);
	const [newAgentOpen, setNewAgentOpen] = useState(false);
	const [newAgentName, setNewAgentName] = useState("");
	const [busy, setBusy] = useState<string | null>(null);
	const [newTarget, setNewTarget] = useState("");
	// null = "not yet initialized": on first graph load we default to showing
	// only the "core" group (every other group hidden). The user can then
	// toggle groups on from the sidebar.
	const [hiddenGroups, setHiddenGroups] = useState<Set<string> | null>(null);

	useEffect(() => {
		load();
	}, [load]);

	useEffect(() => {
		missionsApi
			.listModels()
			.then((r) => setModels(Object.values(r.modelsByProvider).flat()))
			.catch(() => setModels([]));
	}, []);

	const groups = useMemo(() => orderedGroups(graph.nodes), [graph.nodes]);
	const layout = useMemo(() => buildGroupedNodes(graph), [graph]);

	// First-load default: hide every group except "core" so the canvas opens
	// focused. Runs once, after the graph and its groups are known.
	useEffect(() => {
		if (hiddenGroups !== null || groups.length === 0) return;
		setHiddenGroups(new Set(groups.filter((g) => g !== "core")));
	}, [groups, hiddenGroups]);

	// Stamp hidden + selection on top of the computed layout.
	const isHidden = useCallback((g: string) => hiddenGroups?.has(g) ?? false, [hiddenGroups]);
	const decoratedNodes = useMemo(() => {
		const all = [...layout.groupNodes, ...layout.agentNodes];
		return all.map((n) => {
			const g = n.id.startsWith("group:") ? n.id.slice(6) : (n.data as AgentNodeData).group || UNGROUPED;
			return {
				...n,
				hidden: isHidden(g),
				selected: n.id === selected,
			};
		});
	}, [layout, hiddenGroups, selected]);

	const edges = useMemo(() => {
		const base = buildEdges(graph);
		const groupById = new Map(graph.nodes.map((n) => [n.id, groupOf(n)]));
		return base.map((e) => {
			const sg = groupById.get(e.source);
			const tg = groupById.get(e.target);
			const hidden = (sg && isHidden(sg)) || (tg && isHidden(tg));
			return { ...e, hidden: Boolean(hidden) };
		});
	}, [graph, isHidden]);

	const selectedNode = useMemo(
		() => graph.nodes.find((n) => n.id === selected) ?? null,
		[graph.nodes, selected],
	);

	const outgoingEdges = useMemo(
		() => graph.edges.filter((e) => e.source === selected),
		[graph.edges, selected],
	);

	const onConnect = useCallback(
		(params: Connection) => {
			if (!params.source || !params.target) return;
			setEdge(params.source, params.target, "allow").catch(() => undefined);
		},
		[setEdge],
	);

	const onEdgeClick = useCallback(
		(_: React.MouseEvent, edge: Edge) => {
			const [source, target] = edge.id.split("->");
			if (source && target) {
				cycleEdge(source, target).catch(() => undefined);
			}
		},
		[cycleEdge],
	);

	const toggleGroup = useCallback((g: string) => {
		setHiddenGroups((prev) => {
			const base = prev ?? new Set<string>();
			const next = new Set(base);
			if (next.has(g)) next.delete(g);
			else next.add(g);
			return next;
		});
	}, []);

	const handleExport = useCallback(() => {
		const doc: Record<string, unknown> = { $schema: "https://opencode.ai/config.json" };
		for (const n of graph.nodes) {
			if (n.ghost) continue;
			doc[n.id] = { mode: n.mode || "all", model: n.model || undefined };
		}
		const blob = new Blob([JSON.stringify(doc, null, 2)], { type: "application/json" });
		const url = URL.createObjectURL(blob);
		const a = document.createElement("a");
		a.href = url;
		a.download = "opencode-agents.json";
		a.click();
		URL.revokeObjectURL(url);
	}, [graph.nodes]);

	const handleCreateAgent = useCallback(async () => {
		const name = newAgentName.trim();
		if (!name || !/^[a-zA-Z0-9_-]+$/.test(name)) return;
		setBusy("create");
		try {
			await configApi.createAgent(name, "---\nmode: subagent\n---\n\n");
			setNewAgentName("");
			setNewAgentOpen(false);
			await load();
			selectAgent(name);
		} finally {
			setBusy(null);
		}
	}, [newAgentName, load, selectAgent]);

	const handleAddTarget = useCallback(async () => {
		if (!selected || !newTarget.trim()) return;
		const target = newTarget.trim();
		setBusy("add-target");
		try {
			await setEdge(selected, target, "allow");
			setNewTarget("");
		} finally {
			setBusy(null);
		}
	}, [selected, newTarget, setEdge]);

	const handleSetModel = useCallback(
		(model: string) => {
			if (!selected) return;
			setAgentModel(selected, model).catch(() => undefined);
		},
		[selected, setAgentModel],
	);

	return (
		<div className="orchestrator-tab">
			<div className="orchestrator-toolbar">
				<button className="btn btn-sm btn-primary" onClick={() => setNewAgentOpen(true)}>
					<Plus size={14} /> New agent
				</button>
				<button className="btn btn-sm btn-secondary" onClick={() => setImportOpen(true)}>
					<Upload size={14} /> Import opencode.json
				</button>
				<button className="btn btn-sm btn-secondary" onClick={handleExport}>
					<Download size={14} /> Export
				</button>
				<button className="btn btn-sm btn-ghost" onClick={() => load()}>
					<RefreshCw size={14} /> Refresh
				</button>
				{error && <span className="alert alert-danger orch-error">{error}</span>}
				<span className="spacer" />
				<span className="orch-legend">
					<span className="legend-item"><i className="dot allow" /> allow</span>
					<span className="legend-item"><i className="dot ask" /> ask</span>
					<span className="orch-hint">click an edge to cycle allow→ask→remove</span>
				</span>
			</div>

			<div className="orchestrator-body">
				<div className="orchestrator-canvas-wrap">
					{groups.length > 0 && (
						<aside className="orch-group-sidebar">
							<p className="field-label">Groups</p>
							{groups.map((g) => {
								const count = graph.nodes.filter((n) => groupOf(n) === g).length;
								return (
									<label key={g} className="orch-group-toggle">
									<input
										type="checkbox"
										checked={!isHidden(g)}
										onChange={() => toggleGroup(g)}
									/>
										<span>{g === UNGROUPED ? "(sin grupo)" : g}</span>
										<span className="pill pill-sm pill-muted">{count}</span>
									</label>
								);
							})}
						</aside>
					)}

					<div className="orchestrator-canvas">
						{loading && graph.nodes.length === 0 ? (
							<div className="empty-state">Loading delegation graph…</div>
						) : graph.nodes.length === 0 ? (
							<div className="empty-state">
								No agents yet. Create one or import an opencode.json to diagram
								delegations.
							</div>
						) : (
							<ReactFlowProvider>
								<AutoFit trigger={hiddenGroups} />
								<ReactFlow
									nodes={decoratedNodes}
									edges={edges}
									nodeTypes={nodeTypes}
									onConnect={onConnect}
									onEdgeClick={onEdgeClick}
									onNodeClick={(_, n) => selectAgent(n.id)}
									fitView
									proOptions={{ hideAttribution: true }}
								>
									<Background variant={BackgroundVariant.Dots} gap={16} size={1} />
									<Controls />
									<MiniMap
										nodeColor={(n) => {
											const d = n.data as AgentNodeData | GroupNodeData;
											if (n.id.startsWith("group:")) return "var(--surface-hover)";
											const ad = d as AgentNodeData;
											if (ad?.ghost) return "var(--text-faint)";
											return ad?.mode === "primary"
												? "var(--tint-primary)"
												: "var(--tint-purple)";
										}}
									/>
								</ReactFlow>
							</ReactFlowProvider>
						)}
					</div>
				</div>

				<aside className="orchestrator-detail">
					{!selectedNode ? (
						<div className="empty-state orch-detail-empty">
							Select an agent to edit its model and delegations.
						</div>
					) : (
						<AgentDetail
							node={selectedNode}
							models={models}
							outgoing={outgoingEdges}
							newTarget={newTarget}
							onNewTargetChange={setNewTarget}
							onAddTarget={handleAddTarget}
							onCycleEdge={(target) =>
								selected && cycleEdge(selected, target).catch(() => undefined)
							}
							onRemoveEdge={(target) =>
								selected && removeEdge(selected, target).catch(() => undefined)
							}
							onSetModel={handleSetModel}
							onDelete={async () => {
								if (!selected) return;
								await configApi.deleteAgent(selected);
								selectAgent(null);
								await load();
							}}
							busy={busy}
						/>
					)}
				</aside>
			</div>

			<Modal
				open={newAgentOpen}
				onClose={() => setNewAgentOpen(false)}
				title="New agent"
				subtitle="Creates a subagent markdown file; edit its prompt on the Agents tab."
				footer={
					<>
						<button className="btn btn-ghost" onClick={() => setNewAgentOpen(false)}>
							Cancel
						</button>
						<button
							className="btn btn-primary"
							onClick={handleCreateAgent}
							disabled={busy === "create" || !newAgentName.trim()}
						>
							Create
						</button>
					</>
				}
			>
				<div className="field">
					<label className="field-label" htmlFor="new-agent-name">
						Agent name
					</label>
					<input
						id="new-agent-name"
						className="input"
						value={newAgentName}
						onChange={(e) => setNewAgentName(e.target.value)}
						placeholder="e.g. review-readability"
					/>
					<p className="field-hint">
						Lowercase, dashes/underscores. This becomes the file name and the
						delegation target other agents reference.
					</p>
				</div>
			</Modal>

			<ImportAgentsModal open={importOpen} onClose={() => setImportOpen(false)} onDone={load} />
		</div>
	);
}

interface DetailProps {
	node: AgentGraphNode;
	models: ModelInfo[];
	outgoing: AgentGraphEdge[];
	newTarget: string;
	onNewTargetChange: (v: string) => void;
	onAddTarget: () => void;
	onCycleEdge: (target: string) => void;
	onRemoveEdge: (target: string) => void;
	onSetModel: (model: string) => void;
	onDelete: () => Promise<void>;
	busy: string | null;
}

function AgentDetail({
	node,
	models,
	outgoing,
	newTarget,
	onNewTargetChange,
	onAddTarget,
	onCycleEdge,
	onRemoveEdge,
	onSetModel,
	onDelete,
	busy,
}: DetailProps) {
	const valueClass = (v?: string) =>
		v === "allow" ? "pill-success" : v === "ask" ? "pill-warning" : "pill-muted";

	const delegationRules = useAgentsDiagramStore((s) => s.delegationRules);
	const delegationTriggers = useAgentsDiagramStore((s) => s.delegationTriggers);
	const hasDelegationRules = useAgentsDiagramStore((s) => s.hasDelegationRules);
	const loadingRules = useAgentsDiagramStore((s) => s.loadingRules);
	const loadDelegationRules = useAgentsDiagramStore((s) => s.loadDelegationRules);
	const saveDelegationRules = useAgentsDiagramStore((s) => s.saveDelegationRules);

	// Local editable copies of the prompt-body rules.
	const [editRules, setEditRules] = useState<DelegationRule[]>([]);
	const [editTriggers, setEditTriggers] = useState<DelegationTrigger[]>([]);
	const [rulesDirty, setRulesDirty] = useState(false);
	const [savingRules, setSavingRules] = useState(false);

	// Load rules when the selected agent changes or becomes available.
	useEffect(() => {
		if (node && !node.ghost) loadDelegationRules(node.id);
		// Re-run only when the agent id changes; loadDelegationRules is stable
		// enough (zustand action identity) for this not to loop.
	}, [node?.id, loadDelegationRules]);

	// Sync server state into local editable copies when they arrive.
	useEffect(() => {
		setEditRules(delegationRules);
		setEditTriggers(delegationTriggers);
		setRulesDirty(false);
	}, [delegationRules, delegationTriggers]);

	const handleSaveRules = async () => {
		if (!node) return;
		setSavingRules(true);
		try {
			await saveDelegationRules(node.id, editRules, editTriggers);
			setRulesDirty(false);
		} finally {
			setSavingRules(false);
		}
	};

	return (
		<div className="agent-detail">
			<div className="agent-detail-head">
				<div>
					<h3 className="agent-detail-name">{node.name}</h3>
					<span className={`pill pill-sm agent-mode-pill agent-mode-${node.mode || "all"}`}>
						{node.mode || "all"}
					</span>
				</div>
				<button className="btn btn-sm btn-danger" onClick={onDelete} title="Delete agent">
					<Trash2 size={14} />
				</button>
			</div>

			{node.ghost && (
				<div className="alert alert-warning">
					This target is referenced by a delegation but has no agent file. It will be
					created when another agent tries to call it.
				</div>
			)}

			<div className="field">
				<ModelCombobox
					id={`agent-model-${node.id}`}
					label="Model"
					value={node.model || ""}
					models={models}
					onChange={onSetModel}
				/>
			</div>

			{/* #2.A — compact allow/ask/deny delegation matrix (opencode.json task) */}
			<div className="orch-section">
				<div className="orch-section-head">
					<span className="field-label">Delegations (permission.task)</span>
				</div>
				{node.hasWildcard && (
					<div className="orch-wildcard-row">
						Catch-all{" "}
						<span className={`pill pill-sm ${valueClass(node.wildcardValue)}`}>
							* = {node.wildcardValue}
						</span>
					</div>
				)}
				<ul className="orch-delegations">
					{outgoing.length === 0 && (
						<li className="orch-empty">No explicit delegations.</li>
					)}
					{outgoing.map((e) => (
						<li key={e.id} className="orch-delegation">
							<span className="orch-deleg-target">{e.target}</span>
							<button
								className={`pill pill-sm ${valueClass(e.value)}`}
								onClick={() => onCycleEdge(e.target)}
								title="Click to cycle allow → ask → remove"
							>
								{e.value}
							</button>
							<button
								className="btn btn-sm btn-ghost orch-deleg-remove"
								onClick={() => onRemoveEdge(e.target)}
								title="Remove delegation"
							>
								<X size={12} />
							</button>
						</li>
					))}
				</ul>
				<div className="orch-add-target">
					<input
						className="input"
						placeholder="add target agent name…"
						value={newTarget}
						onChange={(e) => onNewTargetChange(e.target.value)}
						onKeyDown={(e) => {
							if (e.key === "Enter") onAddTarget();
						}}
					/>
					<button
						className="btn btn-sm btn-secondary"
						onClick={onAddTarget}
						disabled={busy === "add-target" || !newTarget.trim()}
					>
						<Plus size={14} /> Allow
					</button>
				</div>
			</div>

			{/* #2.B — Delegation Rules editor (prompt-body markdown section) */}
			<div className="orch-section">
				<div className="orch-section-head">
					<span className="field-label">Delegation Rules (prompt)</span>
					{node && !node.ghost && !hasDelegationRules && !loadingRules && (
						<button
							className="btn btn-sm btn-secondary"
							onClick={() => {
								setEditRules(DEFAULT_RULES);
								setEditTriggers(DEFAULT_TRIGGERS);
								setRulesDirty(true);
							}}
						>
							Enable
						</button>
					)}
				</div>

				{loadingRules ? (
					<p className="orch-empty">Loading…</p>
				) : !hasDelegationRules && editRules.length === 0 ? (
					<p className="orch-empty">
						No Delegation Rules section in this agent&apos;s prompt.
					</p>
				) : (
					<>
						<p className="field-hint">Inline vs delegate decision matrix.</p>
						<table className="orch-rules-table">
							<thead>
								<tr>
									<th>Action</th>
									<th>Inline</th>
									<th>Delegate</th>
									<th aria-label="remove" />
								</tr>
							</thead>
							<tbody>
								{editRules.map((r, i) => (
									<tr key={i}>
										<td>
											<input
												className="input input-sm"
												value={r.action}
												onChange={(e) => {
													const next = [...editRules];
													next[i] = { ...r, action: e.target.value };
													setEditRules(next);
													setRulesDirty(true);
												}}
											/>
										</td>
										<td>
											<button
												className={`pill pill-sm ${r.inline === "Yes" ? "pill-success" : "pill-muted"}`}
												onClick={() => {
													const next = [...editRules];
													next[i] = { ...r, inline: r.inline === "Yes" ? "No" : "Yes" };
													setEditRules(next);
													setRulesDirty(true);
												}}
											>
												{r.inline}
											</button>
										</td>
										<td>
											<input
												className="input input-sm"
												value={r.delegate}
												onChange={(e) => {
													const next = [...editRules];
													next[i] = { ...r, delegate: e.target.value };
													setEditRules(next);
													setRulesDirty(true);
												}}
											/>
										</td>
										<td>
											<button
												className="btn btn-sm btn-ghost orch-deleg-remove"
												onClick={() => {
													setEditRules(editRules.filter((_, j) => j !== i));
													setRulesDirty(true);
												}}
											>
												<X size={12} />
											</button>
										</td>
									</tr>
								))}
							</tbody>
						</table>
						<button
							className="btn btn-sm btn-ghost"
							onClick={() => {
								setEditRules([...editRules, { action: "", inline: "No", delegate: "No" }]);
								setRulesDirty(true);
							}}
						>
							<Plus size={12} /> Add rule
						</button>

						<p className="field-label orch-triggers-head">Mandatory Delegation Triggers</p>
						<ul className="orch-triggers">
							{editTriggers.map((t, i) => (
								<li key={i} className="orch-trigger">
									<input
										className="input input-sm orch-trigger-name"
										placeholder="trigger name"
										value={t.name}
										onChange={(e) => {
											const next = [...editTriggers];
											next[i] = { ...t, name: e.target.value };
											setEditTriggers(next);
											setRulesDirty(true);
										}}
									/>
									<input
										className="input input-sm orch-trigger-desc"
										placeholder="description"
										value={t.description}
										onChange={(e) => {
											const next = [...editTriggers];
											next[i] = { ...t, description: e.target.value };
											setEditTriggers(next);
											setRulesDirty(true);
										}}
									/>
									<button
										className="btn btn-sm btn-ghost orch-deleg-remove"
										onClick={() => {
											setEditTriggers(editTriggers.filter((_, j) => j !== i));
											setRulesDirty(true);
										}}
									>
										<X size={12} />
									</button>
								</li>
							))}
						</ul>
						<button
							className="btn btn-sm btn-ghost"
							onClick={() => {
								setEditTriggers([...editTriggers, { name: "", description: "" }]);
								setRulesDirty(true);
							}}
						>
							<Plus size={12} /> Add trigger
						</button>

						<div className="orch-rules-actions">
							<button
								className="btn btn-sm btn-primary"
								onClick={handleSaveRules}
								disabled={!rulesDirty || savingRules}
							>
								<Save size={14} /> {savingRules ? "Saving…" : "Save rules"}
							</button>
						</div>
					</>
				)}
			</div>
		</div>
	);
}
