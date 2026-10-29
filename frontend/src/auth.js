import { ref } from 'vue'

const STORAGE_KEY = 'raspberrybin_session'

function isValid(data) {
  return Boolean(data?.token) && new Date(data.expires_at) > new Date()
}

function load() {
  try {
    const data = JSON.parse(localStorage.getItem(STORAGE_KEY))
    return isValid(data) ? data : null
  } catch {
    return null
  }
}

export const session = ref(load())

export function saveSession(data) {
  session.value = data
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(data))
  } catch {}
}

export function clearSession() {
  session.value = null
  try {
    localStorage.removeItem(STORAGE_KEY)
  } catch {}
}

export function isLoggedIn() {
  return isValid(session.value)
}

// Only hides admin screens; the backend checks the role on every admin request
export function isAdmin() {
  return isLoggedIn() && session.value.role === 'admin'
}
