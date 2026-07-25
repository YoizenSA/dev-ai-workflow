import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, cleanup } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import Settings from "./Settings";

const deferred = <T,>() => {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((r) => {
		resolve = r;
	});
	return { promise, resolve };
};

const listModelsDeferred = deferred<{
	modelsByProvider: Record<string, Array<{ id: string; name: string; provider: string }>>;
}>();

vi.mock("../../api/client", () => ({
	configApi: {
		getConfig: vi.fn().mockResolvedValue({
			model: "opencode/gpt-4.1",
			small_model: "opencode/gpt-4.1-mini",
			default_agent: "orchestrator",
			maxTokens: 8192,
			temperature: 0.7,
		}),
		listAgents: vi.fn().mockResolvedValue([{ name: "orchestrator" }, { name: "dev" }]),
		getUserConfig: vi.fn().mockResolvedValue({ vision_model: "" }),
		listVisionModels: vi.fn().mockResolvedValue({ models: [], current: "" }),
		getAgentsMd: vi.fn().mockResolvedValue({ path: "", content: "" }),
		getSddStatus: vi.fn().mockResolvedValue({ agents: [], total: 0 }),
		listTools: vi.fn().mockResolvedValue({ built_in: [], mcp_tools: {}, plugin_tools: {} }),
		listSkills: vi.fn().mockResolvedValue([]),
		listMCP: vi.fn().mockResolvedValue([]),
		listProviders: vi.fn().mockResolvedValue([]),
		updateConfig: vi.fn(),
		updateUserConfig: vi.fn(),
	},
	missionsApi: {
		// Intentionally never resolves until the test releases it — proves the
		// General form does not wait on the slow opencode models CLI.
		listModels: vi.fn(() => listModelsDeferred.promise),
	},
}));

describe("Settings General tab load", () => {
	beforeEach(() => {
		cleanup();
	});

	afterEach(() => {
		cleanup();
	});

	it("renders the General form before listModels resolves", async () => {
		render(
			<MemoryRouter initialEntries={["/settings"]}>
				<Settings />
			</MemoryRouter>,
		);

		// Form must appear without waiting for the models catalog.
		await waitFor(() => {
			expect(screen.getByLabelText(/^Model$/i)).toBeTruthy();
		});
		expect(screen.getByRole("button", { name: /Save Changes/i })).toBeTruthy();
		// Saved model id is visible even while the catalog is still loading.
		expect(screen.getByDisplayValue("opencode/gpt-4.1")).toBeTruthy();
	});
});
