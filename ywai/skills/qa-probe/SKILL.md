---
name: qa-probe
description: "Probe an app with Jev via jev_do, never the desktop browser. Trigger: qa-probe, try this app, smoke test, exploratory QA."
---

# QA Probe

The user points at a URL and some cases. Run them with **`jev_do`**, not
desktop Chrome, not `opencode-in-chrome`, not `browser.tabs.*`.

Load `jev-gate`. File proof like `qa-evidence`.

## Intake

Need a URL and at least one case. If either is missing, ask — do not invent the app.

Credentials (if login is in the brief):

- Ask once. A password in chat authorizes login only, not extra flows you invented.
- Put them in `jev_do` **`values`**, never in `goal`. Jev sees the goal; TypeSafe
  must not receive the password.
- Mask as `***` in chat, reports, and screenshot captions.

Show a numbered plan (case → expected). Wait for confirmation. Then run.

## Drive

One `jev_do` per case. Goal = the outcome in one sentence. Strings to type go in
`values`. Keys match the field's **id, name, label, placeholder, or index**.
`email` / `password` match `type=email` and `type=password`.

```ts
return await tools.jev_do({
  url: "http://localhost:3001/admin",
  goal: "Log in to the admin dashboard",
  values: { email: "...", password: "..." },
  maxSteps: 8,
})
```

Then, if the case has a visible Then, `jev_check_page` with the snapshot and the
Then verbatim. You do not invent PASS.

A missing `jev_do`, a missing TypeSafe key, a desktop-browser disconnect, **or
any `jev_do` error** is **BLOCKED**, not FAIL. Quote the error verbatim. Cite
the `Evidence:` path if present. Do not retry `browser.tabs.*`.

If `jev_do` refuses TYPE_TEXT: read the field refs in the error (id, name,
type). Retry once with those keys. Then BLOCKED.

Diagnose matching without clicking:

```ts
return await tools.jev_do({
  url: "http://localhost:3001/admin",
  goal: "Log in to the admin dashboard",
  values: { email: "...", password: "..." },
  dryRun: true,
})
```

`jev_do` is the **jev-gate plugin** (OpenCode2 loads `plugins/jev-gate.js`), not
an MCP server. Do not look for it under `mcp.servers`.

No deletes, payments, or mass edits unless that case was explicit.

## Evidence

`jev_do` already writes `.evidence/jev-do-<id>/report.md` with a human title
("# Test: …") and captioned screenshots. Cite that path. Do not paste images
into the chat (413).

Close with:

```
| # | Use case | Verdict | Evidence |
```

FAIL: expected vs observed. BLOCKED: what was missing. Never PASS a step you
did not observe.
