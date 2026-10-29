<script setup>
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'
import { getActivity } from '../api.js'
import ActivityRow from './ActivityRow.vue'

const events = ref([])
const loaded = ref(false)
const error = ref('')

onMounted(async () => {
  try {
    events.value = await getActivity({ limit: 5 })
  } catch {
    error.value = 'Could not load the activity.'
  } finally {
    loaded.value = true
  }
})
</script>

<template>
  <div class="rounded-2xl bg-white p-5 shadow-sm">
    <div class="flex items-center justify-between">
      <div>
        <p class="font-semibold">System logs</p>
        <p class="text-xs text-gray-500">Latest entries: logins, scans and admin actions</p>
      </div>
      <RouterLink :to="{ name: 'admin-activity' }" class="text-sm font-medium text-brand-600">View all →</RouterLink>
    </div>

    <p v-if="!loaded" class="mt-3 text-sm text-gray-500">Loading…</p>
    <p v-else-if="error" class="mt-3 text-sm text-red-700">{{ error }}</p>
    <p v-else-if="events.length === 0" class="mt-3 text-sm text-gray-500">Nothing happened yet.</p>
    <ul v-else class="mt-1 divide-y divide-gray-100">
      <ActivityRow v-for="event in events" :key="event.id ?? event.at" :event="event" />
    </ul>
  </div>
</template>
