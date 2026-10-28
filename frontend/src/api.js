import { clearSession, saveSession, session } from './auth.js'
import * as mock from './mock.js'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

async function request(path, options = {}) {
  const token = session.value?.token
  const response = await fetch(path, {
    ...options,
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...options.headers,
    },
  })
  const data = response.status === 204 ? null : await response.json().catch(() => null)
  if (response.status === 401 && token) {
    clearSession()
  }
  if (!response.ok) {
    throw new Error(data?.error || `Request failed (${response.status})`)
  }
  return data
}

export function register(username, password) {
  if (USE_MOCKS) return mock.register(username, password)
  return request('/api/users/register', { method: 'POST', body: JSON.stringify({ username, password }) })
}

export async function login(username, password) {
  const data = USE_MOCKS
    ? await mock.login(username, password)
    : await request('/api/users/login', { method: 'POST', body: JSON.stringify({ username, password }) })
  saveSession({ token: data.token, expires_at: data.expires_at, username: username.trim().toLowerCase() })
}

export async function logout() {
  try {
    if (USE_MOCKS) await mock.logout()
    else await request('/api/users/logout', { method: 'POST' })
  } finally {
    clearSession()
  }
}

export function startScan(binId, qrToken) {
  if (USE_MOCKS) return mock.startScan(binId, qrToken, session.value?.username)
  return request('/api/scan-requests', {
    method: 'POST',
    body: JSON.stringify({ bin_id: binId, qr_token: qrToken }),
  })
}

export function getScanRequest(requestId, wait = 25) {
  if (USE_MOCKS) return mock.getScanRequest(requestId, wait)
  return request(`/api/scan-requests/${encodeURIComponent(requestId)}?wait=${wait}`, {
    signal: AbortSignal.timeout((wait + 5) * 1000),
  })
}

export async function waitForScanResult(requestId, timeoutMs = 30000) {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const wait = Math.min(25, Math.ceil((deadline - Date.now()) / 1000))
    const response = await getScanRequest(requestId, wait)
    if (response.status !== 'pending') return response
  }
  return { status: 'failed', reason: 'timeout' }
}

export function getMyScans({ limit = 20, before } = {}) {
  if (USE_MOCKS) return mock.getMyScans(session.value?.username, limit, before)
  const params = new URLSearchParams({ limit: String(limit) })
  if (before) params.set('before', before)
  return request(`/api/me/scans?${params}`)
}

export function getMyStreak() {
  if (USE_MOCKS) return mock.getMyStreak(session.value?.username)
  return request('/api/me/streak')
}

export function getMyStats({ from, to } = {}) {
  if (USE_MOCKS) return mock.getMyStats(session.value?.username, from, to)
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  return request(`/api/me/stats${params.size ? `?${params}` : ''}`)
}
