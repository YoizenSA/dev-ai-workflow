import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import NodeFocusModal from "./NodeFocusModal";
import { useWorkflowStore } from "../../stores/workflowStore";
import type { Workflow } from "../../api/types";

vi.mock("@monaco-editor/react", () => ({
	default: ({ value }: { value?: string }) => <textarea data-testid="monaco" value={value ?? ""} readOnly />,
}));

// The installed agent is flattened (planning-planner-draft.md): the bare
// basename 404s, like the real backend.
const getAgent = vi.fn(async (name: string) => {
	if (name === "planning-planner-draft") {
		return { name, content: "# Planner\nYou plan the whole thing." };
	}
	throw new Error("agent not found");
});

vi.mock("../../api/client", () => ({
	configApi: { getAgent: (name: string) => getAgent(name) },
	workflowApi: { listSections: vi.fn().mockResolvedValue([]) },
}));

function seedAgentNode(data: Record<string, unknown>) {
	const wf = {
		id: "wf-1",
		name: "goal",
		version: "1",
		nodes: [{ id: "n1", type: "subAgent", name: "planner", position: { x: 0, y: 0 }, data }],
		connections: [],
	} as unknown as Workflow;
	useWorkflowStore.setState({ current: wf, selectedNodeId: "n1", focusNodeId: "n1" });
}

const focusedNode = () => useWorkflowStore.getState().current?.nodes[0];

beforeEach(() => {
	getAgent.mockClear();
	useWorkflowStore.setState({ current: null, selectedNodeId: null, focusNodeId: null });
});
afterEach(cleanup);

describe("NodeFocusModal linked identity", () => {
	it("shows the full resolved system prompt read-only while linked", async () => {
		seedAgentNode({ description: "plans", agentRef: "planning/planner-draft", prompt: "do it" });
		render(<NodeFocusModal nodeId="n1" onClose={() => undefined} />);

		// The flattened installed name is tried before the bare basename.
		await waitFor(() => {
			expect(getAgent).toHaveBeenCalledWith("planning-planner-draft");
		});
		expect(await screen.findByDisplayValue(/You plan the whole thing/)).toBeInTheDocument();
		expect(screen.getByText(/Resolved from/)).toBeInTheDocument();
		// Still read-only: no editable system-prompt box while linked.
		expect(screen.getByRole("button", { name: /detach/i })).toBeInTheDocument();
	});

	it("detaching keeps the resolved text as an editable override", async () => {
		const user = userEvent.setup();
		seedAgentNode({ description: "plans", agentRef: "planning/planner-draft", prompt: "do it" });
		render(<NodeFocusModal nodeId="n1" onClose={() => undefined} />);

		await user.click(await screen.findByRole("button", { name: /detach/i }));

		await waitFor(() => {
			expect(focusedNode()?.data.agentRef).toBe("");
		});
		expect(focusedNode()?.data.agentDefinition).toContain("You plan the whole thing.");
	});

	it("shows the editor when the node has its own prompt", async () => {
		seedAgentNode({ description: "plans", agentDefinition: "hand-written", prompt: "do it" });
		render(<NodeFocusModal nodeId="n1" onClose={() => undefined} />);

		expect(await screen.findByDisplayValue("hand-written")).toBeInTheDocument();
		expect(screen.queryByText(/Resolved from/)).not.toBeInTheDocument();
		expect(getAgent).not.toHaveBeenCalled();
	});
});
