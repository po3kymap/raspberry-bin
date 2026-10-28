<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { getMyScans } from '../api.js'
import ScanRow from '../components/ScanRow.vue'

const PAGE_SIZE = 20

const scans = ref([])
const loading = ref(false)
const loaded = ref(false)
const hasMore = ref(true)
const error = ref('')

async function loadMore() {
  loading.value = true
  error.value = ''
  try {
    const before = scans.value.at(-1)?.scanned_at
    const page = await getMyScans({ limit: PAGE_SIZE, before })
    scans.value.push(...page)
    hasMore.value = page.length === PAGE_SIZE
  } catch {
    error.value = 'Could not load your scans. Try again.'
  } finally {
    loading.value = false
    loaded.value = true
  }
}

onMounted(loadMore)
</script>

<template>
  <section class="space-y-4">
    <RouterLink to="/" class="inline-flex items-center gap-1 text-sm font-medium text-brand-600">
      <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M15 18l-6-6 6-6" />
      </svg>
      Back
    </RouterLink>

    <div>
      <h1 class="text-2xl font-bold">History</h1>
      <p class="mt-1 text-sm text-gray-500">Every item you scanned, newest first.</p>
    </div>

    <div class="rounded-2xl bg-white px-5 shadow-sm">
      <p v-if="!loaded" class="py-5 text-sm text-gray-500">Loading…</p>
      <p v-else-if="scans.length === 0 && !error" class="py-5 text-sm text-gray-500">No scans yet.</p>
      <ul v-else class="divide-y divide-gray-100">
        <ScanRow v-for="(scan, i) in scans" :key="`${scan.scanned_at}-${i}`" :scan="scan" />
      </ul>
    </div>

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>

    <button
      v-if="loaded && hasMore"
      type="button"
      :disabled="loading"
      class="w-full rounded-xl border border-gray-300 bg-white py-3 font-semibold text-gray-700 disabled:opacity-60"
      @click="loadMore"
    >
      {{ loading ? 'Loading…' : 'Load more' }}
    </button>
    <p v-else-if="loaded && scans.length > 0" class="text-center text-xs text-gray-400">That's everything.</p>
  </section>
</template>
