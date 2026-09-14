import { describe, it, expect, vi } from "vitest";
import { filterAuthedModels } from "./NodeDetail";
import type { ModelInfo } from "../../api/types";

vi.mock("../../api/client", () => ({
	configApi: {},
	toolsApi: { listModels: vi.fn().mockResolvedValue({ modelsByProvider: {} }) },
	workflowApi: {},
}));

const models: ModelInfo[] = [
	{ id: "opencode-go/glm-5.3-flash", name: "glm-5.3-flash", provider: "opencode-go" },
	{ id: "meta/muse-spark-1.2", name: "muse-spark-1.2", provider: "meta" },
	{ id: "opencode-admin/mimo-v2.5-free", name: "mimo-v2.5-free", provider: "opencode-admin" },
];

describe("filterAuthedModels", () => {
	it("keeps only models whose provider has credentials", () => {
		const got = filterAuthedModels(models, ["opencode-go", "meta"]);
		expect(got.map((m) => m.provider)).toEqual(["opencode-go", "meta"]);
	});

	it("shows everything when the authed list is unknown (empty)", () => {
		expect(filterAuthedModels(models, [])).toBe(models);
	});

	it("shows nothing when no provider has credentials only if the list is known", () => {
		expect(filterAuthedModels(models, ["zai"])).toEqual([]);
	});
});
