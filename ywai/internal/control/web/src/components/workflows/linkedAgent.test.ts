import { describe, it, expect, vi } from "vitest";
import { candidateAgentNames, fetchLinkedAgentContent } from "./linkedAgent";

vi.mock("../../api/client", () => ({
	configApi: {
		getAgent: vi.fn(async (name: string) => {
			if (name === "planning-planner-draft" || name === "orchestrator") {
				return { name, content: `# ${name}` };
			}
			throw new Error("agent not found");
		}),
	},
}));

import { configApi } from "../../api/client";

describe("candidateAgentNames", () => {
	it("maps group/name to the flattened installed name first, bare name second", () => {
		expect(candidateAgentNames("planning/planner-draft")).toEqual([
			"planning-planner-draft",
			"planner-draft",
		]);
	});
	it("keeps core refs resolvable via the bare fallback", () => {
		expect(candidateAgentNames("core/orchestrator")).toEqual(["core-orchestrator", "orchestrator"]);
	});
	it("passes bare names through untouched", () => {
		expect(candidateAgentNames("architect")).toEqual(["architect"]);
	});
	it("returns no candidates for an empty ref", () => {
		expect(candidateAgentNames("  ")).toEqual([]);
	});
});

describe("fetchLinkedAgentContent", () => {
	it("falls back to the bare name when the flattened one misses", async () => {
		vi.mocked(configApi.getAgent).mockClear();
		await expect(fetchLinkedAgentContent("core/orchestrator")).resolves.toBe("# orchestrator");
		expect(vi.mocked(configApi.getAgent)).toHaveBeenCalledWith("core-orchestrator");
		expect(vi.mocked(configApi.getAgent)).toHaveBeenCalledWith("orchestrator");
	});
	it("returns '' when nothing resolves", async () => {
		await expect(fetchLinkedAgentContent("planning/missing")).resolves.toBe("");
	});
	it("returns '' without any request for an empty ref", async () => {
		vi.mocked(configApi.getAgent).mockClear();
		await expect(fetchLinkedAgentContent("")).resolves.toBe("");
		expect(vi.mocked(configApi.getAgent)).not.toHaveBeenCalled();
	});
});
