const PREFIX = 'RBIN1'
const BIN_ID_PATTERN = /^[A-Za-z0-9_-]{1,32}$/
const TOKEN_PATTERN = /^[0-9a-f]{64}$/

export function parseBinQr(text) {
  const parts = text.trim().split(':')
  if (parts.length !== 3 || parts[0] !== PREFIX) return null

  const [, binId, qrToken] = parts
  if (!BIN_ID_PATTERN.test(binId) || !TOKEN_PATTERN.test(qrToken)) return null
  return { binId, qrToken }
}
