import { describe, it, expect, beforeEach, vi } from "vitest";
import { useAgentsDiagramStore } from "./agentsDiagramStore";
import type { AgentGraph } from "../api/types";

// Mock the API client so tests don't hit the network.
vi.mock("../api/client", () => ({
	configApi: {
		getAgentGraph: vi.fn(),
		updateAgentTaskPermissions: vi.fn(),
		updateAgentModel: vi.fn(),
	},
}));

import { configApi } from "../api/client";

const mockConfigApi = configApi as unknown as {
	getAgentGraph: ReturnType<typeof vi.fn>;
	updateAgentTaskPermissions: ReturnType<typeof vi.fn>;
	updateAgentModel: ReturnType<typeof vi.fn>;
};

const GRAPH: AgentGraph = {
	nodes: [
		{ id: "orch", name: "orch", mode: "primary", hasWildcard: true, wildcardValue: "deny" },
		{ id: "dev", name: "dev", mode: "subagent" },
		{ id: "reviewer", name: "reviewer", mode: "subagent" },
	],
	edges: [
		{ id: "orch->dev", source: "orch", target: "dev", value: "allow" },
		{ id: "orch->reviewer", source: "orch", target: "reviewer", value: "ask" },
	],
};

beforeEach(() => {
	vi.clearAllMocks();
	useAgentsDiagramStore.setState({
		graph: { nodes: [], edges: [] },
		selected: null,
		loading: false,
		error: null,
	});
});

describe("useAgentsDiagramStore.load", () => {
	it("loads the graph into state", async () => {
		mockConfigApi.getAgentGraph.mockResolvedValue(GRAPH);
		await useAgentsDiagramStore.getState().load();
		expect(useAgentsDiagramStore.getState().graph).toEqual(GRAPH);
		expect(useAgentsDiagramStore.getState().loading).toBe(false);
	});

	it("stores an error message on failure", async () => {
		mockConfigApi.getAgentGraph.mockRejectedValue(new Error("boom"));
		await useAgentsDiagramStore.getState().load();
		expect(useAgentsDiagramStore.getState().error).toBe("boom");
		expect(useAgentsDiagramStore.getState().graph.nodes).toHaveLength(0);
	});
});

describe("useAgentsDiagramStore.setEdge", () => {
	it("optimistically adds an edge and PUTs the full task map (preserving wildcard)", async () => {
		useAgentsDiagramStore.setState({ graph: GRAPH });
		mockConfigApi.updateAgentTaskPermissions.mockResolvedValue(undefined);

		await useAgentsDiagramStore.getState().setEdge("orch", "qa", "allow");

		// Wildcard "*" must be preserved in the PUT body.
		expect(mockConfigApi.updateAgentTaskPermissions).toHaveBeenCalledWith("orch", {
			"*": "deny",
			dev: "allow",
			reviewer: "ask",
			qa: "allow",
		});
		// Optimistic local update.
		const { graph } = useAgentsDiagramStore.getState();
		expect(graph.edges).toContainEqual({
			id: "orch->qa",
			source: "orch",
			target: "qa",
			value: "allow",
		});
		// Ghost node created for the new target.
		expect(graph.nodes.some((n) => n.id === "qa" && n.ghost)).toBe(true);
	});

	it("rolls back on API failure", async () => {
		useAgentsDiagramStore.setState({ graph: GRAPH });
		mockConfigApi.updateAgentTaskPermissions.mockRejectedValue(new Error("nope"));

		await expect(
			useAgentsDiagramStore.getState().setEdge("orch", "qa", "allow"),
		).rejects.toThrow("nope");

		// Graph restored to the pre-mutation snapshot.
		expect(useAgentsDiagramStore.getState().graph).toEqual(GRAPH);
		expect(useAgentsDiagramStore.getState().error).toBe("nope");
	});
});

describe("useAgentsDiagramStore.cycleEdge", () => {
	it("cycles allow -> ask -> deny (removing the edge)", async () => {
		useAgentsDiagramStore.setState({ graph: GRAPH });
		mockConfigApi.updateAgentTaskPermissions.mockResolvedValue(undefined);

		// allow -> ask
		await useAgentsDiagramStore.getState().cycleEdge("orch", "dev");
		expect(
			useAgentsDiagramStore.getState().graph.edges.find((e) => e.id === "orch->dev")?.value,
		).toBe("ask");

		// ask -> deny (edge removed from the graph view)
		await useAgentsDiagramStore.getState().cycleEdge("orch", "dev");
		expect(
			useAgentsDiagramStore.getState().graph.edges.find((e) => e.id === "orch->dev"),
		).toBeUndefined();
	});

	it("creates an edge from deny -> allow when none exists", async () => {
		useAgentsDiagramStore.setState({ graph: GRAPH });
		mockConfigApi.updateAgentTaskPermissions.mockResolvedValue(undefined);

		await useAgentsDiagramStore.getState().cycleEdge("orch", "qa");
		expect(
			useAgentsDiagramStore.getState().graph.edges.find((e) => e.id === "orch->qa")?.value,
		).toBe("allow");
	});
});

describe("useAgentsDiagramStore.setAgentModel", () => {
	it("optimistically updates the node model", async () => {
		useAgentsDiagramStore.setState({ graph: GRAPH });
		mockConfigApi.updateAgentModel.mockResolvedValue(undefined);

		await useAgentsDiagramStore.getState().setAgentModel("dev", "opencode-go/glm-5.1");
		expect(
			useAgentsDiagramStore.getState().graph.nodes.find((n) => n.id === "dev")?.model,
		).toBe("opencode-go/glm-5.1");
		expect(mockConfigApi.updateAgentModel).toHaveBeenCalledWith("dev", "opencode-go/glm-5.1");
	});
});
