<script setup>
import AdminDetail from '../../components/AdminDetail.vue'
import { percent } from '../../format.js'
import { useAdminStats } from '../../useAdminStats.js'

const { period, binId, summary, loading, error } = useAdminStats()
</script>

<template>
  <AdminDetail v-model:period="period" :bin="binId" title="Active users" :loaded="Boolean(summary)" :loading="loading" :error="error">
    <template v-if="summary">
      <div class="rounded-2xl bg-white p-4 shadow-sm">
        <p class="text-sm text-gray-500">Active users</p>
        <p class="mt-1 text-2xl font-bold">{{ summary.active_users }}</p>
        <p class="mt-1 text-xs text-gray-500">scanned at least one item in this period</p>
      </div>

      <div class="rounded-2xl bg-white p-5 shadow-sm">
        <p class="font-semibold">Most active</p>
        <p v-if="summary.top_users.length === 0" class="mt-3 text-sm text-gray-500">Nobody scanned an item in this period.</p>
        <ol v-else class="mt-2 divide-y divide-gray-100 text-sm">
          <li v-for="(user, i) in summary.top_users" :key="user.username" class="flex items-center gap-3 py-3">
            <span class="w-4 text-gray-400">{{ i + 1 }}</span>
            <span class="flex-1 truncate font-medium">{{ user.username }}</span>
            <span class="text-right text-gray-500">
              {{ user.sorted }} sorted · {{ user.total }} scans
              <span class="block text-xs">{{ percent(user.sorted, user.total) }}% accepted</span>
            </span>
          </li>
        </ol>
      </div>
    </template>
  </AdminDetail>
</template>
