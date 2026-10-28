<script setup>
import { computed } from 'vue'
import { categoryInfo } from '../categories.js'

const props = defineProps({
  scan: { type: Object, required: true },
})

const SHORT_REASONS = {
  unsupported_item: 'Item not supported',
  low_confidence: 'Not recognised clearly',
  unreadable_image: 'Photo could not be read',
  classifier_unavailable: 'Classifier unavailable',
}

const todayKey = new Date().toLocaleDateString('sv-SE')

const time = computed(() => {
  const date = new Date(props.scan.scanned_at)
  const clock = date.toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' })
  if (date.toLocaleDateString('sv-SE') === todayKey) return `Today, ${clock}`
  return `${date.toLocaleDateString('en-GB', { weekday: 'short', day: 'numeric', month: 'short' })}, ${clock}`
})

const details = computed(() => {
  if (props.scan.can_sort) return time.value
  return `${time.value} · ${SHORT_REASONS[props.scan.reason] ?? 'Not accepted'}`
})
</script>

<template>
  <li class="flex items-center gap-3 py-3">
    <div class="flex-1">
      <span
        v-if="scan.can_sort"
        class="rounded-full px-2 py-0.5 text-sm font-medium"
        :class="categoryInfo(scan.category).color"
      >
        {{ categoryInfo(scan.category).label }}
      </span>
      <span v-else class="rounded-full bg-gray-100 px-2 py-0.5 text-sm font-medium text-gray-600">Not accepted</span>
      <p class="mt-1 text-xs text-gray-500">{{ details }}</p>
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
</template>
