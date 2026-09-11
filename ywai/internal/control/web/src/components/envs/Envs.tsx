import { Fragment, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { envsApi, type EnvProfile, type EnvStatus, type EnvLogs } from '../../api/envs'
import { setConfigProfileScope } from '../../api/client'
import './Envs.css'

function formatBytes(n: number): string {
  if (!n) return '0 B'
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(1)} MB`
}

function formatDate(iso: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString()
}

export default function Envs() {
  const navigate = useNavigate()
  const [envs, setEnvs] = useState<EnvProfile[]>([])
  const [presets, setPresets] = useState<string[]>([])
  const [presetDescriptions, setPresetDescriptions] = useState<Record<string, string>>({})
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newPreset, setNewPreset] = useState('')
  const [busy, setBusy] = useState<string | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})
  const [details, setDetails] = useState<Record<string, EnvStatus>>({})
  const [logs, setLogs] = useState<Record<string, EnvLogs>>({})
  const [outputs, setOutputs] = useState<Record<string, string>>({})

  const refresh = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const res = await envsApi.list()
      setEnvs(res.envs ?? [])
      setPresets(res.presets ?? [])
      setPresetDescriptions(res.preset_descriptions ?? {})
      if (!newPreset && res.presets?.length) setNewPreset(res.presets[0])
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [newPreset])

  useEffect(() => {
    void refresh()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const fail = (e: unknown) => setError(e instanceof Error ? e.message : String(e))
  const isBusy = (key: string) => busy === key
  const anyBusy = busy !== null

  async function afterMutation(name: string) {
    void name
    try {
      const res = await envsApi.list()
      setEnvs(res.envs ?? [])
      if (res.presets?.length) {
        setPresets(res.presets)
        setPresetDescriptions(res.preset_descriptions ?? {})
      }
    } catch (e) {
      fail(e)
    }
  }

  async function handleCreate() {
    const name = newName.trim()
    if (!name) return
    setBusy(`create:${name}`)
    try {
      await envsApi.create(name, newPreset || undefined)
      setNewName('')
      await afterMutation(name)
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  async function handlePresetChange(name: string, preset: string) {
    setBusy(`preset:${name}`)
    try {
      await envsApi.updatePreset(name, preset)
      await afterMutation(name)
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  async function handleDelete(name: string) {
    setBusy(`delete:${name}`)
    try {
      if (!window.confirm(`Delete env "${name}"? Its service stops and its directory is removed.`)) return
      await envsApi.remove(name)
      setEnvs((prev) => prev.filter((e) => e.name !== name))
    } catch (e) {
      fail(e)
      await afterMutation(name)
    } finally {
      setBusy(null)
    }
  }

  async function handleStartStop(env: EnvProfile) {
    const action = env.running ? 'stop' : 'start'
    setBusy(`${action}:${env.name}`)
    try {
      if (env.running) {
        await envsApi.stop(env.name)
      } else {
        await envsApi.start(env.name)
      }
      await afterMutation(env.name)
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  async function handleApply(name: string) {
    setBusy(`apply:${name}`)
    try {
      const res = await envsApi.apply(name)
      setOutputs((prev) => ({ ...prev, [name]: res.output }))
      setExpanded((prev) => ({ ...prev, [name]: true }))
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  async function refreshDetails(name: string) {
    setBusy(`status:${name}`)
    try {
      const [st, lg] = await Promise.all([envsApi.status(name), envsApi.logs(name, 200)])
      setDetails((prev) => ({ ...prev, [name]: st }))
      setLogs((prev) => ({ ...prev, [name]: lg }))
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  async function toggleDetails(name: string) {
    const next = !expanded[name]
    setExpanded((prev) => ({ ...prev, [name]: next }))
    if (!next) return
    await refreshDetails(name)
  }

  async function handleLogs(name: string) {
    setBusy(`logs:${name}`)
    try {
      const lg = await envsApi.logs(name, 200)
      setLogs((prev) => ({ ...prev, [name]: lg }))
      setExpanded((prev) => ({ ...prev, [name]: true }))
    } catch (e) {
      fail(e)
    } finally {
      setBusy(null)
    }
  }

  if (loading) return <div className="envs"><p>Loading envs…</p></div>

  return (
    <div className="envs">
      <header className="page-header">
        <div className="page-heading">
          <span className="page-eyebrow">Isolation</span>
          <h1 className="page-title">Envs</h1>
          <p className="page-subtitle">Isolated opencode environments, one profile per lane. Edit preset, service and content per env.</p>
        </div>
        <button className="btn btn-ghost btn-sm envs-refresh" onClick={() => void refresh()} disabled={anyBusy}>Refresh</button>
      </header>

      {error && <div className="envs-error" role="alert">Error: {error}</div>}

      <section className="card card-pad envs-create">
        <h2>New env</h2>
        <div className="envs-create-row">
          <input
            aria-label="env name"
            placeholder="name, e.g. dev"
            value={newName}
            disabled={anyBusy}
            onChange={(e) => setNewName(e.target.value)}
            onKeyDown={(e) => { if (e.key === 'Enter') void handleCreate() }}
          />
          <select
            aria-label="preset"
            value={newPreset}
            disabled={anyBusy}
            onChange={(e) => setNewPreset(e.target.value)}
          >
            {presets.map((p) => (
              <option key={p} value={p}>{p}</option>
            ))}
          </select>
          <button className="btn btn-primary btn-sm" onClick={() => void handleCreate()} disabled={anyBusy || !newName.trim()}>
            {isBusy(`create:${newName.trim()}`) ? 'Creating…' : 'Create'}
          </button>
        </div>
        {newPreset && presetDescriptions[newPreset] && (
          <p className="envs-hint">{newPreset}: {presetDescriptions[newPreset]}</p>
        )}
      </section>

      {envs.length === 0 ? (
        <section className="card card-pad">
          <p className="envs-empty">No envs yet. Create one above — each gets its own config, database and service.</p>
        </section>
      ) : (
        <div className="envs-grid">
          {envs.map((env) => (
            <Fragment key={env.name}>
              <section className="card card-pad envs-card">
                <div className="envs-card-head">
                  <span className={`envs-dot ${env.running ? 'on' : 'off'}`} title={env.running ? 'running' : 'stopped'} />
                  <div>
                    <h2 className="envs-card-name">{env.name}</h2>
                    <div className="envs-muted">{env.url} · port {env.port}{env.created_at ? ` · since ${formatDate(env.created_at)}` : ''}</div>
                  </div>
                  <span className={`envs-badge ${env.running ? 'on' : 'off'}`}>
                    {env.running ? 'running' : 'stopped'}
                  </span>
                </div>

                <div className="envs-field">
                  <label htmlFor={`preset-${env.name}`}>Preset</label>
                  <select
                    id={`preset-${env.name}`}
                    value={env.preset}
                    disabled={anyBusy}
                    onChange={(e) => void handlePresetChange(env.name, e.target.value)}
                  >
                    {[env.preset, ...presets.filter((p) => p !== env.preset)].map((p) => (
                      <option key={p} value={p}>{p}</option>
                    ))}
                  </select>
                  {presetDescriptions[env.preset] && (
                    <p className="envs-hint">{presetDescriptions[env.preset]}</p>
                  )}
                </div>

                <div className="envs-actions">
                  <button
                    className={`btn btn-sm ${env.running ? 'btn-ghost' : 'btn-primary'}`}
                    onClick={() => void handleStartStop(env)}
                    disabled={anyBusy}
                  >
                    {isBusy(`start:${env.name}`) || isBusy(`stop:${env.name}`)
                      ? 'Working…'
                      : env.running ? 'Stop' : 'Start'}
                  </button>
                  <button className="btn btn-ghost btn-sm" onClick={() => void toggleDetails(env.name)} disabled={anyBusy}>
                    {expanded[env.name] ? 'Hide' : 'Details'}
                  </button>
                  <button className="btn btn-ghost btn-sm" onClick={() => void handleApply(env.name)} disabled={anyBusy}>
                    {isBusy(`apply:${env.name}`) ? 'Applying…' : 'Apply'}
                  </button>
                  <button className="btn btn-ghost btn-sm" onClick={() => void handleLogs(env.name)} disabled={anyBusy}>Logs</button>
                  <button
                    className="btn btn-ghost btn-sm"
                    title={`Edit ${env.name} config in Settings`}
                    onClick={() => { setConfigProfileScope(env.name); navigate('/settings') }}
                    disabled={anyBusy}
                  >
                    Edit settings
                  </button>
                  <button className="btn btn-danger btn-sm" onClick={() => void handleDelete(env.name)} disabled={anyBusy}>Delete</button>
                </div>

                {expanded[env.name] && (
                  <>
                    <EnvContentEditor
                      env={env}
                      status={details[env.name]}
                      disabled={anyBusy}
                      onSaved={() => void (async () => { await afterMutation(env.name); await refreshDetails(env.name) })()}
                    />
                    <EnvDetails
                      status={details[env.name]}
                      logs={logs[env.name]}
                      output={outputs[env.name]}
                    />
                  </>
                )}
              </section>
            </Fragment>
          ))}
        </div>
      )}
    </div>
  )
}

type SpecSection = 'groups' | 'skills' | 'mcp'

function specStringList(spec: Record<string, unknown> | undefined, key: string): string[] {
  if (!spec) return []
  const v = spec[key]
  if (!Array.isArray(v)) return []
  return v.filter((s): s is string => typeof s === 'string' && s.trim() !== '')
}

function EnvContentEditor({ env, status, disabled, onSaved }: {
  env: EnvProfile
  status?: EnvStatus
  disabled: boolean
  onSaved: () => void
}) {
  const [lists, setLists] = useState<Record<SpecSection, string[]>>({ groups: [], skills: [], mcp: [] })
  const [loaded, setLoaded] = useState(false)
  const [adds, setAdds] = useState<Record<SpecSection, string>>({ groups: '', skills: '', mcp: '' })
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const statusKey = status ? `${env.name}:${env.preset}:${JSON.stringify(status.overrides ?? null)}` : ''
  useEffect(() => {
    if (!status) return
    const spec = status.spec ?? {}
    const ov = status.overrides ?? {}
    setLists({
      groups: ov.groups ?? specStringList(spec, 'groups'),
      skills: ov.skills ?? specStringList(spec, 'skills'),
      mcp: ov.mcp ?? specStringList(spec, 'mcp'),
    })
    setLoaded(true)
    setErr(null)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [statusKey])

  if (!status) return <p>Loading editor…</p>
  const customized = status.overrides != null
  const sections: { key: SpecSection; title: string }[] = [
    { key: 'groups', title: 'Agent groups' },
    { key: 'skills', title: 'Skills' },
    { key: 'mcp', title: 'MCP servers' },
  ]

  async function save(next: Record<SpecSection, string[]>) {
    setSaving(true)
    setErr(null)
    try {
      await envsApi.patch(env.name, { groups: next.groups, skills: next.skills, mcp: next.mcp })
      onSaved()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  async function reset() {
    setSaving(true)
    setErr(null)
    try {
      await envsApi.patch(env.name, { groups: null as unknown as string[], skills: null as unknown as string[], mcp: null as unknown as string[] })
      onSaved()
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  if (!loaded) return <p>Loading editor…</p>
  return (
    <div className="envs-editor">
      <h3>
        Content {customized
          ? <span className="envs-badge on">customized</span>
          : <span className="envs-badge off">preset defaults</span>}
      </h3>
      <p className="envs-muted">Uncheck to drop from this env, or add extra entries. Takes effect on next Apply.</p>
      {err && <div className="envs-error" role="alert">Error: {err}</div>}
      {sections.map(({ key, title }) => (
        <div key={key} className="envs-editor-section">
          <h4>{title}</h4>
          {lists[key].length === 0 && <p className="envs-muted">None — inherits nothing. Add below or Reset.</p>}
          {lists[key].map((item) => (
            <label key={item} className="envs-check">
              <input
                type="checkbox"
                checked
                disabled={disabled || saving}
                onChange={() => {
                  const next = { ...lists, [key]: lists[key].filter((s) => s !== item) }
                  setLists(next)
                  void save(next)
                }}
              />
              {item}
            </label>
          ))}
          <div className="envs-create-row">
            <input
              aria-label={`add ${key}`}
              placeholder={`add ${key.slice(0, -1)}…`}
              value={adds[key]}
              disabled={disabled || saving}
              onChange={(e) => setAdds((p) => ({ ...p, [key]: e.target.value }))}
              onKeyDown={(e) => {
                if (e.key !== 'Enter' || !adds[key].trim()) return
                const next = { ...lists, [key]: [...lists[key], adds[key].trim()] }
                setLists(next)
                setAdds((p) => ({ ...p, [key]: '' }))
                void save(next)
              }}
            />
          </div>
        </div>
      ))}
      <div className="envs-actions">
        <button className="btn btn-ghost btn-sm" onClick={() => void reset()} disabled={disabled || saving || !customized}>
          Reset to preset
        </button>
      </div>
    </div>
  )
}

function EnvDetails({ status, logs, output }: { status?: EnvStatus; logs?: EnvLogs; output?: string }) {
  if (!status && !logs && output === undefined) return <p>Loading details…</p>
  return (
    <div className="envs-details">
      {status && (
        <div className="envs-panels">
          <div>
            <h3>Service</h3>
            <p>{status.service.running ? `Running on ${status.service.url}` : 'Stopped'}</p>
            <p>Port {status.service.port}</p>
          </div>
          <div>
            <h3>Database</h3>
            <p>{status.db.exists ? `${formatBytes(status.db.size_bytes)}` : 'No database yet'}</p>
            <p className="envs-muted">{status.db.path}</p>
          </div>
          <div>
            <h3>Auth</h3>
            <p>{status.auth.hint}</p>
          </div>
          <div>
            <h3>Doctor {status.doctor.ok ? '✓' : '✗'}</h3>
            <ul>
              {status.doctor.checks.map((c) => (
                <li key={c.name} className={c.ok ? 'ok' : 'bad'}>
                  {c.ok ? '✓' : '✗'} {c.name}: {c.message}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}
      {output !== undefined && (
        <div>
          <h3>Apply output</h3>
          <pre className="envs-pre">{output || '(empty)'}</pre>
        </div>
      )}
      {logs && (
        <div>
          <h3>Server log {logs.truncated ? `(last ${logs.returned} of ${logs.total})` : `(${logs.total} lines)`}</h3>
          <pre className="envs-pre">{logs.lines.join('\n') || '(empty)'}</pre>
        </div>
      )}
    </div>
  )
}
