import { clearSession, saveSession, session } from './auth.js'
import * as mock from './mock.js'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'
const BIN_ID = 'bin-27'

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

export function startScan() {
  if (USE_MOCKS) return mock.startScan()
  return request('/api/scan-requests', { method: 'POST', body: JSON.stringify({ bin_id: BIN_ID }) })
}

export function getScanRequest(requestId) {
  if (USE_MOCKS) return mock.getScanRequest(requestId)
  return request(`/api/scan-requests/${encodeURIComponent(requestId)}`)
}

export function getRecentScans(limit = 20) {
  if (USE_MOCKS) return mock.getRecentScans(limit)
  return request(`/api/scans?limit=${limit}`)
}

export function getStats() {
  if (USE_MOCKS) return mock.getStats()
  return request('/api/stats')
}
