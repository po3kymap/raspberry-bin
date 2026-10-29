<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { getActivity } from '../../api.js'
import ActivityRow from '../../components/ActivityRow.vue'

const PAGE_SIZE = 20

const events = ref([])
const loading = ref(false)
const loaded = ref(false)
const hasMore = ref(true)
const error = ref('')

async function loadMore() {
  loading.value = true
  error.value = ''
  try {
    const before = events.value.at(-1)?.at
    const page = await getActivity({ limit: PAGE_SIZE, before })
    events.value.push(...page)
    hasMore.value = page.length === PAGE_SIZE
  } catch {
    error.value = 'Could not load the activity. Try again.'
  } finally {
    loading.value = false
    loaded.value = true
  }
}

onMounted(loadMore)
</script>

<template>
  <section class="space-y-4">
    <RouterLink :to="{ name: 'admin-overview' }" class="inline-flex items-center gap-1 text-sm font-medium text-brand-600">
      <svg viewBox="0 0 24 24" class="size-4" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
        <path d="M15 18l-6-6 6-6" />
      </svg>
      Overview
    </RouterLink>

    <div>
      <h1 class="text-2xl font-bold">System logs</h1>
      <p class="mt-1 text-sm text-gray-500">Logins, registrations, scans and admin actions, newest first.</p>
    </div>

    <div class="rounded-2xl bg-white px-5 shadow-sm">
      <p v-if="!loaded" class="py-5 text-sm text-gray-500">Loading…</p>
      <p v-else-if="events.length === 0 && !error" class="py-5 text-sm text-gray-500">Nothing happened yet.</p>
      <ul v-else class="divide-y divide-gray-100">
        <ActivityRow v-for="event in events" :key="event.id ?? event.at" :event="event" />
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
    <p v-else-if="loaded && events.length > 0" class="text-center text-xs text-gray-400">That's everything.</p>
  </section>
</template>
