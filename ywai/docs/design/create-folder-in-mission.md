# Create Folder from New-Mission Modal

## Problem

In the **New Mission** modal (Step 1 — Project), the user can browse the filesystem
and select an *existing* folder to register it as a project. There is no way to
**create a new folder** from the UI. Today the only path to a fresh folder is to
leave the app, `mkdir` on the terminal, then come back and browse to it.

## Goal

Allow the user, inside the file browser of the New Mission modal, to **create a new
subfolder inside the currently-browsed directory**, then automatically register it
as a project and select it for the mission — mirroring today's "Select This Folder"
flow, but for a folder that did not exist a moment ago.

## Non-goals

- Creating folders outside the browser context (e.g. from the project list directly).
- Creating nested paths (`a/b/c`) in one shot — only a single new segment.
- Initializing the new folder as a git repo, adding boilerplate, etc. The folder is
  just an empty directory; whether it becomes a repo is a separate concern.
- Changes to the project model. `Mission.Project` keeps holding a project **name**;
  the folder path keeps living on the `Project` record, as today.

## Approach

New FS primitive `POST /api/fs/mkdir`, symmetric to the existing `GET /api/fs/browse`.
The browser UI gains an input + button to create a folder in the current directory;
on success it navigates into the new folder and runs the existing folder-selection
flow (register project + select). This keeps the **FS** and **project-store**
responsibilities separate, exactly as the current code does.

## Backend

### Route — `internal/missions/web/server.go`

Register next to `BrowseFS`:

```go
mux.HandleFunc("POST /api/fs/mkdir", h.MkdirFS)
```

### Handler — `internal/missions/web/handlers.go`

`MkdirFS` parses:

```go
var req struct {
    ParentPath string `json:"parentPath"`
    Name       string `json:"name"`
}
```

Behavior:

1. **Validate name**
   - `strings.TrimSpace(name) == ""` → 400 "folder name is required".
   - `name = filepath.Base(name)` — strips any path component, neutralizing
     traversal attempts and nested-path input.
   - Reject if, after `Base`, the result is `.`, `..`, or empty → 400 invalid.
   - Reject names containing a null byte or control chars → 400.
2. **Resolve parent**
   - If `parentPath == ""` → default to `os.UserHomeDir()` (same as `BrowseFS`).
   - `parentPath = filepath.Clean(parentPath)`.
3. **Compute target**
   - `full = filepath.Join(parentPath, name)`.
   - Safety check: after `Join`/`Clean`, `full` must have `parentPath` as a prefix
     (i.e. `filepath.Dir(full) == parentPath`). If not → 400 "invalid folder name".
4. **Create**
   - `os.Mkdir(full, 0755)` — **not** `MkdirAll`. A pre-existing path must error,
     so the user gets clear feedback rather than a silent no-op.
5. **Respond**
   - 201: `{"path": full}`.
   - On `os.IsExist(err)` → 409 "folder already exists".
   - Other IO errors → 500 with the error message.

`MkdirFS` is a method on the existing `*Handlers` struct; no new state needed.

## Frontend

### API client — `internal/control/web/src/api/client.ts`

Add to `missionsApi`:

```ts
createFolder: (parentPath: string, name: string) =>
  api.post<{ path: string }>("/missions/api/fs/mkdir", { parentPath, name }),
```

(Follows the existing `/missions` proxy prefix used by other calls in this file.)

### Component — `internal/control/web/src/components/missions/CreateMissionModal.tsx`

**New wizard state fields:**

```ts
newFolderName: string;     // text input value
creatingFolder: boolean;   // disables input+button while in-flight
```

Reset both on modal open (same block that resets `browseMode`, `browseEntries`, …).

**New handler `handleCreateFolder`:**

1. Local validation: trim `newFolderName`; if empty, set `browseError` and abort.
2. Set `creatingFolder: true`, clear `browseError`.
3. Call `missionsApi.createFolder(state.browsePath, name)`.
4. On success (`{path}`):
   - Clear the input (`newFolderName: ""`).
   - Reuse the existing selection flow by treating the new folder as the current
     browse target: `await handleSelectFolderForPath(path)` — a small refactor that
     extracts the "register project + select" body of the current `handleSelectFolder`
     into a function that takes an explicit path (the current handler derives it
     from `state.browsePath`). The current `handleSelectFolder` then calls this
     helper with `state.browsePath`, and `handleCreateFolder` calls it with the
     freshly-created `path`.
5. On 409 (already exists): show `browseError: "Folder already exists"`, keep
   input value so the user can edit it.
6. On other errors: `browseError: String(err)`.
7. Finally: `creatingFolder: false`.

**UI placement** — inside the existing `fs-browser` block, in `fs-browser-actions`
(currently holds the current-path label + "Select This Folder" / "Cancel" buttons):

```
[ parent path label .............................. ]
[ New folder name: [____input____] [Create Folder] ]
[ Select This Folder ] [ Cancel ]
```

- Input is `type="text"`, `placeholder="new-folder"`, bound to `newFolderName`.
- "Create Folder" button: `btn btn-ghost`, disabled while `creatingFolder` or input
  empty. Shows "Creating…" while in-flight.
- Layout: a thin row above the existing action buttons; reuses existing CSS
  variables (`--space-2`, etc.). No new stylesheet rules required for correctness;
  minor styling can reuse `.fs-browser-actions` / `.row`.

## Security

The server is localhost-only and the operation is a single-segment `mkdir`, but the
handler still hardens against accidental misuse:

- `filepath.Base(name)` strips path separators from the name — no `../`, no nested
  paths, no absolute paths.
- Prefix check (`filepath.Dir(full) == parentPath`) is a second line of defense.
- `os.Mkdir` (not `MkdirAll`) prevents silently "succeeding" on an existing path or
  creating intermediate directories the user didn't intend.

## Testing

### Backend — Go test

`internal/missions/web/handlers_mkdir_test.go` (new file), table-driven over a
temp dir:

| Case | Expectation |
|---|---|
| Valid name in valid parent | 201, dir exists on disk, response `path` matches |
| Empty name | 400 |
| Name with path separator (`a/b`) | 400 (after `Base`, `Dir(full) != parent`) |
| `..` as name | 400 |
| Pre-existing folder | 409 |
| Non-existent parent | 500 (IO error surfaced) |

Uses `httptest.NewRequest` + `httptest.NewRecorder` against a `Handlers{}` with a
nil store (MkdirFS doesn't touch the store). Temp dir created/fixed per test via
`t.TempDir()`.

### Frontend

No test harness exists today (vite, no vitest). Verified manually via the running
dashboard: create folder, confirm it appears in the browser, confirm the mission's
selected project is the new folder.

## Out of scope, noted for later

- The web `RunMission` handler (`handlers.go`) still uses `DefaultEngineConfig()`
  without a `RepoResolver`, so the selected project's folder is **ignored at run
  time** when a mission is started from the dashboard. That is a pre-existing gap,
  unrelated to creating folders, and is not addressed here.
