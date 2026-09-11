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

export interface EnvListResponse {
  envs: EnvProfile[]
  presets: string[]
  preset_descriptions?: Record<string, string>
}

export interface EnvDoctorCheck {
  name: string
  ok: boolean
  message: string
}

export interface EnvSpecOverrides {
  groups?: string[]
  skills?: string[]
  mcp?: string[]
}

export interface EnvStatus {
  name: string
  preset: string
  port: number
  url: string
  spec: Record<string, unknown>
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
  create: (name: string, preset?: string) =>
    request<{ profile: EnvProfile }>(`/api/envs`, {
      method: 'POST',
      body: JSON.stringify({ name, preset }),
    }),
  updatePreset: (name: string, preset: string) =>
    request<{ profile: EnvProfile }>(`/api/envs/${encodeURIComponent(name)}`, {
      method: 'PATCH',
      body: JSON.stringify({ preset }),
    }),
  patch: (name: string, body: { preset?: string; groups?: string[]; skills?: string[]; mcp?: string[] }) =>
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
  status: (name: string) =>
    request<EnvStatus>(`/api/envs/${encodeURIComponent(name)}/status`),
  logs: (name: string, lines = 200) =>
    request<EnvLogs>(`/api/envs/${encodeURIComponent(name)}/logs?lines=${lines}`),
}
