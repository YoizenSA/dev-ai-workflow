// Orca pane-status bridge (CLI/TUI plugin).
// headless-tui-plugin: posts pane status to Orca via /hook/opencode.
//
// Runs inside the pane's own TUI process, so hook payloads carry THIS pane's
// ORCA_PANE_KEY / TAB_ID / WORKTREE_ID. A server plugin cannot do this: the
// shared background service has a single process env for every pane.
//
// The TUI event bus is NOT scoped to the sessions this pane displays: with the
// shared background service, every pane receives every session's events.
//
// Ownership ladder (first available signal wins):
// 1. Displayed session: ui.router.current() names the session this pane shows.
//    An event is reported only when its session is the displayed session.
//    This is what fixes multiple Orca cards on the SAME worktree: directory
//    equality passes for all of them, so sibling cards echoed every busy/idle
//    of every sibling session and every card flipped to the same state.
// 2. Headless children (background subagents): data.session.get cannot resolve
//    them in any pane, so an unresolvable session is owned here when its
//    data.session.family contains the displayed session. Family members are
//    stable, so positive results cache; failures retry on the next event.
// 3. Fallback when the router signal is unavailable: directory equality via
//    data.session.get -> location.directory against data.location.info()
//    (the pre-router behavior; kept for older builds).
//
// (Earlier bug for the record: an even earlier version filtered by session
// directory and dropped everything, because it compared fields that do not
// exist on the returned session object: the directory lives under
// session.location.directory, not session.directory, and data.session.get()
// returns undefined before the pane has an active session.)
//
// Orca's contract (/hook/opencode) accepts: SessionStart, SessionBusy,
// SessionIdle, MessagePart, PermissionRequest, AskUserQuestion.
//
// OpenCode 2.x emits session.* events; this maps them:
//   session.created (root)                  -> SessionStart
//   session.step.started               -> SessionBusy
//   session.prompted / prompt.admitted -> SessionBusy + MessagePart(user)
//   session.step.ended (!tool-calls)   -> SessionIdle
//   session.step.failed                -> SessionIdle
//   session.text.ended                 -> MessagePart(assistant)
//   permission.asked / permission.v2.asked  -> PermissionRequest
//   question.asked / question.v2.asked      -> AskUserQuestion
//   permission|question replied/rejected    -> SessionBusy or SessionIdle
//   session.status / session.idle           -> SessionBusy / SessionIdle

import * as fs from "node:fs"

const HOOK_PATH = "/hook/opencode"
const POST_TIMEOUT_MS = 2000
const DEBUG_LOG = "/tmp/opencode/orca-status-debug.log"

function debug(line: string) {
  try {
    fs.appendFileSync(
      DEBUG_LOG,
      `${new Date().toISOString()} pid=${process.pid} ${line}\n`,
    )
  } catch {}
}

let cachedEndpointKey = ""
let cachedEndpoint: Record<string, string> | null = null
let warnedBadEndpoint = false

function readEndpointFile() {
  const endpointPath = process.env.ORCA_AGENT_HOOK_ENDPOINT
  if (!endpointPath) return null
  try {
    const stat = fs.statSync(endpointPath)
    const key = `${stat.mtimeMs}:${stat.size}:${stat.ino}`
    if (key === cachedEndpointKey && cachedEndpoint) return cachedEndpoint
    const out: Record<string, string> = {}
    for (const line of fs.readFileSync(endpointPath, "utf8").split(/\r?\n/)) {
      const m = line.match(/^(?:set\s+)?([A-Z0-9_]+)=(.*)$/)
      if (m) out[m[1]] = m[2].replace(/\r$/, "")
    }
    cachedEndpointKey = key
    cachedEndpoint = out
    return out
  } catch (err: any) {
    cachedEndpointKey = ""
    cachedEndpoint = null
    if (err && err.code !== "ENOENT" && !warnedBadEndpoint) {
      warnedBadEndpoint = true
      console.warn("[orca-opencode-status] failed to parse endpoint file:", err.message)
    }
    return null
  }
}

function resolveCoords() {
  const file = readEndpointFile() || {}
  return {
    port: file.ORCA_AGENT_HOOK_PORT || process.env.ORCA_AGENT_HOOK_PORT,
    token: file.ORCA_AGENT_HOOK_TOKEN || process.env.ORCA_AGENT_HOOK_TOKEN,
    env: file.ORCA_AGENT_HOOK_ENV || process.env.ORCA_AGENT_HOOK_ENV || "",
    version: file.ORCA_AGENT_HOOK_VERSION || process.env.ORCA_AGENT_HOOK_VERSION || "",
  }
}

async function post(hookEventName: string, properties: Record<string, unknown>) {
  const coords = resolveCoords()
  const paneKey = process.env.ORCA_PANE_KEY
  if (!coords.port || !coords.token || !paneKey) {
    debug(`post ${hookEventName}: missing coords/paneKey`)
    return false
  }

  const url = `http://127.0.0.1:${coords.port}${HOOK_PATH}`
  const body = JSON.stringify({
    paneKey,
    launchToken: process.env.ORCA_AGENT_LAUNCH_TOKEN || "",
    tabId: process.env.ORCA_TAB_ID || "",
    worktreeId: process.env.ORCA_WORKTREE_ID || "",
    env: coords.env,
    version: coords.version,
    payload: { hook_event_name: hookEventName, ...(properties || {}) },
  })

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), POST_TIMEOUT_MS)
  if (timeout.unref) timeout.unref()
  try {
    const response = await fetch(url, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "X-Orca-Agent-Hook-Token": coords.token,
      },
      body,
      signal: controller.signal,
    })
    debug(
      `post ${hookEventName} -> ${response.status} ` +
        `session=${String((properties as any)?.sessionID ?? "-").slice(0, 20)} ` +
        `role=${String((properties as any)?.role ?? "-")}`,
    )
    return response.ok
  } catch (err: any) {
    debug(`post ${hookEventName} -> error ${err?.message ?? err}`)
    return false
  } finally {
    clearTimeout(timeout)
  }
}

// Server events arrive either as {type, properties} or wrapped as
// {details: {type, properties}} depending on the delivery path.
function eventEnvelope(raw: any): { type?: string; props: Record<string, any> } {
  if (!raw || typeof raw !== "object") return { props: {} }
  if (typeof raw.type === "string") return { type: raw.type, props: raw.properties ?? raw.data ?? {} }
  const inner = raw.details ?? raw.event ?? raw
  if (inner && typeof inner === "object" && typeof inner.type === "string") {
    return { type: inner.type, props: inner.properties ?? inner.data ?? {} }
  }
  return { props: {} }
}

function promptText(prompt: any): string {
  if (!prompt) return ""
  if (typeof prompt === "string") return prompt
  const parts = Array.isArray(prompt) ? prompt.parts : undefined
  if (Array.isArray(parts)) {
    return parts
      .filter((p: any) => p && p.type === "text" && typeof p.text === "string")
      .map((p: any) => p.text)
      .join("\n")
  }
  return typeof prompt.text === "string" ? prompt.text : ""
}

const SESSION_DIR_CACHE_MAX = 200

export default {
  id: "orca.opencode.status",
  async setup(context: any) {
    let busy = false
    let lastStatus = ""
    let lastAttention = ""

    // The pane's own directory. Resolved once from the TUI location API with a
    // process.cwd() fallback; undefined means "cannot prove ownership".
    let ownDirectory: string | undefined | null = null

    // sessionID -> session directory. Values are seeded from session.created
    // payloads when possible and otherwise resolved once via a shared promise,
    // so concurrent events for the same session never stampede data.session.get.
    const sessionDirValues = new Map<string, string | undefined>()
    const sessionDirPending = new Map<string, Promise<string | undefined>>()

    // child sessionID -> parent sessionID, learned from session.created
    // payloads. Background subagent sessions are headless: data.session.get
    // cannot resolve them in ANY pane (the TUI session cache does not hold
    // them), so their directory falls back to the parent session's directory.
    // The owner pane then reports the child's busy/idle and every other pane
    // discards it as foreign.
    const sessionParent = new Map<string, string>()

    debug(
      `setup pid=${process.pid} cwd=${process.cwd()} ` +
        `data.listen=${typeof context?.data?.listen === "function"} ` +
        `data.on=${typeof context?.data?.on === "function"} ` +
        `paneKey=${(process.env.ORCA_PANE_KEY || "").slice(0, 12)} ` +
        `endpointPort=${resolveCoords().port}`
    )

    async function resolveOwnDirectory(): Promise<string | undefined> {
      if (ownDirectory != null) return ownDirectory
      try {
        const info =
          typeof context?.data?.location?.info === "function"
            ? await context.data.location.info()
            : undefined
        ownDirectory =
          typeof info?.directory === "string" && info.directory
            ? info.directory
            : (process.cwd() || undefined)
      } catch {
        ownDirectory = process.cwd() || undefined
      }
      debug(`own directory resolved as ${ownDirectory ?? "<none>"}`)
      return ownDirectory
    }

    async function lookupDirectory(id: string): Promise<string | undefined> {
      try {
        const s = await context?.data?.session?.get?.(id)
        const d = s?.location?.directory ?? s?.directory
        return typeof d === "string" && d ? d : undefined
      } catch {
        return undefined
      }
    }

    // data.session.family(id) resolves server-side for EVERY session,
    // including headless child sessions (background subagents) that
    // data.session.get cannot resolve in any pane. The returned list contains
    // the session and its relatives.
    //
    // Family membership is stable once resolved, so positive results cache.
    // Failures are transient (a headless child is not family-resolvable for
    // its first ~seconds), so they are never cached: the next event retries.
    const familyMembersCache = new Map<string, string[]>()
    const familyMembersPending = new Map<string, Promise<string[] | undefined>>()
    async function familyMembers(sessionID: string): Promise<string[] | undefined> {
      const hit = familyMembersCache.get(sessionID)
      if (hit) return hit
      let pending = familyMembersPending.get(sessionID)
      if (!pending) {
        pending = (async () => {
          try {
            const fam = await context?.data?.session?.family?.(sessionID)
            if (!Array.isArray(fam)) return undefined
            const ids = fam
              .map((m: any) => (typeof m === "string" ? m : m?.id))
              .filter((id: any): id is string => typeof id === "string" && !!id)
            return ids.length ? ids : undefined
          } catch {
            return undefined
          }
        })()
        familyMembersPending.set(sessionID, pending)
      }
      const ids = await pending
      if (!ids) {
        familyMembersPending.delete(sessionID)
        return undefined
      }
      familyMembersCache.set(sessionID, ids)
      familyMembersPending.delete(sessionID)
      return ids
    }

    async function familyDirectory(sessionID: string): Promise<string | undefined> {
      const ids = await familyMembers(sessionID)
      if (!ids) return undefined
      for (const id of ids) {
        const dir = await lookupDirectory(id)
        if (dir) return dir
      }
      return undefined
    }

    async function sessionDirectory(sessionID: string): Promise<string | undefined> {
      if (sessionDirValues.has(sessionID)) return sessionDirValues.get(sessionID)
      let pending = sessionDirPending.get(sessionID)
      if (!pending) {
        pending = (async () => {
          const own = await lookupDirectory(sessionID)
          if (own) return own
          // Learned parent chain first (free, no data calls).
          let cur = sessionID
          for (let depth = 0; depth < 5; depth++) {
            const parent = sessionParent.get(cur)
            if (!parent) break
            const dir = await lookupDirectory(parent)
            if (dir) return dir
            cur = parent
          }
          // Then the family walk, which reaches headless children.
          return familyDirectory(sessionID)
        })()
        sessionDirPending.set(sessionID, pending)
        if (sessionDirPending.size > SESSION_DIR_CACHE_MAX) {
          const oldest = sessionDirPending.keys().next().value
          if (oldest !== undefined && oldest !== sessionID) sessionDirPending.delete(oldest)
        }
      }
      const dir = await pending
      if (!dir) {
        // Transient resolution failures happen right after a headless child
        // session is created (the TUI session cache syncs ~seconds later, and
        // family() may not know it yet). Never cache them: the next event
        // retries instead of freezing the session as unresolvable.
        sessionDirPending.delete(sessionID)
        return undefined
      }
      sessionDirValues.set(sessionID, dir)
      if (sessionDirValues.size > SESSION_DIR_CACHE_MAX) {
        const oldest = sessionDirValues.keys().next().value
        if (oldest !== undefined && oldest !== sessionID) sessionDirValues.delete(oldest)
      }
      return dir
    }

    // The session this pane currently displays, read from the router at event
    // time. Semantics: string = displayed session id, null = the router works
    // and names no session (home / plugin route), undefined = the signal is
    // unavailable and callers must fall back to directory equality.
    //
    // The TUI uses a placeholder ("dummy") sessionID before navigation
    // resolves; that counts as "not determinable yet" so the fallback applies
    // instead of comparing against an id no event will carry.
    let routerBroken = false
    let routerShapeLogged = false
    let displayedLogged = ""
    async function resolveDisplayedSession(): Promise<string | null | undefined> {
      if (routerBroken) return undefined
      try {
        const route: any = await context?.ui?.router?.current?.()
        if (!route || typeof route !== "object") return undefined
        if (!routerShapeLogged) {
          routerShapeLogged = true
          debug(`router.current=${JSON.stringify(route).slice(0, 200)}`)
        }
        const isSession = route.type === "session" || route.name === "session"
        if (!isSession) return null
        const sid =
          typeof route.sessionID === "string"
            ? route.sessionID
            : typeof route.params?.sessionID === "string"
              ? route.params.sessionID
              : undefined
        if (sid && sid !== "dummy") {
          if (sid !== displayedLogged) {
            displayedLogged = sid
            debug(`pane displays session ${sid}`)
          }
          return sid
        }
        return undefined
      } catch {
        routerBroken = true
        debug("ui.router.current unavailable; directory fallback active")
        return undefined
      }
    }

    // Does `childID` belong to the displayed session `rootID`? Two cheap
    // checks: the learned parent chain first (free, no data calls), then the
    // server-resolved family, which reaches headless children.
    async function belongsToDisplayed(childID: string, rootID: string): Promise<boolean> {
      let cur = childID
      for (let depth = 0; depth < 5; depth++) {
        const parent = sessionParent.get(cur)
        if (!parent) break
        if (parent === rootID) return true
        cur = parent
      }
      const fam = await familyMembers(childID)
      return !!fam && fam.includes(rootID)
    }

    // A pane only reports events for the session it displays (or for sessions
    // the displayed session owns). Events with no sessionID keep the previous
    // behavior (report) because a dropped permission or question prompt would
    // strand the agent. When the displayed-session signal is unavailable the
    // older directory filter applies.
    // Skip reasons are logged once per session to keep the debug log small.
    const skipLogged = new Set<string>()
    async function isOwnSession(sessionID?: string): Promise<boolean> {
      if (!sessionID) return true
      const displayed = await resolveDisplayedSession()
      if (displayed !== undefined) {
        if (sessionID === displayed) return true
        const owned = await belongsToDisplayed(sessionID, displayed)
        if (!owned) {
          if (!skipLogged.has(`notmine:${sessionID}`)) {
            skipLogged.add(`notmine:${sessionID}`)
            debug(
              `skip session ${sessionID} not displayed here ` +
                `(displayed=${displayed ?? "<none>"})`
            )
          }
          return false
        }
        return true
      }

      // Fallback: directory equality (pre-router behavior).
      const own = await resolveOwnDirectory()
      if (!own) {
        if (!skipLogged.has("own")) {
          skipLogged.add("own")
          debug(`cannot resolve own directory; reporting foreign-capable events`)
        }
        return true
      }
      const dir = await sessionDirectory(sessionID)
      if (!dir) {
        if (!skipLogged.has(`unresolvable:${sessionID}`)) {
          skipLogged.add(`unresolvable:${sessionID}`)
          debug(`skip unresolvable session ${sessionID} (own=${own})`)
        }
        return false
      }
      if (dir !== own) {
        if (!skipLogged.has(`foreign:${sessionID}`)) {
          skipLogged.add(`foreign:${sessionID}`)
          debug(`skip foreign session ${sessionID} (dir=${dir} own=${own})`)
        }
        return false
      }
      return true
    }

    async function setStatus(next: "busy" | "idle", sessionID?: string) {
      const key = `${next}:${sessionID || ""}`
      if (key === lastStatus) return
      const ok = await post(next === "busy" ? "SessionBusy" : "SessionIdle", {
        sessionID: sessionID || "",
      })
      if (ok) lastStatus = key
    }

    async function setAttention(name: string, properties: Record<string, unknown>, dedupe: string) {
      if (dedupe === lastAttention) return
      const ok = await post(name, properties)
      if (ok) lastAttention = dedupe
    }

    async function handle(raw: any) {
      const { type: rawType, props: p } = eventEnvelope(raw)
      // Why: the server stream names these session.*, while the TUI event
      // bus delivers the same events without the ".next." segment.
      const type = (rawType ?? "").replace(/\.next\./, ".")
      // Why: session.created carries the session in info, not in
      // properties.sessionID; ownership must judge the created session, not
      // "an event with no sessionID".
      const sessionID: string | undefined = p.sessionID ?? p.info?.id

      // Seed the caches from session.created payloads: children record their
      // parent (for the headless-subagent fallback) and inherit the parent's
      // directory, so ownership needs no extra data call.
      if (type === "session.created") {
        const info = p.info
        if (info?.id) {
          const dir = info?.location?.directory ?? info?.directory
          const hasOwnDir = typeof dir === "string" && !!dir
          if (hasOwnDir) {
            sessionDirValues.set(info.id, dir)
          }
          if (typeof info.parentID === "string" && info.parentID) {
            sessionParent.set(info.id, info.parentID)
            if (!hasOwnDir) {
              const parentDir = sessionDirValues.get(info.parentID)
              if (parentDir) sessionDirValues.set(info.id, parentDir)
            }
            // If an earlier event negative-cached this child before its
            // creation event arrived, clear it so it resolves now.
            if (sessionDirValues.get(info.id) === undefined) {
              sessionDirValues.delete(info.id)
              sessionDirPending.delete(info.id)
            }
          }
        }
      }

      if (!(await isOwnSession(sessionID))) return

      if (type === "session.created") {
        const info = p.info
        if (info?.id && !info.parentID) {
          lastStatus = ""
          const session = p.sessionID || info.id
          const ok = await post("SessionStart", { sessionID: session })
          if (ok) lastStatus = `idle:${session}`
        }
        return
      }

      if (type === "session.step.started") {
        busy = true
        await setStatus("busy", sessionID)
        return
      }

      if (type === "session.prompted" || type === "session.prompt.admitted") {
        busy = true
        const text = promptText(p.prompt)
        if (text) {
          lastAttention = ""
          await post("MessagePart", { role: "user", text, sessionID, messageID: p.messageID })
        }
        await setStatus("busy", sessionID)
        return
      }

      if (type === "session.step.ended") {
        if (p.finish !== "tool-calls") {
          busy = false
          await setStatus("idle", sessionID)
        }
        return
      }

      if (type === "session.step.failed") {
        busy = false
        await setStatus("idle", sessionID)
        return
      }

      if (type === "session.text.ended") {
        if (typeof p.text === "string" && p.text.trim()) {
          await post("MessagePart", {
            role: "assistant",
            text: p.text,
            sessionID,
            messageID: p.assistantMessageID,
          })
        }
        return
      }

      if (type === "permission.asked" || type === "permission.v2.asked") {
        lastStatus = ""
        await setAttention("PermissionRequest", { ...p, sessionID }, `perm:${p.id || sessionID || ""}`)
        return
      }

      if (type === "question.asked" || type === "question.v2.asked") {
        lastStatus = ""
        await setAttention("AskUserQuestion", { ...p, sessionID }, `q:${p.id || sessionID || ""}`)
        return
      }

      if (type === "permission.replied" || type === "question.replied" || type === "question.rejected") {
        lastAttention = ""
        await setStatus(busy ? "busy" : "idle", sessionID)
        return
      }

      if (type === "session.status") {
        const statusType = p.status?.type
        if (statusType === "busy" || statusType === "retry") {
          busy = true
          await setStatus("busy", sessionID)
        } else if (statusType === "idle") {
          busy = false
          await setStatus("idle", sessionID)
        }
        return
      }

      if (type === "session.idle") {
        busy = false
        await setStatus("idle", sessionID)
        return
      }

      if (type === "session.deleted") {
        lastStatus = ""
        lastAttention = ""
        if (sessionID) {
          sessionDirValues.delete(sessionID)
          sessionDirPending.delete(sessionID)
        }
      }
    }

    async function onEvent(raw: any) {
      try {
        await handle(raw)
      } catch (err: any) {
        debug(`handle error: ${err?.message ?? err}`)
      }
    }
    const unsubscribers: Array<() => void> = []

    // Preferred: one subscription for every server event.
    if (typeof context?.data?.listen === "function") {
      try {
        const off = context.data.listen((details: any) => void onEvent(details))
        if (typeof off === "function") unsubscribers.push(off)
        debug("subscribed via data.listen")
      } catch (err: any) {
        debug(`data.listen failed: ${err?.message ?? err}`)
      }
    }

    // New API, per-type.
    if (!unsubscribers.length && typeof context?.data?.on === "function") {
      for (const type of [
        "session.created",
        "session.step.started",
        "session.prompted",
        "session.prompt.admitted",
        "session.step.ended",
        "session.step.failed",
        "session.text.ended",
        "permission.asked",
        "permission.v2.asked",
        "question.asked",
        "question.v2.asked",
        "permission.replied",
        "question.replied",
        "question.rejected",
        "session.status",
        "session.idle",
        "session.deleted",
      ]) {
        try {
          const off = context.data.on(type, (event: any) => void onEvent(event))
          if (typeof off === "function") unsubscribers.push(off)
        } catch {}
      }
      debug(`subscribed via data.on (${unsubscribers.length} types)`)
    }

    // Older API: per-type on the event bus.
    if (!unsubscribers.length && typeof context?.event?.on === "function") {
      for (const type of [
        "session.created",
        "session.step.started",
        "session.prompted",
        "session.prompt.admitted",
        "session.step.ended",
        "session.step.failed",
        "session.text.ended",
        "permission.asked",
        "permission.v2.asked",
        "question.asked",
        "question.v2.asked",
        "permission.replied",
        "question.replied",
        "question.rejected",
        "session.status",
        "session.idle",
        "session.deleted",
      ]) {
        try {
          const off = context.event.on(type, (event: any) => void onEvent(event))
          if (typeof off === "function") unsubscribers.push(off)
        } catch {}
      }
      debug(`subscribed via event.on (${unsubscribers.length} types)`)
    }

    if (!unsubscribers.length) {
      debug("NO event API found — plugin cannot subscribe")
    }

    return () => {
      for (const off of unsubscribers) {
        try {
          off()
        } catch {}
      }
    }
  },
}
