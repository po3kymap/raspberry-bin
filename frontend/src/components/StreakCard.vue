<script setup>
import { computed } from 'vue'

const props = defineProps({
  streak: { type: Object, required: true },
})

const DAY_LABELS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun']
const todayIndex = (new Date().getDay() + 6) % 7
const todayKey = new Date().toLocaleDateString('sv-SE')

const daysLabel = computed(() => `${props.streak.current} ${props.streak.current === 1 ? 'day' : 'days'}`)

const subtitle = computed(() => {
  const { current, best, last_day: lastDay } = props.streak
  if (current === 0) return 'Sort an item today to start a streak.'
  if (lastDay !== todayKey) return 'Sort an item today to keep it going.'
  if (current >= best) return 'Personal best!'
  return `Best: ${best} days`
})
</script>

<template>
  <div class="rounded-2xl bg-white p-5 shadow-sm">
    <p class="font-semibold">Your streak</p>

    <div class="mt-3 flex items-center gap-3">
      <div class="flex size-12 items-center justify-center rounded-xl bg-orange-100 text-2xl" aria-hidden="true">🔥</div>
      <div>
        <p class="text-xl font-bold">{{ daysLabel }}</p>
        <p class="text-sm text-gray-500">{{ subtitle }}</p>
      </div>
    </div>

    <ul class="mt-4 flex justify-between">
      <li v-for="(label, i) in DAY_LABELS" :key="label" class="flex flex-col items-center gap-1">
        <span
          role="img"
          :aria-label="`${label}: ${streak.week[i] ? 'sorted' : 'no sorting'}`"
          class="flex size-8 items-center justify-center rounded-full"
          :class="streak.week[i] ? 'bg-brand-600 text-white' : i > todayIndex ? 'border border-dashed border-gray-300' : 'bg-gray-100'"
        >
          <svg v-if="streak.week[i]" viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
            <path d="M5 12l5 5L19 7" />
          </svg>
        </span>
        <span class="text-xs" :class="i === todayIndex ? 'font-semibold text-gray-900' : 'text-gray-500'">{{ label }}</span>
      </li>
    </ul>
  </div>
</template>
