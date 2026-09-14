import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  envsApi,
  type EnvCatalog,
  type EnvLogs,
  type EnvProfile,
  type EnvSpecSection,
  type EnvStatus,
} from '../../api/envs'
import { setConfigProfileScope } from '../../api/client'
import './Envs.css'

// Mirrors envprofile.ValidateName (lowercase, digits, hyphens, max 32).
const NAME_RE = /^[a-z][a-z0-9-]{0,31}$/

const EMPTY_CATALOG: EnvCatalog = { groups: [], skills: [], mcp: [] }

function formatBytes(n: number): string {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleDateString()
}

function errText(e: unknown): string {
  const msg = e instanceof Error ? e.message : String(e)
  // Backend errors arrive as `400: {"error":"..."}` — show just the message.
  const m = msg.match(/"error"\s*:\s*"([^"]+)"/)
  return m ? m[1] : msg
}

export default function Envs() {
  const navigate = useNavigate()
  const [envs, setEnvs] = useState<EnvProfile[]>([])
  const [presets, setPresets] = useState<string[]>([])
  const [presetDescriptions, setPresetDescriptions] = useState<Record<string, string>>({})
  const [catalog, setCatalog] = useState<EnvCatalog>(EMPTY_CATALOG)
  const [customPresets, setCustomPresets] = useState<string[]>([])
  // "Copy providers & credentials from global" for new envs. Follows the
  // selected preset's default until the user toggles it.
  const [presetCopyDefaults, setPresetCopyDefaults] = useState<Record<string, boolean>>({})
  const [copyProviders, setCopyProviders] = useState(true)
  const [copyTouched, setCopyTouched] = useState(false)
  const [notice, setNotice] = useState<string | null>(null)
  // Inline duplicate/rename form for the selected preset in "New environment".
  const [presetAction, setPresetAction] = useState<{ kind: 'duplicate' | 'rename'; value: string } | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newPreset, setNewPreset] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [details, setDetails] = useState<Record<string, EnvStatus>>({})
  const [logs, setLogs] = useState<Record<string, EnvLogs>>({})
  const [outputs, setOutputs] = useState<Record<string, { ok: boolean; text: string }>>({})
  // Envs whose content changed since their last Apply (client-side only).
  const [pendingApply, setPendingApply] = useState<Record<string, boolean>>({})

  const load = useCallback(async () => {
    const res = await envsApi.list()
    setEnvs(res.envs ?? [])
    setPresets(res.presets ?? [])
    setPresetDescriptions(res.preset_descriptions ?? {})
    setCatalog(res.catalog ?? EMPTY_CATALOG)
    setCustomPresets(res.custom_presets ?? [])
    setPresetCopyDefaults(res.preset_copy_providers ?? {})
    setNewPreset((cur) => (cur && res.presets?.includes(cur) ? cur : res.presets?.[0] || ''))
  }, [])

  const refresh = useCallback(async () => {
    setError(null)
    try {
      await load()
    } catch (e) {
      setError(errText(e))
    } finally {
      setLoading(false)
    }
  }, [load])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    if (!copyTouched) setCopyProviders(!!presetCopyDefaults[newPreset])
  }, [newPreset, presetCopyDefaults, copyTouched])

  const copiedSummary = (c?: { providers: string[] | null; credentials: string[] | null; note?: string }) => {
    const parts = [
      c?.providers?.length ? `providers ${c.providers.join(', ')}` : '',
      c?.credentials?.length ? `logins ${c.credentials.join(', ')}` : '',
    ].filter(Boolean)
    const head = parts.length ? `Copied from global: ${parts.join(' · ')}.` : 'Nothing new to copy from global.'
    return c?.note ? `${head} Note: ${c.note}.` : head
  }

  const handleImportProviders = (name: string) =>
    void run(`import:${name}`, async () => {
      const res = await envsApi.importProviders(name)
      setNotice(`${name}: ${copiedSummary(res.copied)}`)
    })

  const adoptedAdoSummary = (c?: { profiles: string[] | null; defaultProfile?: boolean }) => {
    const head = c?.profiles?.length
      ? `Ado profiles ${c.profiles.join(', ')}${c.defaultProfile ? ' + default' : ''}.`
      : 'Ado profiles already in sync.'
    return head
  }

  const handleImportAdo = (name: string) =>
    void run(`import-ado:${name}`, async () => {
      const res = await envsApi.importAdo(name)
      setNotice(`${name}: ${adoptedAdoSummary(res.copied_ado)}`)
    })

  const anyBusy = busy !== null
  const isBusy = (key: string) => busy === key

  // run wraps one mutation: busy flag, error banner, list reload.
  async function run(key: string, fn: () => Promise<void>, reload = true) {
    setBusy(key)
    setError(null)
    try {
      await fn()
      if (reload) await load()
    } catch (e) {
      setError(errText(e))
    } finally {
      setBusy(null)
    }
  }

  async function refreshDetails(name: string) {
    try {
      const [st, lg] = await Promise.all([envsApi.status(name), envsApi.logs(name, 200)])
      setDetails((prev) => ({ ...prev, [name]: st }))
      setLogs((prev) => ({ ...prev, [name]: lg }))
    } catch (e) {
      setError(errText(e))
    }
  }

  const nameTrim = newName.trim()
  const nameError = nameTrim && !NAME_RE.test(nameTrim)
    ? 'Use lowercase letters, digits and hyphens; start with a letter.'
    : envs.some((e) => e.name === nameTrim) ? 'An env with this name already exists.' : null
  const canCreate = !!nameTrim && !nameError && !anyBusy

  const handleCreate = () => {
    if (!canCreate) return
    void run(`create:${nameTrim}`, async () => {
      const res = await envsApi.create(nameTrim, newPreset || undefined, copyProviders)
      setNewName('')
      setCopyTouched(false)
      if (res.copy_error) setError(`Env created, but copying providers failed: ${res.copy_error}`)
      else if (copyProviders) setNotice(`${nameTrim}: ${copiedSummary(res.copied)}`)
      if (res.copy_ado_error) setError(`Env created, but copying ado profiles failed: ${res.copy_ado_error}`)
      else if (copyProviders && (res.copied_ado?.profiles?.length || res.copied_ado?.defaultProfile)) {
        setNotice(`${nameTrim}: ${adoptedAdoSummary(res.copied_ado)}`)
      }
    })
  }

  const submitPresetAction = () => {
    if (!presetAction) return
    const name = presetAction.value.trim()
    if (!name) return
    const { kind } = presetAction
    void run(`preset-${kind}:${newPreset}`, async () => {
      if (kind === 'duplicate') await envsApi.copyPreset({ from: newPreset, name })
      else await envsApi.patchPreset(newPreset, { name })
      setPresetAction(null)
      setNewPreset(name)
    })
  }

  const handleDeletePreset = () => {
    if (!window.confirm(`Delete preset "${newPreset}"? Envs already created from it keep working only if they switch preset first.`)) return
    void run(`preset-delete:${newPreset}`, async () => {
      await envsApi.deletePreset(newPreset)
      setNewPreset('')
    })
  }

  const handlePresetChange = (env: EnvProfile, preset: string) => {
    const customized = details[env.name]?.overrides != null
    if (customized && !window.confirm(`Switching "${env.name}" to "${preset}" discards its custom groups/skills/MCP. Continue?`)) return
    void run(`preset:${env.name}`, async () => {
      await envsApi.updatePreset(env.name, preset)
      setPendingApply((p) => ({ ...p, [env.name]: true }))
      if (expanded[env.name]) await refreshDetails(env.name)
    })
  }

  const handleDelete = (name: string) => {
    if (!window.confirm(`Delete env "${name}"? Its service stops and its directory (config, database, logs) is removed.`)) return
    void run(`delete:${name}`, async () => {
      await envsApi.remove(name)
    })
  }

  const handleStartStop = (env: EnvProfile) =>
    void run(`${env.running ? 'stop' : 'start'}:${env.name}`, async () => {
      if (env.running) await envsApi.stop(env.name)
      else await envsApi.start(env.name)
    })

  const handleApply = (name: string) =>
    void run(`apply:${name}`, async () => {
      try {
        const res = await envsApi.apply(name)
        setOutputs((prev) => ({ ...prev, [name]: { ok: true, text: res.output } }))
        setPendingApply((p) => ({ ...p, [name]: false }))
      } catch (e) {
        setOutputs((prev) => ({ ...prev, [name]: { ok: false, text: errText(e) } }))
        throw e
      } finally {
        setExpanded((prev) => ({ ...prev, [name]: true }))
        await refreshDetails(name)
      }
    }, false)

  const toggleExpanded = (name: string) => {
    const next = !expanded[name]
    setExpanded((prev) => ({ ...prev, [name]: next }))
    if (next) void refreshDetails(name)
  }

  if (loading) {
    return <div className="envs"><p className="envs-muted">Loading environments…</p></div>
  }

  return (
    <div className="envs">
      <header className="page-header envs-header">
        <div className="page-heading">
          <span className="page-eyebrow">Isolation</span>
          <h1 className="page-title">Environments</h1>
          <p className="page-subtitle">
            Each environment is an isolated opencode home — its own agents, skills, MCP servers, memory database and service.
          </p>
        </div>
        <button className="btn btn-ghost btn-sm" onClick={() => void refresh()} disabled={anyBusy}>Refresh</button>
      </header>

      {notice && (
        <div className="envs-notice" role="status">
          <span>{notice}</span>
          <button className="envs-alert-close" aria-label="Dismiss" onClick={() => setNotice(null)}>×</button>
        </div>
      )}

      {error && (
        <div className="envs-alert" role="alert">
          <span>{error}</span>
          <button className="envs-alert-close" aria-label="Dismiss" onClick={() => setError(null)}>×</button>
        </div>
      )}

      <section className="card card-pad envs-create">
        <div className="envs-section-head">
          <h2>New environment</h2>
          <p className="envs-muted">Pick a starting preset — you can fine-tune its content afterwards.</p>
        </div>
        <div className="field">
          <label className="field-label" htmlFor="env-new-name">Name</label>
          <input
            id="env-new-name"
            className={`input envs-name-input ${nameError ? 'invalid' : ''}`}
            placeholder="e.g. client-acme"
            value={newName}
            disabled={anyBusy}
            onChange={(e) => setNewName(e.target.value.toLowerCase())}
            onKeyDown={(e) => { if (e.key === 'Enter') handleCreate() }}
          />
          <span className={`field-help ${nameError ? 'error' : ''}`}>
            {nameError ?? 'Also works as a shortcut: ywai <name> runs opencode inside it.'}
          </span>
        </div>
        <div className="field">
          <span className="field-label">Preset</span>
          <div className="envs-preset-grid" role="radiogroup" aria-label="Preset">
            {presets.map((p) => (
              <button
                key={p}
                type="button"
                role="radio"
                aria-checked={newPreset === p}
                className={`envs-preset ${newPreset === p ? 'is-selected' : ''}`}
                onClick={() => setNewPreset(p)}
                disabled={anyBusy}
              >
                <span className="envs-preset-name">
                  {p}
                  {customPresets.includes(p) && <span className="envs-preset-tag">custom</span>}
                </span>
                {presetDescriptions[p] && <span className="envs-preset-desc">{presetDescriptions[p]}</span>}
              </button>
            ))}
          </div>
          {newPreset && (
            <div className="envs-preset-tools">
              {presetAction ? (
                <>
                  <input
                    className="input envs-preset-input"
                    aria-label={presetAction.kind === 'duplicate' ? 'New preset name' : 'Rename preset to'}
                    value={presetAction.value}
                    autoFocus
                    disabled={anyBusy}
                    onChange={(e) => setPresetAction({ ...presetAction, value: e.target.value.toLowerCase() })}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter') submitPresetAction()
                      if (e.key === 'Escape') setPresetAction(null)
                    }}
                  />
                  <button className="btn btn-primary btn-sm" onClick={submitPresetAction} disabled={anyBusy || !NAME_RE.test(presetAction.value.trim())}>
                    {presetAction.kind === 'duplicate' ? 'Duplicate' : 'Rename'}
                  </button>
                  <button className="btn btn-ghost btn-sm" onClick={() => setPresetAction(null)} disabled={anyBusy}>Cancel</button>
                </>
              ) : (
                <>
                  <span className="envs-muted">
                    <code>{newPreset}</code> · {customPresets.includes(newPreset) ? 'custom preset' : 'built-in preset'}
                  </span>
                  <button type="button" className="envs-link" disabled={anyBusy} onClick={() => setPresetAction({ kind: 'duplicate', value: `${newPreset}-copy` })}>
                    Duplicate
                  </button>
                  {customPresets.includes(newPreset) && (
                    <>
                      <button type="button" className="envs-link" disabled={anyBusy} onClick={() => setPresetAction({ kind: 'rename', value: newPreset })}>
                        Rename
                      </button>
                      <button type="button" className="envs-link envs-link-danger" disabled={anyBusy} onClick={handleDeletePreset}>
                        Delete
                      </button>
                    </>
                  )}
                </>
              )}
            </div>
          )}
        </div>
        <label className="envs-check-row">
          <input
            type="checkbox"
            checked={copyProviders}
            disabled={anyBusy}
            onChange={(e) => { setCopyTouched(true); setCopyProviders(e.target.checked) }}
          />
          <span>
            <strong>Copy providers &amp; credentials from global</strong>
            <span className="field-help">
              Seeds this env with your global opencode providers (opencode.json) and logins (opencode auth), so its models work right away. The env keeps its own copy.
            </span>
          </span>
        </label>
        <div className="envs-create-foot">
          <button className="btn btn-primary" onClick={handleCreate} disabled={!canCreate}>
            {isBusy(`create:${nameTrim}`) ? 'Creating…' : 'Create environment'}
          </button>
        </div>
      </section>

      {envs.length === 0 ? (
        <section className="card card-pad envs-empty">
          <p>No environments yet. Create one above to get an isolated opencode setup.</p>
        </section>
      ) : (
        <div className="envs-grid">
          {envs.map((env) => {
            const open = !!expanded[env.name]
            const st = details[env.name]
            return (
              <section key={env.name} className={`card envs-card ${env.running ? 'is-running' : ''}`}>
                <div className="envs-card-head">
                  <span className={`envs-dot ${env.running ? 'on' : 'off'}`} aria-hidden />
                  <div className="envs-card-title">
                    <h2>{env.name}</h2>
                    <div className="envs-meta">
                      <code>{env.url}</code>
                      {env.created_at && <span>created {formatDate(env.created_at)}</span>}
                    </div>
                  </div>
                  <span className={`envs-badge ${env.running ? 'on' : 'off'}`}>
                    {env.running ? 'Running' : 'Stopped'}
                  </span>
                </div>

                <div className="envs-card-body">
                  <div className="field envs-preset-field">
                    <label className="field-label" htmlFor={`preset-${env.name}`}>Preset</label>
                    <select
                      id={`preset-${env.name}`}
                      className="select"
                      value={env.preset}
                      disabled={anyBusy}
                      onChange={(e) => handlePresetChange(env, e.target.value)}
                    >
                      {[env.preset, ...presets.filter((p) => p !== env.preset)].map((p) => (
                        <option key={p} value={p}>{p}</option>
                      ))}
                    </select>
                    {presetDescriptions[env.preset] && <span className="field-help">{presetDescriptions[env.preset]}</span>}
                  </div>

                  <div className="envs-actions">
                    <button
                      className={`btn btn-sm ${env.running ? 'btn-ghost' : 'btn-primary'}`}
                      onClick={() => handleStartStop(env)}
                      disabled={anyBusy}
                    >
                      {isBusy(`start:${env.name}`) ? 'Starting…' : isBusy(`stop:${env.name}`) ? 'Stopping…' : env.running ? 'Stop' : 'Start'}
                    </button>
                    <button
                      className={`btn btn-sm ${pendingApply[env.name] ? 'btn-primary' : 'btn-ghost'}`}
                      onClick={() => handleApply(env.name)}
                      disabled={anyBusy}
                      title="Install agents, skills and MCP servers into this env"
                    >
                      {isBusy(`apply:${env.name}`) ? 'Applying…' : 'Apply'}
                    </button>
                    <button
                      className="btn btn-ghost btn-sm"
                      title={`Edit ${env.name} model, agents and providers in Settings`}
                      onClick={() => { setConfigProfileScope(env.name); navigate('/settings') }}
                      disabled={anyBusy}
                    >
                      Settings
                    </button>
                    <button className="btn btn-ghost btn-sm" onClick={() => toggleExpanded(env.name)} aria-expanded={open}>
                      {open ? 'Hide content' : 'Content & health'}
                    </button>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleImportProviders(env.name)}
                      disabled={anyBusy}
                      title="Copy your global opencode providers and credentials into this env (keeps what it already has)"
                    >
                      {isBusy(`import:${env.name}`) ? 'Importing…' : 'Import providers'}
                    </button>
                    <button
                      className="btn btn-ghost btn-sm"
                      onClick={() => handleImportAdo(env.name)}
                      disabled={anyBusy}
                      title="Copy your global ado CLI profiles into this env (keeps what it already has)"
                    >
                      {isBusy(`import-ado:${env.name}`) ? 'Importing…' : 'Import ado'}
                    </button>
                    <span className="envs-actions-spacer" />
                    <button className="btn btn-ghost btn-sm envs-delete" onClick={() => handleDelete(env.name)} disabled={anyBusy}>
                      {isBusy(`delete:${env.name}`) ? 'Deleting…' : 'Delete'}
                    </button>
                  </div>
                </div>

                <EnvCommands name={env.name} />

                {pendingApply[env.name] && (
                  <div className="envs-pending">
                    <span>Content changed. Run <strong>Apply</strong> to install it into this environment.</span>
                    <button className="btn btn-primary btn-sm" onClick={() => handleApply(env.name)} disabled={anyBusy}>Apply now</button>
                  </div>
                )}

                {open && (
                  <div className="envs-expand">
                    {st ? (
                      <>
                        <EnvContentEditor
                          env={env}
                          status={st}
                          catalog={catalog}
                          disabled={anyBusy}
                          onSaved={async () => {
                            setPendingApply((p) => ({ ...p, [env.name]: true }))
                            await refreshDetails(env.name)
                          }}
                          onError={setError}
                          onPresetSaved={async (name) => {
                            await load()
                            setNewPreset(name)
                          }}
                        />
                        <EnvHealth
                          status={st}
                          logs={logs[env.name]}
                          output={outputs[env.name]}
                          onRefresh={() => void refreshDetails(env.name)}
                        />
                      </>
                    ) : (
                      <p className="envs-muted">Loading…</p>
                    )}
                  </div>
                )}
              </section>
            )
          })}
        </div>
      )}
    </div>
  )
}

// ---- Terminal commands -----------------------------------------------------

// Mirrors cmd/ywai/env.go: `ywai <env>` is a registered shortcut that starts
// the env service if needed, then opens the TUI (no args) or runs a headless
// `opencode2 run` with the prompt.
function envCommands(name: string) {
  return [
    { label: 'Open TUI', cmd: `ywai ${name}`, help: 'Interactive opencode inside this env' },
    { label: 'Run a prompt', cmd: `ywai ${name} "describe the task here"`, help: 'Headless run, prints the answer and exits' },
    { label: 'Model & agent', cmd: `ywai ${name} "describe the task here" --model opencode-go/glm-5.3-flash --agent ask --auto`, help: 'Pick model and agent for this run; --auto approves permissions so it never waits' },
    { label: 'Run a workflow', cmd: `ywai ${name} <workflow> "free text"`, help: 'Runs an exported workflow with its orchestrator agent (same as the Run button in Workflows)' },
    { label: 'Apply', cmd: `ywai install --profile ${name} --agent opencode`, help: 'Same as the Apply button' },
    { label: 'Status', cmd: `ywai env status ${name}`, help: 'Service, database and log paths' },
  ]
}

function EnvCommands({ name }: { name: string }) {
  const [copied, setCopied] = useState<string | null>(null)
  const [open, setOpen] = useState(false)
  const cmds = envCommands(name)

  async function copy(cmd: string) {
    try {
      await navigator.clipboard.writeText(cmd)
    } catch {
      // Clipboard API needs a secure context; fall back to a hidden textarea.
      const ta = document.createElement('textarea')
      ta.value = cmd
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      ta.remove()
    }
    setCopied(cmd)
    window.setTimeout(() => setCopied((c) => (c === cmd ? null : c)), 1500)
  }

  const shown = open ? cmds : cmds.slice(0, 2)
  return (
    <div className="envs-cmds">
      <div className="envs-cmds-head">
        <span className="envs-stat-label">Run in terminal</span>
        <button type="button" className="envs-link" onClick={() => setOpen((v) => !v)}>
          {open ? 'Fewer commands' : 'More commands'}
        </button>
      </div>
      {shown.map((c) => (
        <div key={c.cmd} className="envs-cmd">
          <span className="envs-cmd-label" title={c.help}>{c.label}</span>
          <code className="envs-cmd-code">{c.cmd}</code>
          <button
            type="button"
            className={`btn btn-ghost btn-sm envs-cmd-copy ${copied === c.cmd ? 'is-copied' : ''}`}
            onClick={() => void copy(c.cmd)}
            aria-label={`Copy: ${c.cmd}`}
          >
            {copied === c.cmd ? 'Copied ✓' : 'Copy'}
          </button>
        </div>
      ))}
    </div>
  )
}

// ---- Content editor --------------------------------------------------------

type Mode = 'inherit' | 'custom' | 'none'

interface ChipOption {
  value: string
  label: string
  hint?: string
  // Category heading; options without one render as a flat chip list.
  group?: string
}

// Category display order; unknown categories sort alphabetically after
// these, with "other" always last.
const GROUP_ORDER = ['review', 'testing', 'qa', 'frontend', 'architecture', 'debugging', 'devops', 'git', 'planning', 'delegation', 'communication', 'meta']

function groupOptions(options: ChipOption[]): [string, ChipOption[]][] {
  if (!options.some((o) => o.group)) return [['', options]]
  const map = new Map<string, ChipOption[]>()
  for (const o of options) {
    const g = o.group || 'other'
    if (!map.has(g)) map.set(g, [])
    map.get(g)!.push(o)
  }
  const rank = (g: string) => (g === 'other' ? 1e9 : GROUP_ORDER.includes(g) ? GROUP_ORDER.indexOf(g) : 1e6)
  return [...map.entries()].sort(([a], [b]) => rank(a) - rank(b) || a.localeCompare(b))
}

function specList(spec: Record<string, unknown> | undefined, key: string): string[] {
  const v = spec?.[key]
  return Array.isArray(v) ? v.filter((s): s is string => typeof s === 'string' && s.trim() !== '') : []
}

function EnvContentEditor({ env, status, catalog, disabled, onSaved, onError, onPresetSaved }: {
  env: EnvProfile
  status: EnvStatus
  catalog: EnvCatalog
  disabled: boolean
  onSaved: () => Promise<void>
  onError: (msg: string) => void
  onPresetSaved: (name: string) => Promise<void>
}) {
  const [saving, setSaving] = useState<EnvSpecSection | null>(null)
  // "Save as preset" inline form: null = closed, string = the name being typed.
  const [saveAs, setSaveAs] = useState<string | null>(null)
  const [savedPreset, setSavedPreset] = useState<string | null>(null)

  async function saveAsPreset() {
    const name = (saveAs ?? '').trim()
    if (!NAME_RE.test(name)) return
    try {
      await envsApi.copyPreset({ env: env.name, name, description: `Saved from env ${env.name} (based on ${env.preset}).` })
      setSaveAs(null)
      setSavedPreset(name)
      await onPresetSaved(name)
    } catch (e) {
      onError(errText(e))
    }
  }
  const bare = status.preset_spec?.bare === true

  async function save(key: EnvSpecSection, next: string[] | null) {
    setSaving(key)
    try {
      await envsApi.patch(env.name, next === null ? { reset: [key] } : { [key]: next })
      await onSaved()
    } catch (e) {
      onError(errText(e))
    } finally {
      setSaving(null)
    }
  }

  const sections: {
    key: EnvSpecSection
    title: string
    help: string
    options: ChipOption[]
    locked?: string[]
    allowNone: boolean
    allLabel: string
  }[] = [
    {
      key: 'groups',
      title: 'Agent groups',
      help: 'Which agent teams get installed. Core is always included.',
      options: catalog.groups.map((g) => ({ value: g.name, label: g.name, hint: `${g.agents} agents · ${g.description}` })),
      locked: ['core'],
      allowNone: false,
      allLabel: 'Installer default',
    },
    {
      key: 'skills',
      title: 'Skills',
      help: 'ywai extra skills copied into this env.',
      options: catalog.skills.map((sk) => ({
        value: sk.name,
        label: sk.name,
        hint: [sk.description, sk.tags.length ? `#${sk.tags.join(' #')}` : ''].filter(Boolean).join('\n'),
        group: sk.tags[0] ?? 'other',
      })),
      allowNone: true,
      allLabel: 'All skills',
    },
    {
      key: 'mcp',
      title: 'MCP servers',
      help: 'Optional MCP servers. Engram memory and plugins always install.',
      options: catalog.mcp.map((m) => ({ value: m, label: m })),
      allowNone: true,
      allLabel: 'All servers',
    },
  ]

  return (
    <div className="envs-editor">
      <div className="envs-section-head envs-section-head-row">
        <h3>Content</h3>
        {saveAs === null ? (
          <button
            type="button"
            className="btn btn-ghost btn-sm"
            onClick={() => { setSavedPreset(null); setSaveAs(`${env.name}-preset`) }}
            disabled={disabled}
            title="Create a reusable preset from this env's current content"
          >
            {savedPreset ? `Saved as ${savedPreset} ✓` : 'Save as preset'}
          </button>
        ) : (
          <div className="envs-preset-tools">
            <input
              className="input envs-preset-input"
              aria-label="New preset name"
              value={saveAs}
              autoFocus
              onChange={(e) => setSaveAs(e.target.value.toLowerCase())}
              onKeyDown={(e) => {
                if (e.key === 'Enter') void saveAsPreset()
                if (e.key === 'Escape') setSaveAs(null)
              }}
            />
            <button className="btn btn-primary btn-sm" onClick={() => void saveAsPreset()} disabled={!NAME_RE.test(saveAs.trim())}>Save</button>
            <button className="btn btn-ghost btn-sm" onClick={() => setSaveAs(null)}>Cancel</button>
          </div>
        )}
      </div>
      <div className="envs-section-head">
        <p className="envs-muted">
          {bare
            ? 'This preset is bare: nothing is installed regardless of the lists below.'
            : 'Click any item to include or exclude it. Changes save immediately and take effect on the next Apply.'}
        </p>
      </div>
      {sections.map((s) => {
        const override = status.overrides?.[s.key]
        const presetList = specList(status.preset_spec, s.key)
        const mode: Mode = override == null ? 'inherit' : override.length === 0 ? 'none' : 'custom'
        const inheritLabel = presetList.length ? `Preset (${presetList.length})` : s.allLabel
        const effective = new Set(override ?? (presetList.length ? presetList : s.options.map((o) => o.value)))
        for (const l of s.locked ?? []) effective.add(l)
        // Keep unknown saved entries visible so they can be removed.
        const options = [...s.options]
        for (const v of override ?? []) {
          if (!options.some((o) => o.value === v)) options.push({ value: v, label: v, hint: 'not found in catalog' })
        }
        const busyHere = disabled || saving !== null

        const setMode = (m: Mode) => {
          if (m === mode) return
          if (m === 'inherit') return void save(s.key, null)
          if (m === 'none') return void save(s.key, [])
          const start = [...effective]
          void save(s.key, start.length ? start : options.slice(0, 1).map((o) => o.value))
        }
        // setItems includes/excludes several values at once (a whole category)
        // and saves the result as a Custom list.
        const setItems = (values: string[], on: boolean) => {
          const next = new Set(effective)
          for (const v of values) {
            if (on) next.add(v)
            else next.delete(v)
          }
          for (const l of s.locked ?? []) next.add(l)
          const list = options.map((o) => o.value).filter((x) => next.has(x))
          if (list.length === 0 && s.allowNone) return void save(s.key, [])
          if (list.length === 0) return
          void save(s.key, list)
        }
        const toggle = (v: string) => setItems([v], !effective.has(v))
        const renderChip = (o: ChipOption) => {
          const on = effective.has(o.value)
          const locked = s.locked?.includes(o.value)
          return (
            <button
              key={o.value}
              type="button"
              aria-pressed={on}
              title={locked ? 'Always installed' : o.hint}
              className={`envs-chip ${on ? 'is-on' : ''}`}
              // Toggling from Preset/All switches the list to Custom
              // with this item flipped — no mode click needed first.
              onClick={() => (!locked ? toggle(o.value) : undefined)}
              disabled={busyHere || locked}
            >
              <span className="envs-chip-box" aria-hidden>{on ? '✓' : ''}</span>
              {o.label}
            </button>
          )
        }

        return (
          <div key={s.key} className={`envs-list ${bare ? 'is-bare' : ''}`}>
            <div className="envs-list-head">
              <div>
                <h4>{s.title}{saving === s.key && <span className="envs-saving">saving…</span>}</h4>
                <p className="field-help">{s.help}</p>
              </div>
              <div className="toggle-seg" role="radiogroup" aria-label={`${s.title} mode`}>
                <button type="button" role="radio" aria-checked={mode === 'inherit'} className={`seg-btn ${mode === 'inherit' ? 'active' : ''}`} onClick={() => setMode('inherit')} disabled={busyHere}>{inheritLabel}</button>
                <button type="button" role="radio" aria-checked={mode === 'custom'} className={`seg-btn ${mode === 'custom' ? 'active' : ''}`} onClick={() => setMode('custom')} disabled={busyHere || options.length === 0}>Custom</button>
                {s.allowNone && (
                  <button type="button" role="radio" aria-checked={mode === 'none'} className={`seg-btn ${mode === 'none' ? 'active' : ''}`} onClick={() => setMode('none')} disabled={busyHere}>None</button>
                )}
              </div>
            </div>
            {mode === 'none' ? (
              <p className="envs-muted envs-list-empty">Nothing from this list is installed.</p>
            ) : options.length === 0 ? (
              <p className="envs-muted envs-list-empty">No options found in the catalog.</p>
            ) : (
              <div className="envs-chip-groups">
                {groupOptions(options).map(([group, opts]) => {
                  if (!group) return <div key="_" className="envs-chips">{opts.map(renderChip)}</div>
                  const onCount = opts.filter((o) => effective.has(o.value)).length
                  const allOn = onCount === opts.length
                  return (
                    <div key={group} className="envs-chip-group">
                      <button
                        type="button"
                        className={`envs-chip-group-label ${onCount === 0 ? 'is-off' : ''}`}
                        onClick={() => setItems(opts.map((o) => o.value), !allOn)}
                        disabled={busyHere}
                        title={allOn ? `Exclude all ${group} skills` : `Include all ${group} skills`}
                      >
                        {group}
                        <span className="envs-chip-group-count">{onCount}/{opts.length}</span>
                      </button>
                      <div className="envs-chips">{opts.map(renderChip)}</div>
                    </div>
                  )
                })}
              </div>
            )}
          </div>
        )
      })}
    </div>
  )
}

// ---- Health / logs ---------------------------------------------------------

function EnvHealth({ status, logs, output, onRefresh }: {
  status: EnvStatus
  logs?: EnvLogs
  output?: { ok: boolean; text: string }
  onRefresh: () => void
}) {
  const failing = status.doctor.checks.filter((c) => !c.ok)
  return (
    <div className="envs-health">
      <div className="envs-section-head envs-section-head-row">
        <h3>Health</h3>
        <button className="btn btn-ghost btn-sm" onClick={onRefresh}>Refresh</button>
      </div>
      <div className="envs-stats">
        <Stat label="Service" value={status.service.running ? 'Running' : 'Stopped'} tone={status.service.running ? 'ok' : undefined} sub={status.service.url} />
        <Stat label="Memory DB" value={status.db.exists ? formatBytes(status.db.size_bytes) : 'Not created'} sub={status.db.exists ? status.db.path : 'Created on first start'} />
        <Stat label="Auth" value={status.auth.configured ? 'Configured' : 'Pending'} tone={status.auth.configured ? 'ok' : undefined} sub={status.auth.hint} />
        <Stat label="Doctor" value={status.doctor.ok ? 'All checks pass' : `${failing.length} issue${failing.length === 1 ? '' : 's'}`} tone={status.doctor.ok ? 'ok' : 'bad'} sub={status.doctor.ok ? undefined : failing.map((c) => c.message).join(' · ')} />
      </div>
      {output && (
        <details className="envs-log" open>
          <summary>Last apply <span className={`envs-badge ${output.ok ? 'on' : 'bad'}`}>{output.ok ? 'ok' : 'failed'}</span></summary>
          <pre>{output.text || '(no output)'}</pre>
        </details>
      )}
      {logs && (
        <details className="envs-log">
          <summary>Server log <span className="envs-muted">{logs.truncated ? `last ${logs.returned} of ${logs.total} lines` : `${logs.total} lines`}</span></summary>
          <pre>{logs.lines.join('\n') || '(empty)'}</pre>
        </details>
      )}
    </div>
  )
}

function Stat({ label, value, sub, tone }: { label: string; value: string; sub?: string; tone?: 'ok' | 'bad' }) {
  return (
    <div className="envs-stat">
      <span className="envs-stat-label">{label}</span>
      <span className={`envs-stat-value ${tone ?? ''}`}>{value}</span>
      {sub && <span className="envs-stat-sub" title={sub}>{sub}</span>}
    </div>
  )
}
