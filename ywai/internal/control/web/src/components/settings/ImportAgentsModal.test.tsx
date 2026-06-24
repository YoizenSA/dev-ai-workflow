import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import ImportAgentsModal from "./ImportAgentsModal";

// Mock the API client; we only assert the preview (parse logic), not the
// network round-trip here.
vi.mock("../../api/client", () => ({
	configApi: {
		createAgent: vi.fn(),
		updateAgent: vi.fn(),
		updateAgentTaskPermissions: vi.fn(),
		updateAgentModel: vi.fn(),
	},
}));

describe("ImportAgentsModal — parse & preview", () => {
	beforeEach(() => cleanup());

	it("detects agents from a full opencode.json wrapper with a task map", () => {
		render(<ImportAgentsModal open onClose={() => {}} onDone={() => {}} />);

		const textarea = screen.getByPlaceholderText(
			/{[\s\S]*"agent"[\s\S]*}/,
		) as HTMLTextAreaElement;

		fireEvent.change(textarea, {
			target: {
				value: JSON.stringify({
					$schema: "https://opencode.ai/config.json",
					agent: {
						"gentle-orchestrator": {
							mode: "primary",
							model: "anthropic/claude-sonnet",
							permission: {
								task: { "*": "deny", dev: "allow", reviewer: "ask" },
							},
						},
						dev: { mode: "subagent" },
					},
				}),
			},
		});

		expect(screen.getByText("Detected 2 agent(s):")).toBeTruthy();
		expect(screen.getByText("gentle-orchestrator")).toBeTruthy();
		expect(screen.getByText("dev")).toBeTruthy();
		// The orchestrator should show delegation + model badges.
		expect(screen.getByText(/2 delegation/)).toBeTruthy();
		expect(screen.getAllByText("model").length).toBeGreaterThan(0);
	});

	it("shows an invalid-JSON error for malformed input", () => {
		render(<ImportAgentsModal open onClose={() => {}} onDone={() => {}} />);
		const textarea = screen.getByPlaceholderText(/{[\s\S]*/) as HTMLTextAreaElement;
		fireEvent.change(textarea, { target: { value: "{ not json" } });
		expect(screen.getByText(/Invalid JSON/)).toBeTruthy();
	});

	it("tolerates a bare agent object without the wrapper", () => {
		render(<ImportAgentsModal open onClose={() => {}} onDone={() => {}} />);
		const textarea = screen.getByPlaceholderText(/{[\s\S]*/) as HTMLTextAreaElement;
		fireEvent.change(textarea, {
			target: {
				value: JSON.stringify({
					dev: { mode: "subagent" },
					qa: { mode: "subagent" },
				}),
			},
		});
		expect(screen.getByText("Detected 2 agent(s):")).toBeTruthy();
	});
});
