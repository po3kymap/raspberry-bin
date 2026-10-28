const PREFIX = 'rbin'
const BIN_ID_PATTERN = /^[A-Za-z0-9_-]{1,32}$/
const CODE_PATTERN = /^[A-Za-z0-9_-]{32}$/

export function parseBinQr(text) {
  const parts = text.trim().split(':')
  if (parts.length !== 3 || parts[0] !== PREFIX) return null

  const [, binId, code] = parts
  if (!BIN_ID_PATTERN.test(binId) || !CODE_PATTERN.test(code)) return null
  return { binId, code }
}
