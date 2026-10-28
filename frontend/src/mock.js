const CATEGORIES = ['GENERAL_WASTE', 'ORGANIC', 'PAPER', 'PMD']
const requests = new Map()
const scans = []

const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

function randomResult() {
  const roll = Math.random()
  if (roll < 0.15) {
    return { category: 'UNSUPPORTED', confidence: 0.84, can_sort: false, reason: 'unsupported_item' }
  }
  if (roll < 0.3) {
    return { category: 'PAPER', confidence: 0.52, can_sort: false, reason: 'low_confidence' }
  }
  const category = CATEGORIES[Math.floor(Math.random() * CATEGORIES.length)]
  return { category, confidence: 0.9 + Math.random() * 0.09, can_sort: true, reason: '' }
}

export async function startScan() {
  await wait(300)
  const requestId = crypto.randomUUID()
  requests.set(requestId, { createdAt: Date.now(), result: randomResult() })
  return { request_id: requestId }
}

export async function getScanRequest(requestId) {
  await wait(200)
  const entry = requests.get(requestId)
  if (!entry) throw new Error('Scan request not found')
  if (Date.now() - entry.createdAt < 2500) return { status: 'pending' }

  const result = { ...entry.result, scanned_at: new Date().toISOString() }
  if (!entry.saved) {
    scans.unshift(result)
    entry.saved = true
  }
  return { status: 'done', result }
}

export async function getRecentScans(limit) {
  await wait(200)
  return scans.slice(0, limit)
}

export async function getStats() {
  await wait(200)
  return {
    total: 42,
    sorted: 37,
    rejected: 5,
    by_category: { PMD: 15, ORGANIC: 9, PAPER: 7, GENERAL_WASTE: 6 },
    per_day: [
      { date: '2026-10-22', count: 4 },
      { date: '2026-10-23', count: 7 },
      { date: '2026-10-24', count: 3 },
      { date: '2026-10-25', count: 9 },
      { date: '2026-10-26', count: 6 },
      { date: '2026-10-27', count: 8 },
      { date: '2026-10-28', count: 5 },
    ],
  }
}
