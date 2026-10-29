import { ref } from 'vue'

// The bin an admin connected to by scanning its QR code. Shared by the
// overview and bins tabs; kept for the browser tab only, not across logins.
const STORAGE_KEY = 'raspberrybin_admin_bin'

function load() {
  try {
    return sessionStorage.getItem(STORAGE_KEY)
  } catch {
    return null
  }
}

export const connectedBin = ref(load())

export function setConnectedBin(binId) {
  connectedBin.value = binId
  try {
    sessionStorage.setItem(STORAGE_KEY, binId)
  } catch {}
}

export function disconnectBin() {
  connectedBin.value = null
  try {
    sessionStorage.removeItem(STORAGE_KEY)
  } catch {}
}
