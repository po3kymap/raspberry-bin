<script setup>
import { RouterLink } from 'vue-router'
import PeriodTabs from './PeriodTabs.vue'

// Layout of the overview detail pages: back link, title and period. With a bin
// the page shows that bin only and leads back to the Bins tab.
defineProps({
  title: { type: String, required: true },
  bin: { type: String, default: null },
  error: { type: String, default: '' },
  loaded: { type: Boolean, default: false },
  loading: { type: Boolean, default: false },
})
const period = defineModel('period', { type: String, required: true })
</script>

<template>
  <section class="space-y-4">
    <RouterLink
      :to="bin ? { name: 'admin-bins' } : { name: 'admin-overview', query: { period } }"
      class="inline-flex items-center gap-1 text-sm font-medium text-brand-600"
    >
      <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M15 18l-6-6 6-6" />
      </svg>
      {{ bin ? 'Bins' : 'Overview' }}
    </RouterLink>

    <div>
      <h1 class="text-2xl font-bold">{{ title }}</h1>
      <p class="mt-1 text-sm text-gray-500">{{ bin ?? 'All bins' }}</p>
    </div>

    <PeriodTabs v-model="period" />
    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="!loaded" class="text-sm text-gray-500">Loading…</p>
    <div v-else class="space-y-4" :class="{ 'opacity-60': loading }">
      <slot />
    </div>
  </section>
</template>
