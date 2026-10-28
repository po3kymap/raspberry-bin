import * as mock from './mock.js'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'
const BIN_ID = 'bin-27'

async function request(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: { 'Content-Type': 'application/json', ...options.headers },
  })
  const data = response.status === 204 ? null : await response.json().catch(() => null)
  if (!response.ok) {
    throw new Error(data?.error || `Request failed (${response.status})`)
  }
  return data
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
