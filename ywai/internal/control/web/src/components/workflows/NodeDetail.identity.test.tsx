import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import NodeDetail from "./NodeDetail";
import { useWorkflowStore } from "../../stores/workflowStore";
import type { Workflow } from "../../api/types";

vi.mock("../../api/client", () => ({
	configApi: {
		listAgents: vi.fn().mockResolvedValue([
			{ name: "orchestrator", group: "core" },
			{ name: "dev", group: "core" },
		]),
		getAgent: vi.fn().mockResolvedValue({ name: "orchestrator", content: "# Orchestrator\nYou own the goal." }),
	},
	missionsApi: { listModels: vi.fn().mockResolvedValue({ modelsByProvider: {} }) },
	workflowApi: {
		listSkills: vi.fn().mockResolvedValue([]),
		list: vi.fn().mockResolvedValue([]),
		listSections: vi.fn().mockResolvedValue([]),
		listMcpCatalog: vi.fn().mockResolvedValue([]),
		listMcpServers: vi.fn().mockResolvedValue([]),
	},
}));

function seedWorkflow(startData: Record<string, unknown>) {
	const wf = {
		id: "wf-1",
		name: "goal",
		version: "1",
		nodes: [{ id: "start", type: "start", name: "start", position: { x: 0, y: 0 }, data: startData }],
		connections: [],
	} as unknown as Workflow;
	useWorkflowStore.setState({ current: wf, selectedNodeId: "start" });
	return wf;
}

const startNode = () => useWorkflowStore.getState().current?.nodes[0];

beforeEach(() => {
	useWorkflowStore.setState({ current: null, selectedNodeId: null });
});
afterEach(cleanup);

describe("START node identity", () => {
	it("shows the linked agent instead of an empty prompt box", async () => {
		seedWorkflow({ label: "Feature request", agentRef: "core/orchestrator" });
		render(<NodeDetail />);

		expect(await screen.findByText("core/orchestrator")).toBeInTheDocument();
		expect(screen.getByText("linked")).toBeInTheDocument();
		// The raw textarea must NOT be offered while linked: typing in it would
		// set agentDefinition, which silently overrides agentRef on export.
		expect(screen.queryByLabelText("System prompt / identity")).not.toBeInTheDocument();
	});

	it("linking an agent clears the inline prompt that would override it", async () => {
		const user = userEvent.setup();
		seedWorkflow({ label: "Feature request", agentDefinition: "stale hand-written copy" });
		render(<NodeDetail />);

		await user.click(await screen.findByLabelText("Link to an existing agent"));
		await user.click(await screen.findByRole("button", { name: "core/orchestrator" }));

		await waitFor(() => {
			expect(startNode()?.data.agentRef).toBe("core/orchestrator");
		});
		expect(startNode()?.data.agentDefinition ?? "").toBe("");
	});

	it("detaching keeps the resolved text as an editable override", async () => {
		const user = userEvent.setup();
		seedWorkflow({ label: "Feature request", agentRef: "core/orchestrator" });
		render(<NodeDetail />);

		await user.click(await screen.findByRole("button", { name: /detach/i }));

		await waitFor(() => {
			expect(startNode()?.data.agentRef).toBe("");
		});
		expect(startNode()?.data.agentDefinition).toContain("You own the goal.");
	});
});
