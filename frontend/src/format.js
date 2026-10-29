// "2026-10-29" -> "Thu, 29 Oct"
export function formatDay(date) {
  return new Date(`${date}T12:00:00`).toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })
}

// ISO time -> "just now", "5 min ago", "3 h ago", "2 days ago", or the date when older
export function timeAgo(iso) {
  const minutes = Math.floor((Date.now() - new Date(iso).getTime()) / 60000)
  if (minutes < 1) return 'just now'
  if (minutes < 60) return `${minutes} min ago`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} h ago`
  const days = Math.floor(hours / 24)
  if (days < 7) return `${days} ${days === 1 ? 'day' : 'days'} ago`
  return new Date(iso).toLocaleDateString('en-GB', { day: 'numeric', month: 'short', year: 'numeric' })
}

export const percent = (part, total) => (total ? Math.round((part / total) * 100) : 0)

// ISO time -> "Today, 14:32" or "Thu 29 Oct, 14:32"
export function formatTime(iso) {
  const date = new Date(iso)
  const clock = date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  if (date.toLocaleDateString('sv-SE') === new Date().toLocaleDateString('sv-SE')) return `Today, ${clock}`
  return `${date.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })}, ${clock}`
}
