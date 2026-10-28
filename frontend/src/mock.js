const CATEGORIES = ['GENERAL_WASTE', 'ORGANIC', 'PAPER', 'PMD']
const requests = new Map()
const scans = []

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const USERS_KEY = 'raspberrybin_mock_users'
const USERNAME_PATTERN = /^[a-z0-9._-]{3,32}$/
const MAX_FAILED_ATTEMPTS = 5
const LOCK_MS = 5 * 60 * 1000
const SESSION_MS = 30 * 24 * 60 * 60 * 1000

function loadUsers() {
  try {
    return JSON.parse(localStorage.getItem(USERS_KEY)) || {}
  } catch {
    return {}
  }
}

function saveUsers(users) {
  try {
    localStorage.setItem(USERS_KEY, JSON.stringify(users))
  } catch {}
}

async function hash(text) {
  const bytes = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text))
  return Array.from(new Uint8Array(bytes), (b) => b.toString(16).padStart(2, '0')).join('')
}

function checkPasswordStrength(password) {
  if (password.length < 12) throw new Error('weak password: must be at least 12 characters')
  if (password.length > 72) throw new Error('weak password: must be at most 72 bytes')
  if (!/\p{L}/u.test(password) || !/\p{N}/u.test(password)) {
    throw new Error('weak password: must contain both letters and digits')
  }
}

export async function register(username, password) {
  await sleep(300)
  const name = username.trim().toLowerCase()
  if (!USERNAME_PATTERN.test(name)) {
    throw new Error("username must be 3-32 characters: a-z, 0-9, '.', '_' or '-'")
  }
  checkPasswordStrength(password)

  const users = loadUsers()
  if (users[name]) throw new Error('username is already taken')
  users[name] = { passwordHash: await hash(password), failedAttempts: 0, lockedUntil: 0 }
  saveUsers(users)
  return { username: name }
}

export async function login(username, password) {
  await sleep(300)
  const name = username.trim().toLowerCase()
  const users = loadUsers()
  const user = users[name]
  if (!user) throw new Error('invalid username or password')
  if (Date.now() < user.lockedUntil) throw new Error('too many failed attempts, try again later')

  if (user.passwordHash !== (await hash(password))) {
    user.failedAttempts += 1
    if (user.failedAttempts >= MAX_FAILED_ATTEMPTS) {
      user.failedAttempts = 0
      user.lockedUntil = Date.now() + LOCK_MS
      saveUsers(users)
      throw new Error('too many failed attempts, try again later')
    }
    saveUsers(users)
    throw new Error('invalid username or password')
  }

  user.failedAttempts = 0
  user.lockedUntil = 0
  saveUsers(users)
  return { token: crypto.randomUUID(), expires_at: new Date(Date.now() + SESSION_MS).toISOString() }
}

export async function logout() {
  await sleep(100)
}

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

export const TEST_BIN_QR = {
  binId: 'bin-27',
  qrToken: '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
}

export async function startScan(binId, qrToken) {
  await sleep(300)
  if (binId !== TEST_BIN_QR.binId || qrToken !== TEST_BIN_QR.qrToken) {
    throw new Error('invalid qr code')
  }
  const requestId = crypto.randomUUID()
  requests.set(requestId, { createdAt: Date.now(), result: randomResult() })
  return { request_id: requestId }
}

export async function getScanRequest(requestId, wait = 25) {
  await sleep(200)
  const entry = requests.get(requestId)
  if (!entry) throw new Error('Scan request not found')

  const remaining = 2500 - (Date.now() - entry.createdAt)
  if (remaining > 0) {
    if (remaining > wait * 1000) {
      await sleep(wait * 1000)
      return { status: 'pending' }
    }
    await sleep(remaining)
  }

  const result = { ...entry.result, scanned_at: new Date().toISOString() }
  if (!entry.saved) {
    scans.unshift(result)
    entry.saved = true
  }
  return { status: 'done', result }
}

export async function getRecentScans(limit) {
  await sleep(200)
  return scans.slice(0, limit)
}

export async function getStats() {
  await sleep(200)
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
