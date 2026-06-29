import { describe, it, expect, beforeEach, vi } from "vitest";
import { useWorkflowStore, disconnectEdgeId } from "./workflowStore";
import type { Workflow } from "../api/types";

// Mock the API client so tests don't hit the network.
vi.mock("../api/client", () => ({
	workflowApi: {
		list: vi.fn(),
		get: vi.fn(),
		create: vi.fn(),
		save: vi.fn(),
		delete: vi.fn(),
		import: vi.fn(),
		validate: vi.fn(),
		export: vi.fn(),
	},
}));

import { workflowApi } from "../api/client";

const mockApi = workflowApi as unknown as {
	list: ReturnType<typeof vi.fn>;
	get: ReturnType<typeof vi.fn>;
	create: ReturnType<typeof vi.fn>;
	save: ReturnType<typeof vi.fn>;
	delete: ReturnType<typeof vi.fn>;
	import: ReturnType<typeof vi.fn>;
	validate: ReturnType<typeof vi.fn>;
	export: ReturnType<typeof vi.fn>;
};

const WORKFLOW: Workflow = {
	id: "w",
	name: "w",
	description: "test",
	version: "1.0.0",
	nodes: [
		{ id: "s", type: "start", name: "s", position: { x: 0, y: 0 }, data: { label: "Start" } },
		{ id: "a", type: "subAgent", name: "a", position: { x: 100, y: 0 }, data: { description: "does a" } },
		{ id: "e", type: "end", name: "e", position: { x: 200, y: 0 }, data: { label: "End" } },
	],
	connections: [
		{ from: "s", to: "a", fromPort: "out", toPort: "input" },
		{ from: "a", to: "e", fromPort: "out", toPort: "in" },
	],
	createdAt: "2025-01-01T00:00:00Z",
	updatedAt: "2025-01-01T00:00:00Z",
};

beforeEach(() => {
	vi.clearAllMocks();
	useWorkflowStore.setState({
		summaries: [],
		loadingList: false,
		current: null,
		loading: false,
		dirty: false,
		error: null,
		validation: null,
		exportPlan: null,
		exporting: false,
		selectedNodeId: null,
	});
});

describe("list", () => {
	it("loads summaries into state", async () => {
		mockApi.list.mockResolvedValue({ workflows: [{ name: "w", description: "", version: "1.0.0", nodeCount: 3, updatedAt: "" }] });
		await useWorkflowStore.getState().list();
		expect(useWorkflowStore.getState().summaries).toHaveLength(1);
		expect(useWorkflowStore.getState().loadingList).toBe(false);
	});

	it("stores an error on failure", async () => {
		mockApi.list.mockRejectedValue(new Error("boom"));
		await useWorkflowStore.getState().list();
		expect(useWorkflowStore.getState().error).toBe("boom");
	});
});

describe("load", () => {
	it("loads a workflow into current", async () => {
		mockApi.get.mockResolvedValue(WORKFLOW);
		await useWorkflowStore.getState().load("w");
		expect(useWorkflowStore.getState().current?.name).toBe("w");
		expect(useWorkflowStore.getState().loading).toBe(false);
	});

	it("surfaces load errors", async () => {
		mockApi.get.mockRejectedValue(new Error("nope"));
		await useWorkflowStore.getState().load("missing");
		expect(useWorkflowStore.getState().current).toBeNull();
		expect(useWorkflowStore.getState().error).toBe("nope");
	});
});

describe("graph editing (optimistic)", () => {
	it("addNode appends a node and marks dirty", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		const id = useWorkflowStore.getState().addNode("prompt", 50, 50);
		const nodes = useWorkflowStore.getState().current!.nodes;
		expect(nodes.some((n) => n.id === id)).toBe(true);
		expect(useWorkflowStore.getState().dirty).toBe(true);
	});

	it("updateNode patches node data", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		useWorkflowStore.getState().updateNode("a", { description: "updated" });
		const node = useWorkflowStore.getState().current!.nodes.find((n) => n.id === "a");
		expect(node?.data.description).toBe("updated");
	});

	it("removeNode drops the node and its connections", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		useWorkflowStore.getState().removeNode("a");
		const state = useWorkflowStore.getState().current!;
		expect(state.nodes.some((n) => n.id === "a")).toBe(false);
		// Connections referencing 'a' on either side are gone.
		expect(state.connections.some((c) => c.from === "a" || c.to === "a")).toBe(false);
	});

	it("connect adds an edge", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		useWorkflowStore.getState().connect({ from: "s", to: "e", fromPort: "out", toPort: "in" });
		const conns = useWorkflowStore.getState().current!.connections;
		expect(conns.some((c) => c.from === "s" && c.to === "e")).toBe(true);
	});

	it("connect ignores duplicate edges", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		const before = useWorkflowStore.getState().current!.connections.length;
		// Same edge as an existing one (s->a).
		useWorkflowStore.getState().connect({ from: "s", to: "a", fromPort: "out", toPort: "input" });
		expect(useWorkflowStore.getState().current!.connections.length).toBe(before);
	});

	it("disconnect removes an edge by its id", () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		const edge = WORKFLOW.connections[0];
		const id = disconnectEdgeId(edge);
		useWorkflowStore.getState().disconnect(id);
		const conns = useWorkflowStore.getState().current!.connections;
		expect(conns.some((c) => disconnectEdgeId(c) === id)).toBe(false);
	});
});

describe("saveCurrent rollback", () => {
	it("rolls back to the previous workflow when save fails", async () => {
		useWorkflowStore.setState({ current: WORKFLOW, dirty: true });
		mockApi.save.mockRejectedValue(new Error("disk full"));

		await useWorkflowStore.getState().saveCurrent();

		// On failure the store restores the snapshot and re-asserts dirty.
		expect(useWorkflowStore.getState().current).toEqual(WORKFLOW);
		expect(useWorkflowStore.getState().dirty).toBe(true);
		expect(useWorkflowStore.getState().error).toBe("disk full");
	});
});

describe("exportCurrent", () => {
	it("stores the export plan (dry-run by default)", async () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		mockApi.export.mockResolvedValue({ workflowName: "w", files: [], dryRun: true });
		await useWorkflowStore.getState().exportCurrent(false);
		expect(useWorkflowStore.getState().exportPlan?.dryRun).toBe(true);
	});

	it("passes apply=true through to the API", async () => {
		useWorkflowStore.setState({ current: WORKFLOW });
		mockApi.export.mockResolvedValue({ workflowName: "w", files: [], dryRun: false });
		await useWorkflowStore.getState().exportCurrent(true);
		expect(mockApi.export).toHaveBeenCalledWith("w", true);
	});
});
