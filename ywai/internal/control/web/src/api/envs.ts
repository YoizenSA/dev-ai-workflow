// API client for isolated env profiles (GET/POST /api/envs and friends).
// Backend: ywai/internal/control/envprofiles.go.

export interface EnvProfile {
  name: string
  preset: string
  port: number
  created_at: string
  running: boolean
  url: string
}

export interface EnvCatalog {
  groups: { name: string; description: string; agents: number }[]
  // tags[0] is the skill's category (from its .ywai-extra marker).
  skills: { name: string; description: string; tags: string[] }[]
  mcp: string[]
}

export interface EnvListResponse {
  envs: EnvProfile[]
  presets: string[]
  preset_descriptions?: Record<string, string>
  // User-created presets (editable); every other preset is built in.
  custom_presets?: string[]
  // Default of the "copy global providers" check per preset.
  preset_copy_providers?: Record<string, boolean>
  catalog?: EnvCatalog
}

export type EnvSpecSection = 'groups' | 'skills' | 'mcp'

export interface EnvCopiedProviders {
  providers: string[] | null
  // opencode logins (integration ids) copied from the global opencode.db.
  credentials: string[] | null
  // Why a step was skipped, e.g. the env database does not exist yet.
  note?: string
}

export interface EnvDoctorCheck {
  name: string
  ok: boolean
  message: string
}

// null/absent = inherit the preset; [] = install none (groups: core only).
export interface EnvSpecOverrides {
  groups?: string[] | null
  skills?: string[] | null
  mcp?: string[] | null
}

export interface EnvStatus {
  name: string
  preset: string
  port: number
  url: string
  spec: Record<string, unknown>
  preset_spec?: Record<string, unknown>
  overrides: EnvSpecOverrides | null
  service: { running: boolean; port: number; url: string }
  db: { path: string; exists: boolean; size_bytes: number }
  serverLog: { path: string; exists: boolean; size_bytes: number }
  auth: { configured: boolean; hint: string }
  doctor: { ok: boolean; checks: EnvDoctorCheck[] }
}

export interface EnvLogs {
  name: string
  path: string
  exists: boolean
  lines: string[]
  returned: number
  total: number
  truncated: boolean
}

export interface EnvApplyResult {
  name: string
  ok: boolean
  output: string
}

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  })
  if (!res.ok) {
    const body = await res.text().catch(() => res.statusText)
    throw new Error(`${res.status}: ${body}`)
  }
  return res.json() as Promise<T>
}

export const envsApi = {
  list: () => request<EnvListResponse>('/api/envs'),
  create: (name: string, preset?: string, copyProviders?: boolean) =>
    request<{ profile: EnvProfile; copied?: EnvCopiedProviders; copy_error?: string }>(`/api/envs`, {
      method: 'POST',
      body: JSON.stringify({ name, preset, copy_providers: copyProviders }),
    }),
  // Merge the global providers + credentials into an existing env.
  importProviders: (name: string) =>
    request<{ name: string; copied: EnvCopiedProviders }>(`/api/envs/${encodeURIComponent(name)}/import-providers`, {
      method: 'POST',
    }),
  updatePreset: (name: string, preset: string) =>
    request<{ profile: EnvProfile }>(`/api/envs/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify({ preset }),
    }),
  patch: (name: string, body: { preset?: string; groups?: string[]; skills?: string[]; mcp?: string[]; reset?: EnvSpecSection[] }) =>
    request<{ profile: EnvProfile }>(`/api/envs/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    }),
  remove: (name: string) =>
    request<{ status: string }>(`/api/envs/${encodeURIComponent(name)}`, {
      method: 'DELETE',
    }),
  apply: (name: string) =>
    request<EnvApplyResult>(`/api/envs/${encodeURIComponent(name)}/apply`, {
      method: 'POST',
    }),
  start: (name: string) =>
    request<{ name: string; running: boolean; url: string }>(`/api/envs/${encodeURIComponent(name)}/start`, {
      method: 'POST',
    }),
  stop: (name: string) =>
    request<{ name: string; running: boolean }>(`/api/envs/${encodeURIComponent(name)}/stop`, {
      method: 'POST',
    }),
  // User presets. With env, the copy captures that env's customized content.
  copyPreset: (body: { name: string; from?: string; env?: string; description?: string }) =>
    request<{ name: string }>('/api/env-presets', { method: 'POST', body: JSON.stringify(body) }),
  patchPreset: (name: string, body: { name?: string; description?: string }) =>
    request<{ name: string }>(`/api/env-presets/${encodeURIComponent(name)}`, { method: 'PATCH', body: JSON.stringify(body) }),
  deletePreset: (name: string) =>
    request<{ status: string }>(`/api/env-presets/${encodeURIComponent(name)}`, { method: 'DELETE' }),
  status: (name: string) =>
    request<EnvStatus>(`/api/envs/${encodeURIComponent(name)}/status`),
  logs: (name: string, lines = 200) =>
    request<EnvLogs>(`/api/envs/${encodeURIComponent(name)}/logs?lines=${lines}`),
}
