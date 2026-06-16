# Create Folder in New-Mission Modal — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the user create a new subfolder from inside the file browser of the New Mission modal, then automatically register it as a project and select it for the mission.

**Architecture:** A new read-only-parallel `POST /api/fs/mkdir` endpoint creates a single directory segment with hardened name validation. The React file browser gains a "New folder" input; on success it reuses the existing "register project + select" flow, factored into a helper that takes an explicit path.

**Tech Stack:** Go (stdlib `net/http`, `os`, `path/filepath`, `strings`) · React 18 + TypeScript (Vite) · existing `missionsApi` fetch wrapper.

**Spec:** `docs/design/create-folder-in-mission.md`

---

## File Structure

**Backend (Go):**
- **Modify** `internal/missions/web/server.go` — register the `POST /api/fs/mkdir` route next to `BrowseFS`.
- **Modify** `internal/missions/web/handlers.go` — add `MkdirFS` method on `*Handlers`. No new imports needed beyond `path/filepath` (confirm in Task 1).
- **Create** `internal/missions/web/handlers_mkdir_test.go` — table-driven handler test using `httptest`.

**Frontend (TS/React):**
- **Modify** `internal/control/web/src/api/client.ts` — add `createFolder` to `missionsApi`.
- **Modify** `internal/control/web/src/components/missions/CreateMissionModal.tsx` — refactor `handleSelectFolder` into a path-parameterized helper, add `handleCreateFolder`, add wizard state fields, render the "New folder" UI row inside `fs-browser-actions`.

**No new files other than the test.** No data-model changes. No new dependencies.

---

## Task 1: Backend — add `MkdirFS` handler + route

**Files:**
- Modify: `internal/missions/web/handlers.go` (append new handler after `BrowseFS`, ~line 784)
- Modify: `internal/missions/web/server.go` (register route at ~line 91, next to `BrowseFS`)

- [ ] **Step 1: Write the failing test**

Create `internal/missions/web/handlers_mkdir_test.go`:

```go
package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMkdirFS(t *testing.T) {
	parent := t.TempDir()

	cases := []struct {
		name       string
		body       string
		wantStatus int
		// for success cases: the folder that should exist on disk afterwards
		wantDir string
		// substring expected in the JSON response body ("" = skip check)
		wantBody string
		// precondition: create this folder before the test runs (relative to parent)
		preCreate string
	}{
		{
			name:       "valid single segment",
			body:       `{"parentPath":"` + parent + `","name":"newproj"}`,
			wantStatus: http.StatusCreated,
			wantDir:    filepath.Join(parent, "newproj"),
			wantBody:   `"path":"` + filepath.Join(parent, "newproj") + `"`,
		},
		{
			name:       "empty name",
			body:       `{"parentPath":"` + parent + `","name":"  "}`,
			wantStatus: http.StatusBadRequest,
			wantBody:   "folder name is required",
		},
		{
			name:       "name with path separator is rejected",
			body:       `{"parentPath":"` + parent + `","name":"a/b"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "dotdot as name rejected",
			body:       `{"parentPath":"` + parent + `","name":".."}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "already exists",
			body:       `{"parentPath":"` + parent + `","name":"exists"}`,
			wantStatus: http.StatusConflict,
			wantBody:   "already exists",
			preCreate:  "exists",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.preCreate != "" {
				if err := os.Mkdir(filepath.Join(parent, tc.preCreate), 0755); err != nil {
					t.Fatalf("precreate: %v", err)
				}
			}

			h := &Handlers{}
			req := httptest.NewRequest(http.MethodPost, "/api/fs/mkdir", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			h.MkdirFS(rec, req)

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			if tc.wantBody != "" && !strings.Contains(rec.Body.String(), tc.wantBody) {
				t.Fatalf("body %q does not contain %q", rec.Body.String(), tc.wantBody)
			}
			if tc.wantDir != "" {
				info, err := os.Stat(tc.wantDir)
				if err != nil {
					t.Fatalf("expected dir %s to exist: %v", tc.wantDir, err)
				}
				if !info.IsDir() {
					t.Fatalf("%s is not a directory", tc.wantDir)
				}
			}
		})
	}
}

// TestMkdirFS_DefaultParent exercises the empty-parentPath branch (defaults to home).
// Kept separate because it depends on os.UserHomeDir().
func TestMkdirFS_DefaultParent(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("cannot resolve home dir: %v", err)
	}
	// Use a unique name to avoid colliding with real user data.
	dirName := "ywai-mkdir-test-" + filepath.Base(t.Name())
	defer os.RemoveAll(filepath.Join(home, dirName))

	h := &Handlers{}
	body := `{"name":"` + dirName + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/fs/mkdir", strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.MkdirFS(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var resp struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Path != filepath.Join(home, dirName) {
		t.Fatalf("path = %q, want %q", resp.Path, filepath.Join(home, dirName))
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai && go test ./internal/missions/web/... -run TestMkdirFS -v
```
Expected: FAIL — `h.MkdirFS undefined` (compile error).

- [ ] **Step 3: Add the `MkdirFS` handler to `internal/missions/web/handlers.go`**

First, ensure the `path/filepath` import is present. The current imports (verified) are `net/http`, `os`, `strings`. Add `"path/filepath"` to the import block. The full import block currently starts at line 3; insert `path/filepath` in alphabetical order between `os` and `strings`.

Then, append this handler right after `BrowseFS` (after its closing brace at line 784):

```go
// MkdirFS creates a single new directory inside parentPath. It is the write-side
// counterpart to BrowseFS. The name is sanitized to a single path segment: nested
// paths and traversal attempts are rejected.
func (h *Handlers) MkdirFS(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentPath string `json:"parentPath"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	// Sanitize the name to a single path segment.
	name := strings.TrimSpace(filepath.Base(req.Name))
	if name == "" || name == "." || name == ".." {
		writeError(w, http.StatusBadRequest, "folder name is required")
		return
	}
	// Reject control characters / null bytes.
	if strings.ContainsAny(name, "\x00\r\n") {
		writeError(w, http.StatusBadRequest, "folder name contains invalid characters")
		return
	}

	parent := req.ParentPath
	if strings.TrimSpace(parent) == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusBadRequest, "parentPath is required and home dir unavailable")
			return
		}
		parent = home
	}
	parent = filepath.Clean(parent)

	full := filepath.Join(parent, name)
	// Second line of defense: the joined result must sit directly under parent.
	if filepath.Dir(full) != parent {
		writeError(w, http.StatusBadRequest, "invalid folder name")
		return
	}

	if err := os.Mkdir(full, 0755); err != nil {
		if os.IsExist(err) {
			writeError(w, http.StatusConflict, "folder already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("cannot create folder: %v", err))
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"path": full,
	})
}
```

> `fmt` is already imported (used elsewhere in the file, e.g. `BrowseFS`). `json`, `os`, `strings` are already imported. Only `path/filepath` is new.

- [ ] **Step 4: Register the route in `internal/missions/web/server.go`**

In the route-registration block, the `BrowseFS` line is currently:
```go
	mux.HandleFunc("GET /api/fs/browse", h.BrowseFS)
```
Add immediately after it:
```go
	mux.HandleFunc("POST /api/fs/mkdir", h.MkdirFS)
```

- [ ] **Step 5: Run the test to verify it passes**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai && go test ./internal/missions/web/... -run TestMkdirFS -v
```
Expected: PASS — all subtests green.

- [ ] **Step 6: Run the full web package tests to confirm no regressions**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai && go test ./internal/missions/web/... -v
```
Expected: PASS (all existing tests still green).

- [ ] **Step 7: Commit**

```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow
git add ywai/internal/missions/web/handlers.go ywai/internal/missions/web/server.go ywai/internal/missions/web/handlers_mkdir_test.go
git commit -m "feat(missions): add POST /api/fs/mkdir endpoint

Add a write-side counterpart to BrowseFS that creates a single new
directory inside a parent path. The folder name is sanitized with
filepath.Base and a prefix check, rejecting nested paths and traversal
attempts. os.Mkdir (not MkdirAll) ensures pre-existing folders error
with 409 for clear user feedback."
```

---

## Task 2: Frontend — add `createFolder` to the API client

**Files:**
- Modify: `internal/control/web/src/api/client.ts` (the `missionsApi` object, after `browseFS` at ~line 187)

- [ ] **Step 1: Add `createFolder` to `missionsApi`**

In `internal/control/web/src/api/client.ts`, locate the existing `browseFS` entry inside `missionsApi`:

```ts
  // File system browser
  browseFS: (path?: string) =>
    request<BrowseFSResponse>(
      `/missions/api/fs/browse${path ? `?path=${encodeURIComponent(path)}` : ''}`,
    ),
```

Add a `createFolder` entry immediately after it (inside the same `missionsApi` object, before the closing `}`):

```ts
  createFolder: (parentPath: string, name: string) =>
    request<{ path: string }>('/missions/api/fs/mkdir', {
      method: 'POST',
      body: JSON.stringify({ parentPath, name }),
    }),
```

> This mirrors the existing `createProject` style (same `request<T>` wrapper, same `/missions` proxy prefix). The returned `path` is the absolute path of the newly-created folder.

- [ ] **Step 2: Type-check the change**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai/internal/control/web && npx tsc --noEmit
```
Expected: no errors (the new function matches the `request<T>` signature; no new types needed).

- [ ] **Step 3: Commit**

```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow
git add ywai/internal/control/web/src/api/client.ts
git commit -m "feat(ui): add missionsApi.createFolder client method

Wraps POST /missions/api/fs/mkdir, returning the absolute path of the
created folder. Follows the existing request<T> + /missions proxy
convention used by browseFS and createProject."
```

---

## Task 3: Frontend — refactor `handleSelectFolder` into a path-parameterized helper

**Files:**
- Modify: `internal/control/web/src/components/missions/CreateMissionModal.tsx` (the `handleSelectFolder` function, lines 225–266)

This task is a pure refactor — no behavior change — so that Task 4 can reuse the "register project + select" flow for a freshly-created folder without duplicating the ~40-line body.

- [ ] **Step 1: Extract `handleSelectFolderForPath`**

Replace the existing `handleSelectFolder` (lines 225–266) with a parameterized helper plus a thin wrapper that calls it with the current browse path. The exact replacement:

```ts
	const handleSelectFolderForPath = async (folderPath: string) => {
		const folderName = folderPath.split("/").filter(Boolean).pop() ?? folderPath;
		try {
			const project = await missionsApi.createProject(folderName, folderPath);
			update({
				projects: [...state.projects, project],
				selectedProject: project,
				browseMode: false,
				baseBranch: project.branch ?? "main",
			});
		} catch (err) {
			// If project already exists (409), find it in the list and select it
			const existing = state.projects.find((p) => p.name === folderName || p.path === folderPath);
			if (existing) {
				update({
					selectedProject: existing,
					browseMode: false,
					baseBranch: existing.branch ?? "main",
				});
				return;
			}
			// For other errors, try to refresh the projects list and find it
			try {
				const projects = await missionsApi.listProjects();
				const found = projects.find((p) => p.name === folderName || p.path === folderPath);
				if (found) {
					update({
						projects,
						selectedProject: found,
						browseMode: false,
						baseBranch: found.branch ?? "main",
					});
					return;
				}
			} catch {
				// ignore — fall through to error display
			}
			// If all else fails, show the error
			update({ browseError: String(err) });
		}
	};

	const handleSelectFolder = () => handleSelectFolderForPath(state.browsePath);
```

> Only change vs. original: the body moved into `handleSelectFolderForPath(folderPath)`, the local `const folderPath = state.browsePath` and `folderName` derivation now take the parameter, and `handleSelectFolder` is now a zero-arg wrapper. The 409 fallback logic is byte-for-byte identical. `handleSelectFolder` is still referenced by the existing "Select This Folder" button.

- [ ] **Step 2: Type-check**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai/internal/control/web && npx tsc --noEmit
```
Expected: no errors.

- [ ] **Step 3: Manual smoke check (build still compiles)**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai/internal/control/web && npm run build
```
Expected: build succeeds (this is a no-op refactor; behavior is unchanged).

- [ ] **Step 4: Commit**

```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow
git add ywai/internal/control/web/src/components/missions/CreateMissionModal.tsx
git commit -m "refactor(ui): extract handleSelectFolderForPath helper

Factor the register-project-and-select body of handleSelectFolder into
a helper that takes an explicit folderPath. No behavior change; the
original handler is now a thin wrapper. Sets up reuse for folder
creation in the next commit."
```

---

## Task 4: Frontend — add "New folder" UI + handler

**Files:**
- Modify: `internal/control/web/src/components/missions/CreateMissionModal.tsx` (wizard state, new handler, JSX in `fs-browser-actions`)

- [ ] **Step 1: Add wizard state fields**

In the `WizardState` interface (around line 23, in the "File browser" group), add two fields after `browseError`:

```ts
	// File browser
	browseMode: boolean;
	browsePath: string;
	browseEntries: FSEntry[];
	browseLoading: boolean;
	browseError: string | null;
	newFolderName: string;
	creatingFolder: boolean;
```

In the initial `useState` value (around line 103), add the same two fields:

```ts
		browseLoading: false,
		browseError: null,
		newFolderName: "",
		creatingFolder: false,
		baseBranch: "main",
```

In the reset `useEffect` block (around line 167, inside the `setState((prev) => ({ ...prev, ... }))`), add resets so a reopened modal starts clean:

```ts
			browseLoading: false,
			browseError: null,
			newFolderName: "",
			creatingFolder: false,
			baseBranch: "main",
```

- [ ] **Step 2: Add the `handleCreateFolder` handler**

Place it immediately after the `handleSelectFolderForPath` / `handleSelectFolder` pair from Task 3 (before the `// ─── Step 2: Branch ───` comment):

```ts
	const handleCreateFolder = async () => {
		const name = state.newFolderName.trim();
		if (!name) {
			update({ browseError: "Enter a folder name" });
			return;
		}
		update({ creatingFolder: true, browseError: null });
		try {
			const { path } = await missionsApi.createFolder(state.browsePath, name);
			// Clear the input, then reuse the existing select flow which registers
			// the new folder as a project and selects it for the mission.
			update({ newFolderName: "" });
			await handleSelectFolderForPath(path);
		} catch (err) {
			update({ browseError: String(err) });
		} finally {
			update({ creatingFolder: false });
		}
	};
```

- [ ] **Step 3: Render the "New folder" input row inside `fs-browser-actions`**

Locate the `fs-browser-actions` block inside `renderProjectStep` (lines 573–583). It currently contains the current-path label and the Select/Cancel buttons. Insert a new row between the path label and the action buttons. Replace:

```tsx
					<div className="fs-browser-actions">
						<span className="fs-current-path">{state.browsePath}</span>
						<div className="row" style={{ gap: "var(--space-2)" }}>
							<button type="button" className="btn btn-primary" onClick={handleSelectFolder}>
								Select This Folder
							</button>
							<button type="button" className="btn btn-ghost" onClick={() => update({ browseMode: false })}>
								Cancel
							</button>
						</div>
					</div>
```

with:

```tsx
					<div className="fs-browser-actions">
						<span className="fs-current-path">{state.browsePath}</span>
						<div className="row" style={{ gap: "var(--space-2)", flex: 1 }}>
							<input
								type="text"
								className="fs-new-folder-input"
								placeholder="new-folder"
								value={state.newFolderName}
								onChange={(e) => update({ newFolderName: e.target.value, browseError: null })}
								onKeyDown={(e) => {
									if (e.key === "Enter" && !state.creatingFolder && state.newFolderName.trim()) {
										e.preventDefault();
										handleCreateFolder();
									}
								}}
								disabled={state.creatingFolder}
							/>
							<button
								type="button"
								className="btn btn-ghost"
								onClick={handleCreateFolder}
								disabled={state.creatingFolder || !state.newFolderName.trim()}
							>
								{state.creatingFolder ? "Creating…" : "Create Folder"}
							</button>
						</div>
						<div className="row" style={{ gap: "var(--space-2)" }}>
							<button type="button" className="btn btn-primary" onClick={handleSelectFolder}>
								Select This Folder
							</button>
							<button type="button" className="btn btn-ghost" onClick={() => update({ browseMode: false })}>
								Cancel
							</button>
						</div>
					</div>
```

> The new "New folder" row sits between the current-path label and the existing Select/Cancel row. Enter key submits. `browseError` is cleared on input change. No new CSS file needed — the existing `.btn`, `.btn-ghost`, `.row`, and `--space-*` variables style it. The `fs-new-folder-input` class is referenced but has no rule yet (it will render with browser defaults); styling is deliberately out of scope per the spec ("minor styling can reuse `.fs-browser-actions` / `.row`").

- [ ] **Step 4: Type-check and build**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai/internal/control/web && npx tsc --noEmit && npm run build
```
Expected: tsc clean, vite build succeeds (no new chunks fail).

- [ ] **Step 5: Commit**

```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow
git add ywai/internal/control/web/src/components/missions/CreateMissionModal.tsx
git commit -m "feat(ui): create new folder from mission file browser

Add a 'New folder' input + button to the fs-browser actions row in the
Create Mission modal. Entering a name creates a subfolder in the
currently-browsed directory via POST /api/fs/mkdir, then reuses the
existing select flow to register it as a project and select it for the
mission. Enter key submits; input is disabled while in-flight."
```

---

## Task 5: End-to-end verification + binary rebuild

**Files:** none modified — verification only.

- [ ] **Step 1: Run the backend test suite once more**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai && go test ./internal/missions/web/... -v
```
Expected: PASS (mkdir tests + all existing tests).

- [ ] **Step 2: Rebuild the embedded binary and restart the server**

Run:
```bash
cd /Users/nahuelcioffi/Proyectos/Yoizen/dev-ai-workflow/ywai
pkill -f "ywai-test serve" 2>/dev/null || true
npm --prefix internal/control/web run build
bash scripts/prepare-embedded.sh
go build -tags embedded -o /tmp/ywai-test ./cmd/ywai
nohup /tmp/ywai-test serve --port 5768 --no-mcp > /tmp/ywai-test.log 2>&1 &
sleep 2
```

- [ ] **Step 3: Verify the new endpoint responds**

Run:
```bash
TMPDIR_PARENT=$(mktemp -d)
curl -s -X POST http://localhost:5768/api/fs/mkdir \
  -H "Content-Type: application/json" \
  -d "{\"parentPath\":\"$TMPDIR_PARENT\",\"name\":\"verify-folder\"}"
echo
ls -la "$TMPDIR_PARENT/verify-folder" && echo "OK: folder created on disk"
```
Expected: `{"path":".../verify-folder"}` and the directory exists.

- [ ] **Step 4: Manual UI verification**

Open `http://localhost:5768` in a browser. Create Mission → Step 1 → "Browse Folders" → navigate to a writable directory → type a name in the new "New folder" input → click "Create Folder".

Expected:
- Input clears, browse mode exits, the new folder shows as the selected project.
- The folder exists on disk (`ls` the browsed parent).
- A duplicate name shows an error and keeps the input.

- [ ] **Step 5: (No commit — verification only)**

If anything fails, stop and debug with superpowers:systematic-debugging before declaring done.

---

## Self-Review (run after writing, before handing off)

- **Spec coverage:**
  - `POST /api/fs/mkdir` route + handler → Task 1. ✓
  - Sanitization (`filepath.Base`, prefix check, control-char reject) → Task 1 Step 3. ✓
  - `os.Mkdir` not `MkdirAll`, 409 on exist → Task 1 Step 3. ✓
  - Test table (valid, empty, separator, `..`, exist) + default-parent → Task 1 Step 1. ✓
  - `missionsApi.createFolder` client → Task 2. ✓
  - `handleSelectFolderForPath` refactor → Task 3. ✓
  - `handleCreateFolder` + wizard state (`newFolderName`, `creatingFolder`) + UI row → Task 4. ✓
  - Reuse of select flow for register+select → Task 4 Step 2. ✓
  - E2E verification (rebuild + curl + manual) → Task 5. ✓
- **Placeholder scan:** none — every code block is complete and copy-pasteable.
- **Type consistency:** `handleSelectFolderForPath(folderPath: string)` defined in Task 3, called in Task 4 (`handleSelectFolderForPath(path)`). `missionsApi.createFolder(parentPath, name)` defined in Task 2, called in Task 4. `MkdirFS` defined in Task 1, referenced in Task 1 Step 4 route registration. ✓
