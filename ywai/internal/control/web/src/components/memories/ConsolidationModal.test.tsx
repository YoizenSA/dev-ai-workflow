import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/react";
import ConsolidationModal from "./ConsolidationModal";
import { useMemoriesStore } from "../../stores/memoriesStore";

// Regression test for React #185 ("Maximum update depth exceeded") on
// /memories: `useMemoriesStore((s) => s.stats?.projects ?? [])` returned a NEW
// [] on every snapshot while stats is null, which React 19's
// useSyncExternalStore rejects as an uncached getSnapshot result and loops
// forever. The selector must point at a stable ref; derive arrays outside.
vi.mock("../../api/client", () => ({
	memoriesApi: {
		listModels: vi.fn().mockResolvedValue({ modelsByProvider: {} }),
		listAgents: vi.fn().mockResolvedValue({ agents: ["memory"] }),
	},
	configApi: {},
	missionsApi: {
		listModels: vi.fn().mockResolvedValue({ modelsByProvider: {} }),
		listAgents: vi.fn().mockResolvedValue({ agents: ["memory"] }),
	},
}));

describe("ConsolidationModal", () => {
	beforeEach(() => {
		cleanup();
		// Fresh store state: stats === null is the crash context.
		useMemoriesStore.setState({ stats: null, consolidation: null });
	});

	afterEach(() => {
		cleanup();
	});

	it("renders without an infinite update loop when stats is null", () => {
		let threw = false;
		try {
			render(<ConsolidationModal open={false} onClose={() => {}} />);
		} catch {
			threw = true;
		}
		expect(threw).toBe(false);
	});

	it("renders with stats set (projects present)", () => {
		useMemoriesStore.setState({
			stats: {
				total_sessions: 1,
				total_observations: 1,
				total_prompts: 1,
				projects: ["dev-ai-workflow"],
			},
		});
		let threw = false;
		try {
			render(<ConsolidationModal open={false} onClose={() => {}} />);
		} catch {
			threw = true;
		}
		expect(threw).toBe(false);
	});
});
