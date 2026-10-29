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

// Activity log, like the "activity" collection on the server
const ACTIVITY_KEY = 'raspberrybin_mock_activity'

function loadActivity() {
  try {
    return JSON.parse(localStorage.getItem(ACTIVITY_KEY)) || []
  } catch {
    return []
  }
}

function record(event) {
  const id = crypto.randomUUID().replaceAll('-', '').slice(0, 20)
  const events = [{ can_sort: false, ...event, id, at: new Date().toISOString() }, ...loadActivity()]
  try {
    localStorage.setItem(ACTIVITY_KEY, JSON.stringify(events.slice(0, 500)))
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

async function createAccount(username, password, role) {
  const name = username.trim().toLowerCase()
  if (!USERNAME_PATTERN.test(name)) {
    throw new Error("username must be 3-32 characters: a-z, 0-9, '.', '_' or '-'")
  }
  checkPasswordStrength(password)

  const users = loadUsers()
  if (users[name]) throw new Error('username is already taken')
  users[name] = {
    passwordHash: await hash(password),
    role,
    createdAt: new Date().toISOString(),
    failedAttempts: 0,
    lockedUntil: 0,
  }
  saveUsers(users)
  seedHistory(name)
  return name
}

// In mock mode an account registered as "admin" is an admin, so the admin
// screens can be tried without the backend
const roleOf = (name, user) => user.role ?? (name === 'admin' ? 'admin' : 'user')

export async function register(username, password) {
  await sleep(300)
  const role = username.trim().toLowerCase() === 'admin' ? 'admin' : 'user'
  const name = await createAccount(username, password, role)
  record({ type: 'register', username: name })
  return { username: name }
}

export async function login(username, password) {
  await sleep(300)
  const name = username.trim().toLowerCase()
  const users = loadUsers()
  const user = users[name]
  const fail = (reason, message) => {
    record({ type: 'login_failed', username: name, reason })
    throw new Error(message)
  }
  if (!user) fail('wrong_password', 'invalid username or password')
  if (Date.now() < user.lockedUntil) fail('locked', 'too many failed attempts, try again later')

  if (user.passwordHash !== (await hash(password))) {
    user.failedAttempts += 1
    if (user.failedAttempts >= MAX_FAILED_ATTEMPTS) {
      user.failedAttempts = 0
      user.lockedUntil = Date.now() + LOCK_MS
      saveUsers(users)
      fail('locked', 'too many failed attempts, try again later')
    }
    saveUsers(users)
    fail('wrong_password', 'invalid username or password')
  }

  user.failedAttempts = 0
  user.lockedUntil = 0
  user.lastLoginAt = new Date().toISOString()
  saveUsers(users)
  record({ type: 'login', username: name, role: roleOf(name, user) })
  return { token: crypto.randomUUID(), expires_at: new Date(Date.now() + SESSION_MS).toISOString(), role: roleOf(name, user) }
}

export async function logout(username, role) {
  await sleep(100)
  record({ type: 'logout', username, role })
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
      scans.push({ ...(i === 0 ? acceptedResult() : randomResult()), bin_id: TEST_BIN_QR.binId, scanned_at: at.toISOString() })
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
  requests.set(requestId, { createdAt: Date.now(), result: { ...randomResult(), bin_id: binId }, username })
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
    record({
      type: 'scan',
      username: entry.username,
      bin_id: result.bin_id,
      scan_id: crypto.randomUUID().replaceAll('-', '').slice(0, 20),
      image_id: `scans/${crypto.randomUUID()}.jpg`,
      category: result.category,
      confidence: result.confidence,
      can_sort: result.can_sort,
      reason: result.reason,
    })
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

function defaultRange(from, to) {
  const toKey = to ?? dayKey(new Date())
  const fromKey = from ?? (() => {
    const start = new Date(`${toKey}T12:00:00`)
    start.setDate(start.getDate() - 29)
    return dayKey(start)
  })()
  return { fromKey, toKey }
}

function inRange(scans, fromKey, toKey) {
  return scans.filter((scan) => {
    const day = dayKey(scan.scanned_at)
    return day >= fromKey && day <= toKey
  })
}

function summarize(scans) {
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
    total: scans.length,
    sorted,
    rejected: scans.length - sorted,
    by_category: byCategory,
    per_day: [...perDay]
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([date, { count, sorted }]) => ({ date, count, sorted })),
  }
}

export async function getMyStats(username, from, to) {
  await sleep(200)
  const { fromKey, toKey } = defaultRange(from, to)
  const all = userScans(username)
  return { summary: summarize(inRange(all, fromKey, toKey)), streak: streakOf(all) }
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

// Admin. Mirrors /api/admin/users and /api/admin/stats; the current admin is
// passed in because the mock has no server-side session.

export async function listUsers() {
  await sleep(200)
  const users = loadUsers()
  return Object.keys(users)
    .sort()
    .map((name) => {
      const scans = userScans(name)
      const streak = streakOf(scans)
      return {
        username: name,
        role: roleOf(name, users[name]),
        created_at: users[name].createdAt ?? null,
        last_login_at: users[name].lastLoginAt ?? null,
        locked: Date.now() < users[name].lockedUntil,
        current_streak: streak.current,
        longest_streak: streak.longest,
        last_sorted_date: streak.last_sorted_date,
        total_scans: scans.length,
        sorted_scans: scans.filter((scan) => scan.can_sort).length,
      }
    })
}

export async function createUser(username, password, role, admin) {
  await sleep(300)
  if (role !== 'user' && role !== 'admin') throw new Error('role must be user or admin')
  const name = await createAccount(username, password, role)
  record({ type: 'user_created', username: admin, target: name, role })
  return { username: name, role }
}

export async function deleteUser(username, currentUsername) {
  await sleep(300)
  const users = loadUsers()
  if (username === currentUsername) throw new Error('you cannot delete your own account')
  if (!users[username]) throw new Error('user not found')
  const admins = Object.keys(users).filter((name) => roleOf(name, users[name]) === 'admin')
  if (roleOf(username, users[username]) === 'admin' && admins.length < 2) {
    throw new Error('the last admin cannot be deleted')
  }

  delete users[username]
  saveUsers(users)
  const history = loadHistory()
  delete history[username]
  saveHistory(history)
  record({ type: 'user_deleted', username: currentUsername, target: username })
}

export async function connectBin(binId, code, admin) {
  await sleep(300)
  if (binId !== TEST_BIN_QR.binId || code !== TEST_BIN_QR.code) throw new Error('invalid bin code')
  record({ type: 'bin_connect', username: admin, bin_id: binId })
  return { bin_id: binId }
}

export async function setLid(binId, compartment, action, admin) {
  await sleep(300)
  if (binId !== TEST_BIN_QR.binId) throw new Error('bin not found')
  record({ type: 'lid', username: admin, bin_id: binId, compartment, action })
  return { status: 'queued' }
}

export async function getActivity(limit, before) {
  await sleep(200)
  return loadActivity()
    .filter((event) => !before || event.at < before)
    .slice(0, limit)
}

// Scans saved before bin_id was added to the mock belong to the test bin
const binOf = (scan) => scan.bin_id ?? TEST_BIN_QR.binId

export async function getAdminStats(from, to, binId) {
  await sleep(200)
  const { fromKey, toKey } = defaultRange(from, to)
  const users = loadUsers()
  const names = Object.keys(users)

  const scans = []
  const perUser = []
  for (const name of names) {
    const own = inRange(userScans(name), fromKey, toKey).filter((scan) => !binId || binOf(scan) === binId)
    scans.push(...own)
    if (own.length) {
      perUser.push({ username: name, total: own.length, sorted: own.filter((scan) => scan.can_sort).length })
    }
  }

  const byReason = {}
  const byBin = {}
  let confidenceSum = 0
  for (const scan of scans) {
    byBin[binOf(scan)] = (byBin[binOf(scan)] ?? 0) + 1
    if (scan.can_sort) {
      confidenceSum += scan.confidence
    } else {
      const reason = scan.reason || 'unknown'
      byReason[reason] = (byReason[reason] ?? 0) + 1
    }
  }

  const summary = summarize(scans)
  perUser.sort((a, b) => b.sorted - a.sorted || b.total - a.total || a.username.localeCompare(b.username))
  return {
    summary: {
      ...summary,
      by_reason: byReason,
      by_bin: byBin,
      average_confidence: summary.sorted ? confidenceSum / summary.sorted : 0,
      active_users: perUser.length,
      top_users: perUser.slice(0, 5),
    },
    users: {
      total: names.length,
      admins: names.filter((name) => roleOf(name, users[name]) === 'admin').length,
      new: names.filter((name) => {
        const created = users[name].createdAt
        return created && dayKey(created) >= fromKey && dayKey(created) <= toKey
      }).length,
    },
  }
}

export async function checkItem() {
  await sleep(1200)
  return randomResult()
}
