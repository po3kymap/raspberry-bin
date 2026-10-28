<script setup>
import { categoryInfo } from '../categories.js'

defineProps({
  scans: { type: Array, required: true },
})

const todayKey = new Date().toLocaleDateString('sv-SE')

function formatTime(iso) {
  const date = new Date(iso)
  const time = date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  if (date.toLocaleDateString('sv-SE') === todayKey) return `Today, ${time}`
  return `${date.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })}, ${time}`
}
</script>

<template>
  <div class="rounded-2xl bg-white p-5 shadow-sm">
    <p class="font-semibold">Recent activity</p>

    <p v-if="scans.length === 0" class="mt-3 text-sm text-gray-500">No scans yet. Your sorted items will show up here.</p>

    <ul v-else class="mt-1 divide-y divide-gray-100">
      <li v-for="(scan, i) in scans" :key="`${scan.scanned_at}-${i}`" class="flex items-center gap-3 py-3">
        <div class="flex-1">
          <span
            v-if="scan.can_sort"
            class="rounded-full px-2 py-0.5 text-sm font-medium"
            :class="categoryInfo(scan.category).color"
          >
            {{ categoryInfo(scan.category).label }}
          </span>
          <span v-else class="rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">Not accepted</span>
          <p class="mt-1 text-xs text-gray-500">{{ formatTime(scan.scanned_at) }}</p>
        </div>

        <span
          role="img"
          :aria-label="scan.can_sort ? 'Accepted' : 'Not accepted'"
          class="flex size-7 items-center justify-center rounded-full"
          :class="scan.can_sort ? 'bg-brand-100 text-brand-700' : 'bg-red-50 text-red-600'"
        >
          <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
            <path :d="scan.can_sort ? 'M5 12l5 5L19 7' : 'M6 6l12 12M18 6L6 18'" />
          </svg>
        </span>
      </li>
    </ul>
  </div>
</template>
