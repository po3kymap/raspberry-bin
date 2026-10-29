<script setup>
import { computed } from 'vue'
import AdminDetail from '../../components/AdminDetail.vue'
import { percent } from '../../format.js'
import { useAdminStats } from '../../useAdminStats.js'

const REASONS = {
  unsupported_item: { label: 'Item not supported', hint: 'The model saw something that belongs in none of the bins.' },
  low_confidence: { label: 'Not recognised clearly', hint: 'The model was not sure enough to open a lid.' },
  classifier_unavailable: { label: 'Classifier unavailable', hint: 'The ML service did not answer.' },
  unreadable_image: { label: 'Photo could not be read', hint: 'The camera image was broken or empty.' },
  unknown: { label: 'Other', hint: '' },
}

const { period, binId, summary, loading, error } = useAdminStats()

const reasons = computed(() =>
  Object.entries(summary.value?.by_reason ?? {})
    .sort(([, a], [, b]) => b - a)
    .map(([reason, count]) => ({
      reason,
      count,
      share: percent(count, summary.value.rejected),
      ...(REASONS[reason] ?? { label: reason, hint: '' }),
    })),
)
</script>

<template>
  <AdminDetail v-model:period="period" :bin="binId" title="Not accepted" :loaded="Boolean(summary)" :loading="loading" :error="error">
    <template v-if="summary">
      <div class="rounded-2xl bg-white p-4 shadow-sm">
        <p class="text-sm text-gray-500">Not accepted</p>
        <p class="mt-1 text-2xl font-bold">{{ summary.rejected }}</p>
        <p class="mt-1 text-xs text-gray-500">{{ percent(summary.rejected, summary.total) }}% of {{ summary.total }} scans; all lids stayed closed</p>
      </div>

      <div class="rounded-2xl bg-white p-5 shadow-sm">
        <p class="font-semibold">Reasons</p>
        <p v-if="reasons.length === 0" class="mt-3 text-sm text-gray-500">Every item was accepted.</p>
        <ul v-else class="mt-2 divide-y divide-gray-100">
          <li v-for="item in reasons" :key="item.reason" class="py-3">
            <div class="flex justify-between text-sm">
              <span class="font-medium">{{ item.label }}</span>
              <span class="text-gray-500">{{ item.count }} · {{ item.share }}%</span>
            </div>
            <p v-if="item.hint" class="mt-1 text-xs text-gray-500">{{ item.hint }}</p>
          </li>
        </ul>
      </div>
    </template>
  </AdminDetail>
</template>
