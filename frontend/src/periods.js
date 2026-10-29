// Shared by the user statistics and the admin overview.

export const PERIODS = [
  { id: 'today', label: 'Today' },
  { id: 'week', label: 'This week' },
  { id: 'month', label: 'This month' },
  { id: 'year', label: 'This year' },
]

export const SORTED_CATEGORIES = ['PMD', 'ORGANIC', 'PAPER', 'GENERAL_WASTE']

const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const dayKey = (date) => date.toLocaleDateString('sv-SE')

function startOfWeek(date) {
  const monday = new Date(date)
  monday.setDate(date.getDate() - ((date.getDay() + 6) % 7))
  return monday
}

export function periodRange(id) {
  const today = new Date()
  if (id === 'today') return { from: dayKey(today), to: dayKey(today) }
  if (id === 'week') return { from: dayKey(startOfWeek(today)), to: dayKey(today) }
  if (id === 'month') return { from: dayKey(new Date(today.getFullYear(), today.getMonth(), 1)), to: dayKey(today) }
  return { from: dayKey(new Date(today.getFullYear(), 0, 1)), to: dayKey(today) }
}

// Bars for the "scans per day" chart; only the week and month periods have one.
export function dailyChart(perDay, period) {
  if (period !== 'week' && period !== 'month') return []

  const counts = new Map(perDay.map((day) => [day.date, day.count]))
  const today = new Date()
  const days = []

  if (period === 'week') {
    const monday = startOfWeek(today)
    for (let i = 0; i < 7; i++) {
      const date = new Date(monday)
      date.setDate(monday.getDate() + i)
      days.push({ key: dayKey(date), label: WEEKDAYS[i], date: String(date.getDate()) })
    }
  } else {
    const lastDay = new Date(today.getFullYear(), today.getMonth() + 1, 0).getDate()
    for (let day = 1; day <= lastDay; day++) {
      const date = new Date(today.getFullYear(), today.getMonth(), day)
      days.push({ key: dayKey(date), label: day === 1 || day % 5 === 0 ? String(day) : '' })
    }
  }

  const max = Math.max(1, ...days.map((day) => counts.get(day.key) ?? 0))
  return days.map((day) => {
    const count = counts.get(day.key) ?? 0
    return { ...day, count, height: count ? Math.max(6, Math.round((count / max) * 100)) : 0 }
  })
}
