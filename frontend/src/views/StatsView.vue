<script setup>
import { computed, onMounted, ref } from 'vue'
import { getMyStats } from '../api.js'
import { categoryInfo } from '../categories.js'

const PERIODS = [
  { id: 'today', label: 'Today' },
  { id: 'week', label: 'This week' },
  { id: 'month', label: 'This month' },
  { id: 'all', label: 'All time' },
]
const SORTED_CATEGORIES = ['PMD', 'ORGANIC', 'PAPER', 'GENERAL_WASTE']
const WEEKDAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']

const period = ref('week')
const stats = ref(null)
const loading = ref(false)
const error = ref('')
let latestRequest = 0

const dayKey = (date) => date.toLocaleDateString('sv-SE')

function startOfWeek(date) {
  const monday = new Date(date)
  monday.setDate(date.getDate() - ((date.getDay() + 6) % 7))
  return monday
}

function periodRange(id) {
  const today = new Date()
  if (id === 'today') return { from: dayKey(today), to: dayKey(today) }
  if (id === 'week') return { from: dayKey(startOfWeek(today)), to: dayKey(today) }
  if (id === 'month') return { from: dayKey(new Date(today.getFullYear(), today.getMonth(), 1)), to: dayKey(today) }
  return {}
}

async function load() {
  const requestId = ++latestRequest
  loading.value = true
  error.value = ''
  try {
    const data = await getMyStats(periodRange(period.value))
    if (requestId === latestRequest) stats.value = data
  } catch {
    if (requestId === latestRequest) error.value = 'Could not load your statistics. Try again later.'
  } finally {
    if (requestId === latestRequest) loading.value = false
  }
}

function selectPeriod(id) {
  period.value = id
  load()
}

onMounted(load)

const categories = computed(() =>
  SORTED_CATEGORIES.map((category) => {
    const count = stats.value?.by_category?.[category] ?? 0
    const share = stats.value?.sorted ? Math.round((count / stats.value.sorted) * 100) : 0
    return { category, count, share, ...categoryInfo(category) }
  }),
)

const chart = computed(() => {
  if (!stats.value || (period.value !== 'week' && period.value !== 'month')) return []

  const counts = new Map(stats.value.per_day.map((day) => [day.date, day.count]))
  const today = new Date()
  const days = []

  if (period.value === 'week') {
    const monday = startOfWeek(today)
    for (let i = 0; i < 7; i++) {
      const date = new Date(monday)
      date.setDate(monday.getDate() + i)
      days.push({ key: dayKey(date), label: WEEKDAYS[i] })
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
})
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">Statistics</h1>
      <p class="mt-1 text-sm text-gray-500">Your personal waste-sorting results.</p>
    </div>

    <div class="grid grid-cols-4 gap-1 rounded-xl bg-gray-100 p-1 text-xs font-medium">
      <button
        v-for="item in PERIODS"
        :key="item.id"
        type="button"
        class="rounded-lg py-2"
        :class="period === item.id ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500'"
        :aria-pressed="period === item.id"
        @click="selectPeriod(item.id)"
      >
        {{ item.label }}
      </button>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="!stats" class="text-sm text-gray-500">Loading…</p>

    <div v-else class="space-y-4" :class="{ 'opacity-60': loading }">
      <div class="grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-white p-4 shadow-sm">
          <p class="text-sm text-gray-500">Sorted</p>
          <p class="mt-1 text-2xl font-bold">{{ stats.sorted }}</p>
        </div>
        <div class="rounded-2xl bg-white p-4 shadow-sm">
          <p class="text-sm text-gray-500">Not accepted</p>
          <p class="mt-1 text-2xl font-bold">{{ stats.rejected }}</p>
        </div>
      </div>

      <div v-if="stats.total === 0" class="rounded-2xl bg-white p-5 text-sm text-gray-500 shadow-sm">
        No scans in this period.
      </div>

      <template v-else>
        <div class="rounded-2xl bg-white p-5 shadow-sm">
          <p class="font-semibold">By category</p>
          <ul class="mt-3 space-y-3">
            <li v-for="item in categories" :key="item.category">
              <div class="flex justify-between text-sm">
                <span>{{ item.label }}</span>
                <span class="text-gray-500">{{ item.count }} · {{ item.share }}%</span>
              </div>
              <div class="mt-1 h-2 rounded-full bg-gray-100">
                <div class="h-2 rounded-full" :class="item.bar" :style="{ width: `${item.share}%` }"></div>
              </div>
            </li>
          </ul>
        </div>

        <div v-if="chart.length" class="rounded-2xl bg-white p-5 shadow-sm">
          <p class="font-semibold">Scans per day</p>
          <div class="mt-4 flex h-32 items-end gap-1">
            <div v-for="day in chart" :key="day.key" class="flex h-full flex-1 flex-col justify-end">
              <div
                role="img"
                :aria-label="`${day.key}: ${day.count} scans`"
                class="rounded-t bg-brand-500"
                :style="{ height: `${day.height}%` }"
              ></div>
            </div>
          </div>
          <div class="mt-1 flex gap-1 text-[10px] text-gray-400">
            <span v-for="day in chart" :key="day.key" class="flex-1 text-center">{{ day.label }}</span>
          </div>
        </div>
      </template>
    </div>
  </section>
</template>
