<script setup>
import { RouterLink } from 'vue-router'
import ScanRow from './ScanRow.vue'

defineProps({
  scans: { type: Array, required: true },
})
</script>

<template>
  <div class="rounded-2xl bg-white p-5 shadow-sm">
    <div class="flex items-center justify-between">
      <p class="font-semibold">Recent activity</p>
      <RouterLink v-if="scans.length > 0" to="/history" class="text-sm font-medium text-brand-600">View all →</RouterLink>
    </div>

    <p v-if="scans.length === 0" class="mt-3 text-sm text-gray-500">No scans yet. Your sorted items will show up here.</p>

    <ul v-else class="mt-1 divide-y divide-gray-100">
      <ScanRow v-for="(scan, i) in scans" :key="`${scan.scanned_at}-${i}`" :scan="scan" />
    </ul>
  </div>
</template>
