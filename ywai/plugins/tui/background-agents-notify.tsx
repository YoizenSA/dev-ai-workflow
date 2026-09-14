// @ts-nocheck
/**
 * background-agents-notify — the human half of delegation notifications.
 *
 * The server plugin tells the MODEL (synthetic session input); this sidecar
 * tells the HUMAN: it subscribes to the delegation terminal event the server
 * half emits and raises an OS notification (only when the terminal is
 * unfocused) plus a native in-TUI toast.
 *
 * Why not just use the built-in notifications plugin: it reacts to
 * session.execution.succeeded/failed and calls attention.notify, but for a
 * subagent (session.parentID set) it passes `notification: false`, so the
 * human gets a sound and no OS notification. That suppression is the gap.
 *
 * Requires `attention.enabled` in cli.json: attention.notify RETURNS a reason
 * ("attention_disabled") instead of throwing when it is off, which is how this
 * once looked like it worked while notifying nobody.
 *
 * RPC mirror: these constants duplicate terminal-events.ts in the server
 * bundle (standalone source, cannot import it); the test pins both copies.
 */

// headless-tui-plugin: event subscriber (attention.notify + ui.toast), renders nothing by design.
const id = "ywai-background-agents-notify"

const TERMINAL_RPC_ID = "ywai-background-agents"
const TERMINAL_EVENT_NAME = "delegation_terminal"

// The host publishes plugin-RPC events on the normal data bus as
// `rpc.<rpcID>.<event>`; the client-side RPC surface this once reached for
// does not exist in the runtime.
const TERMINAL_EVENT_TYPE = `rpc.${TERMINAL_RPC_ID}.${TERMINAL_EVENT_NAME}`

const oneLine = (value, limit) => String(value ?? "").replace(/\s+/g, " ").trim().slice(0, limit)

function noteFor(data) {
  if (!data || typeof data !== "object") return undefined
  if (data.kind === "all-complete") {
    return { title: "Background agents", message: "All delegations complete.", variant: "success" }
  }
  if (data.kind !== "terminal") return undefined
  const ok = data.status === "complete"
  const label = data.title ? oneLine(data.title, 60) : data.delegationID
  const remaining =
    typeof data.remaining === "number" && data.remaining > 0 ? ` (${data.remaining} more running)` : ""
  // The error text is what makes a dead key or a bad model name visible
  // instead of the delegation just never coming back.
  const detail = ok ? "" : data.error ? ` — ${oneLine(data.error, 120)}` : ""
  return {
    title: oneLine(`Delegation ${data.status}: ${data.delegationID}`, 80),
    message: oneLine(`${data.agent} — ${label}${remaining}${detail}`, 240),
    variant: ok ? "success" : "error",
  }
}

const setup = (ctx) => {
  const handle = (event) => {
    let note
    try {
      note = noteFor(event?.data)
    } catch {
      return
    }
    if (!note) return
    try {
      // OS notification only when unfocused; no sound — the built-in plugin
      // already owns subagent sounds, and doubling them is worse than silence.
      const result = ctx.attention.notify({
        title: note.title,
        message: note.message,
        notification: { when: "blurred" },
        sound: false,
      })
      void Promise.resolve(result).catch(() => {})
    } catch {}
    try {
      ctx.ui.toast.show({ title: note.title, message: note.message, variant: note.variant })
    } catch {}
  }

  // Deliberately unguarded: a missing subscribe surface means the host
  // contract changed, and swallowing that is what made this fail silently.
  const unsubscribe = ctx.data.on(TERMINAL_EVENT_TYPE, handle)

  return () => {
    try {
      unsubscribe?.()
    } catch {}
  }
}

const plugin = { id, setup }
export default plugin
