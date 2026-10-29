import { clearSession, saveSession, session } from './auth.js'
import { disconnectBin } from './bin.js'
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
  return request('/api/auth/register', { method: 'POST', body: JSON.stringify({ username, password }) })
}

export async function login(username, password) {
  const data = USE_MOCKS
    ? await mock.login(username, password)
    : await request('/api/auth/login', { method: 'POST', body: JSON.stringify({ username, password }) })
  saveSession({
    token: data.token,
    expires_at: data.expires_at,
    username: username.trim().toLowerCase(),
    role: data.role,
  })
}

export async function logout() {
  try {
    if (USE_MOCKS) await mock.logout()
    else await request('/api/auth/logout', { method: 'POST' })
  } finally {
    clearSession()
    disconnectBin()
  }
}

export function startScan(binId, code) {
  if (USE_MOCKS) return mock.startScan(binId, code, session.value?.username)
  return request('/api/scan-requests', {
    method: 'POST',
    body: JSON.stringify({ bin_id: binId, code }),
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

const dayKey = (date) => date.toLocaleDateString('sv-SE')

function fetchStats({ from, to } = {}) {
  if (USE_MOCKS) return mock.getMyStats(session.value?.username, from, to)
  const params = new URLSearchParams()
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  return request(`/api/me/stats${params.size ? `?${params}` : ''}`)
}

export async function getMyStats(range) {
  const { summary } = await fetchStats(range)
  return summary
}

export async function getMyStreak() {
  const today = new Date()
  const monday = new Date(today)
  monday.setDate(today.getDate() - ((today.getDay() + 6) % 7))

  const { summary, streak } = await fetchStats({ from: dayKey(monday), to: dayKey(today) })
  const activeDays = new Set(
    summary.per_day.filter((day) => (day.sorted ?? day.count) > 0).map((day) => day.date),
  )
  const week = Array.from({ length: 7 }, (_, i) => {
    const day = new Date(monday)
    day.setDate(monday.getDate() + i)
    return activeDays.has(dayKey(day))
  })

  return { current: streak.current, best: streak.longest, last_day: streak.last_sorted_date || null, week }
}

// Admin

export function getUsers() {
  if (USE_MOCKS) return mock.listUsers()
  return request('/api/admin/users')
}

export function createUser({ username, password, role }) {
  if (USE_MOCKS) return mock.createUser(username, password, role)
  return request('/api/admin/users', { method: 'POST', body: JSON.stringify({ username, password, role }) })
}

export function deleteUser(username) {
  if (USE_MOCKS) return mock.deleteUser(username, session.value?.username)
  return request(`/api/admin/users/${encodeURIComponent(username)}`, { method: 'DELETE' })
}

export function getAdminStats({ from, to, binId } = {}) {
  if (USE_MOCKS) return mock.getAdminStats(from, to, binId)
  const params = new URLSearchParams()
  if (binId) params.set('bin_id', binId)
  if (from) params.set('from', from)
  if (to) params.set('to', to)
  return request(`/api/admin/stats${params.size ? `?${params}` : ''}`)
}

export function connectBin(binId, code) {
  if (USE_MOCKS) return mock.connectBin(binId, code)
  return request('/api/admin/bins/connect', { method: 'POST', body: JSON.stringify({ bin_id: binId, code }) })
}

// action is "open" or "close"; the bin receives the command within a second
export function setLid(binId, compartment, action) {
  if (USE_MOCKS) return mock.setLid(binId, compartment, action)
  return request(`/api/admin/lids/${compartment}/${action}`, { method: 'POST', body: JSON.stringify({ bin_id: binId }) })
}
