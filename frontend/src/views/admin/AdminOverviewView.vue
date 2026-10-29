<script setup>
import { computed } from 'vue'
import { RouterLink } from 'vue-router'
import PeriodTabs from '../../components/PeriodTabs.vue'
import { percent } from '../../format.js'
import { useAdminStats } from '../../useAdminStats.js'

const { period, data, summary, loading, error } = useAdminStats()

const tiles = computed(() => {
  const s = summary.value
  const users = data.value.users
  const query = { period: period.value }
  return [
    {
      label: 'Total users',
      value: users.total,
      note: `+${users.new} new`,
      to: { name: 'admin-users' },
      tint: 'bg-blue-50 text-blue-600',
      icon: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75',
    },
    {
      label: 'Total sorted',
      value: s.sorted,
      note: `of ${s.total} scans`,
      to: { name: 'admin-sorting', query },
      tint: 'bg-brand-50 text-brand-600',
      icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2',
    },
    {
      label: 'Not accepted',
      value: s.rejected,
      note: `${percent(s.rejected, s.total)}% of scans`,
      to: { name: 'admin-rejected', query },
      tint: 'bg-red-50 text-red-600',
      icon: 'M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z',
    },
    {
      label: 'Active users',
      value: s.active_users,
      note: 'scanned an item',
      to: { name: 'admin-active', query },
      tint: 'bg-amber-50 text-amber-600',
      icon: 'M22 12h-4l-3 9L9 3l-3 9H2',
    },
  ]
})
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">Overview</h1>
      <p class="mt-1 text-sm text-gray-500">Sorting results of all bins.</p>
    </div>

    <PeriodTabs v-model="period" />

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="!data" class="text-sm text-gray-500">Loading…</p>

    <div v-else class="grid grid-cols-2 gap-3" :class="{ 'opacity-60': loading }">
      <RouterLink
        v-for="tile in tiles"
        :key="tile.label"
        :to="tile.to"
        class="rounded-2xl bg-white p-4 shadow-sm active:bg-gray-50"
      >
        <span class="flex size-10 items-center justify-center rounded-xl" :class="tile.tint">
          <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
            <path :d="tile.icon" />
          </svg>
        </span>
        <p class="mt-3 text-sm text-gray-500">{{ tile.label }}</p>
        <p class="text-2xl font-bold">{{ tile.value }}</p>
        <p class="mt-1 text-xs text-gray-500">{{ tile.note }} →</p>
      </RouterLink>
    </div>
  </section>
</template>
