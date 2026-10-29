<script setup>
import { computed } from 'vue'
import { categoryInfo } from '../../categories.js'
import AdminDetail from '../../components/AdminDetail.vue'
import { percent } from '../../format.js'
import { dailyChart, SORTED_CATEGORIES } from '../../periods.js'
import { useAdminStats } from '../../useAdminStats.js'

const { period, binId, summary, loading, error } = useAdminStats()

const categories = computed(() =>
  SORTED_CATEGORIES.map((category) => {
    const count = summary.value?.by_category?.[category] ?? 0
    return { category, count, share: percent(count, summary.value?.sorted), ...categoryInfo(category) }
  }),
)

const chart = computed(() => (summary.value ? dailyChart(summary.value.per_day, period.value) : []))
</script>

<template>
  <AdminDetail v-model:period="period" :bin="binId" title="Sorting" :loaded="Boolean(summary)" :loading="loading" :error="error">
    <template v-if="summary">
      <div class="grid grid-cols-2 gap-3">
        <div class="rounded-2xl bg-white p-4 shadow-sm">
          <p class="text-sm text-gray-500">Sorted</p>
          <p class="mt-1 text-2xl font-bold">{{ summary.sorted }}</p>
          <p class="mt-1 text-xs text-gray-500">of {{ summary.total }} scans</p>
        </div>
        <div class="rounded-2xl bg-white p-4 shadow-sm">
          <p class="text-sm text-gray-500">Model confidence</p>
          <p class="mt-1 text-2xl font-bold">{{ summary.sorted ? `${Math.round(summary.average_confidence * 100)}%` : '–' }}</p>
          <p class="mt-1 text-xs text-gray-500">average of sorted items</p>
        </div>
      </div>

      <div v-if="summary.total === 0" class="rounded-2xl bg-white p-5 text-sm text-gray-500 shadow-sm">
        No scans in this period.
      </div>

      <template v-else>
        <div class="rounded-2xl bg-white p-5 shadow-sm">
          <p class="font-semibold">Where the waste went</p>
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
          <div class="mt-4 flex h-36 items-end gap-1">
            <div v-for="day in chart" :key="day.key" class="flex h-full flex-1 flex-col justify-end">
              <!-- 85% leaves room for the count above the tallest bar -->
              <span v-if="period === 'week' && day.count" class="mb-1 text-center text-xs font-medium text-gray-600">{{ day.count }}</span>
              <div
                role="img"
                :aria-label="`${day.key}: ${day.count} scans`"
                class="rounded-t bg-brand-500"
                :style="{ height: `${day.height * 0.85}%` }"
              ></div>
            </div>
          </div>
          <div class="mt-1 flex gap-1 text-[10px] text-gray-400">
            <span v-for="day in chart" :key="day.key" class="flex-1 text-center">
              {{ day.date ? `${day.label} ${day.date}` : day.label }}
            </span>
          </div>
        </div>
      </template>
    </template>
  </AdminDetail>
</template>
