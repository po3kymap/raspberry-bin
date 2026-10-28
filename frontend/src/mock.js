const CATEGORIES = ['GENERAL_WASTE', 'ORGANIC', 'PAPER', 'PMD']
const requests = new Map()

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms))

const USERS_KEY = 'raspberrybin_mock_users'
const USERNAME_PATTERN = /^[a-z0-9._-]{3,32}$/
const MAX_FAILED_ATTEMPTS = 5
const LOCK_MS = 5 * 60 * 1000
const SESSION_MS = 8 * 60 * 60 * 1000

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
  seedHistory(name)
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
  return { token: crypto.randomUUID(), expires_at: new Date(Date.now() + SESSION_MS).toISOString(), role: 'user' }
}

export async function logout() {
  await sleep(100)
}

const HISTORY_KEY = 'raspberrybin_mock_history'

function loadHistory() {
  try {
    return JSON.parse(localStorage.getItem(HISTORY_KEY)) || {}
  } catch {
    return {}
  }
}

function saveHistory(history) {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history))
  } catch {}
}

function userScans(username) {
  return loadHistory()[username] ?? []
}

function addScan(username, scan) {
  const history = loadHistory()
  history[username] = [scan, ...(history[username] ?? [])]
  saveHistory(history)
}

const dayKey = (date) => new Date(date).toLocaleDateString('sv-SE')

function nextDay(key) {
  const date = new Date(`${key}T12:00:00`)
  date.setDate(date.getDate() + 1)
  return dayKey(date)
}

function acceptedResult() {
  const category = CATEGORIES[Math.floor(Math.random() * CATEGORIES.length)]
  return { category, confidence: 0.9 + Math.random() * 0.09, can_sort: true, reason: '' }
}

function randomResult() {
  const roll = Math.random()
  if (roll < 0.15) return { category: 'UNSUPPORTED', confidence: 0.84, can_sort: false, reason: 'unsupported_item' }
  if (roll < 0.3) return { category: 'PAPER', confidence: 0.52, can_sort: false, reason: 'low_confidence' }
  return acceptedResult()
}

function seedHistory(username) {
  const now = new Date()
  const scans = []
  for (let daysAgo = 13; daysAgo >= 0; daysAgo--) {
    if (daysAgo === 7 || daysAgo === 10) continue
    const count = 1 + Math.floor(Math.random() * 3)
    for (let i = 0; i < count; i++) {
      const at = new Date(now)
      at.setDate(at.getDate() - daysAgo)
      at.setHours(9 + i * 3, Math.floor(Math.random() * 60), 0, 0)
      if (at > now) continue
      scans.push({ ...(i === 0 ? acceptedResult() : randomResult()), scanned_at: at.toISOString() })
    }
  }
  scans.sort((a, b) => b.scanned_at.localeCompare(a.scanned_at))
  const history = loadHistory()
  history[username] = scans
  saveHistory(history)
}

export const TEST_BIN_QR = {
  binId: 'bin-27',
  code: 'test-code-for-bin-27-abcdefghijk',
}

export async function startScan(binId, code, username) {
  await sleep(300)
  if (binId !== TEST_BIN_QR.binId || code !== TEST_BIN_QR.code) {
    throw new Error('invalid bin code')
  }
  const requestId = crypto.randomUUID()
  requests.set(requestId, { createdAt: Date.now(), result: randomResult(), username })
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
    addScan(entry.username, result)
    entry.saved = true
  }
  return { status: 'done', result }
}

export async function getMyScans(username, limit, before) {
  await sleep(200)
  return userScans(username)
    .filter((scan) => !before || scan.scanned_at < before)
    .slice(0, limit)
}

export async function getMyStats(username, from, to) {
  await sleep(200)
  const today = new Date()
  const toKey = to ?? dayKey(today)
  const fromKey = from ?? (() => {
    const start = new Date(`${toKey}T12:00:00`)
    start.setDate(start.getDate() - 29)
    return dayKey(start)
  })()

  const all = userScans(username)
  const scans = all.filter((scan) => {
    const day = dayKey(scan.scanned_at)
    return day >= fromKey && day <= toKey
  })

  const byCategory = {}
  const perDay = new Map()
  for (const scan of scans) {
    if (scan.can_sort) byCategory[scan.category] = (byCategory[scan.category] ?? 0) + 1
    const day = dayKey(scan.scanned_at)
    const entry = perDay.get(day) ?? { count: 0, sorted: 0 }
    entry.count += 1
    if (scan.can_sort) entry.sorted += 1
    perDay.set(day, entry)
  }
  const sorted = scans.filter((scan) => scan.can_sort).length

  return {
    summary: {
      total: scans.length,
      sorted,
      rejected: scans.length - sorted,
      by_category: byCategory,
      per_day: [...perDay]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([date, { count, sorted }]) => ({ date, count, sorted })),
    },
    streak: streakOf(all),
  }
}

function streakOf(scans) {
  const days = [...new Set(scans.filter((scan) => scan.can_sort).map((scan) => dayKey(scan.scanned_at)))].sort()
  let longest = 0
  let run = 0
  days.forEach((day, i) => {
    run = i > 0 && nextDay(days[i - 1]) === day ? run + 1 : 1
    longest = Math.max(longest, run)
  })

  const lastSortedDate = days.at(-1) ?? ''
  const today = dayKey(new Date())
  const yesterday = new Date()
  yesterday.setDate(yesterday.getDate() - 1)
  const alive = lastSortedDate === today || lastSortedDate === dayKey(yesterday)

  return { current: alive ? run : 0, longest, last_sorted_date: lastSortedDate }
}
