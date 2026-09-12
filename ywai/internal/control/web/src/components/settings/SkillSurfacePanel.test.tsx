import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import SkillSurfacePanel from "./SkillSurfacePanel";

const surface = {
  projectDir: "D:/proj",
  locations: [
    { key: "global-opencode", label: "Global opencode", path: "C:/cfg/skills", found: true },
    { key: "project-opencode", label: "Project opencode", path: "D:/proj/.opencode/skills", found: false },
  ],
  skills: [
    {
      name: "solo",
      status: "unique",
      entries: [{ location: "global-opencode", path: "C:/cfg/skills/solo", hash: "abcdef123456" }],
    },
    {
      name: "dup",
      status: "shadowed",
      entries: [
        { location: "global-opencode", path: "C:/cfg/skills/dup", hash: "111111111111" },
        { location: "global-claude", path: "C:/cl/skills/dup", hash: "222222222222" },
      ],
    },
    {
      name: "dead",
      status: "unique",
      entries: [{ location: "global-opencode", path: "C:/cfg/skills/dead", hash: "333333333333", symlink: "D:/gone", broken: true }],
    },
  ],
};

function mockSurface() {
  globalThis.fetch = vi.fn(async () => ({
    ok: true,
    json: async () => surface,
  })) as unknown as typeof fetch;
}

beforeEach(() => {
  globalThis.fetch = vi.fn();
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("SkillSurfacePanel", () => {
  it("renders counts, shadow warning and per-location chips", async () => {
    mockSurface();
    render(<SkillSurfacePanel />);

    expect(await screen.findByText(/3 skill\(s\) visible/)).toBeInTheDocument();
    expect(screen.getByText(/Shadowed names.*dup/)).toBeInTheDocument();
    expect(screen.getByText("shadowed")).toBeInTheDocument();
    // Hash chips are shortened to 7 chars.
    expect(screen.getByText(/global-opencode · abcdef1/)).toBeInTheDocument();
    expect(screen.getByText(/broken link/)).toBeInTheDocument();
  });

  it("shows the backend error when the endpoint fails", async () => {
    globalThis.fetch = vi.fn(async () => ({
      ok: false,
      status: 500,
      statusText: "boom",
      text: async () => JSON.stringify({ error: "boom" }),
    })) as unknown as typeof fetch;
    render(<SkillSurfacePanel />);

    expect(await screen.findByText(/500/)).toBeInTheDocument();
  });

  it("deletes one location entry and reloads the surface", async () => {
    const calls: string[] = [];
    globalThis.fetch = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      calls.push(`${init?.method ?? "GET"} ${url}`);
      if ((init?.method ?? "GET") === "DELETE") {
        return { ok: true, json: async () => ({ status: "deleted" }) };
      }
      return { ok: true, json: async () => surface };
    }) as unknown as typeof fetch;
    globalThis.confirm = vi.fn(() => true);

    render(<SkillSurfacePanel />);

    const btn = await screen.findByTitle(/Delete dup at C:\/cl\/skills\/dup/);
    await btn.click();

    expect(calls.some((c) => c.startsWith("DELETE /api/config/skills/surface?path="))).toBe(true);
    // Surface reloads after delete: the list endpoint is hit twice.
    await waitFor(() => {
      expect(calls.filter((c) => c === "GET /api/config/skills/surface").length).toBe(2);
    });
  });

  it("filters by status and search", async () => {
    mockSurface();
    render(<SkillSurfacePanel />);
    await screen.findByText(/3 skill\(s\) visible/);

    fireEvent.click(screen.getByRole("radio", { name: /Shadowed/ }));
    expect(screen.getByText("dup")).toBeInTheDocument();
    expect(screen.queryByText("solo")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("radio", { name: /All/ }));
    fireEvent.change(screen.getByLabelText("Search skills"), { target: { value: "sol" } });
    expect(screen.getByText("solo")).toBeInTheDocument();
    expect(screen.queryByText("dead")).not.toBeInTheDocument();
  });

  it("previews standardize and applies only the checked paths", async () => {
    const posts: { url: string; body?: string }[] = [];
    const plan = {
      actions: [
        { kind: "remove-duplicate", name: "x", path: "C:/a/x", detail: "identical copy; kept C:/c/x" },
        { kind: "delete-empty-dir", name: "y", path: "C:/b/y", detail: "no readable SKILL.md" },
      ],
    };
    globalThis.fetch = vi.fn(async (input: string | URL | Request, init?: RequestInit) => {
      const url = String(input);
      if ((init?.method ?? "GET") === "POST") {
        posts.push({ url, body: init?.body as string | undefined });
        if (url.includes("dry_run=1")) return { ok: true, json: async () => plan };
        return { ok: true, json: async () => ({ deleted: [plan.actions[0]], failed: [] }) };
      }
      return { ok: true, json: async () => surface };
    }) as unknown as typeof fetch;

    render(<SkillSurfacePanel />);
    fireEvent.click(await screen.findByRole("button", { name: /Standardize/ }));
    expect(await screen.findByText("Duplicate copies")).toBeInTheDocument();
    expect(posts[0].url).toContain("dry_run=1");
    expect(posts[0].url).toContain("dedupe=1");

    // Uncheck the empty-folder row, then apply.
    const rows = screen.getAllByRole("checkbox").filter((c) => c.closest(".ss-plan-row"));
    fireEvent.click(rows[1]);
    fireEvent.click(screen.getByRole("button", { name: /Apply 1 change/ }));

    await waitFor(() => expect(posts.length).toBe(2));
    expect(posts[1].url).toContain("dry_run=0");
    expect(JSON.parse(posts[1].body ?? "{}")).toEqual({ paths: ["C:/a/x"] });
    expect(await screen.findByText(/removed 1 item/)).toBeInTheDocument();
  });
});
