<script setup>
import { ref, watch } from 'vue'
import { RouterLink } from 'vue-router'
import { getAdminStats, setLid } from '../../api.js'
import { connectedBin } from '../../bin.js'
import { categoryInfo } from '../../categories.js'
import BinConnect from '../../components/BinConnect.vue'
import ConnectedBin from '../../components/ConnectedBin.vue'
import { percent } from '../../format.js'
import { periodRange, SORTED_CATEGORIES } from '../../periods.js'

// Short summary of the connected bin for this week
const summary = ref(null)
const summaryError = ref('')

watch(
  connectedBin,
  async (binId) => {
    summary.value = null
    summaryError.value = ''
    if (!binId) return
    try {
      const { summary: data } = await getAdminStats({ ...periodRange('week'), binId })
      if (connectedBin.value === binId) summary.value = data
    } catch {
      if (connectedBin.value === binId) summaryError.value = 'Could not load the statistics of this bin.'
    }
  },
  { immediate: true },
)

const busy = ref('')
const message = ref(null)

async function send(compartment, action) {
  busy.value = `${compartment}-${action}`
  message.value = null
  try {
    await setLid(connectedBin.value, compartment, action)
    message.value = { ok: true, text: `${action === 'open' ? 'Open' : 'Close'} command sent to ${categoryInfo(compartment).label}.` }
  } catch (e) {
    message.value = { ok: false, text: e.message }
  } finally {
    busy.value = ''
  }
}
</script>

<template>
  <section class="space-y-4">
    <div>
      <h1 class="text-2xl font-bold">Bins</h1>
      <p class="mt-1 text-sm text-gray-500">Connect to a bin to see its summary and open its compartments.</p>
    </div>

    <BinConnect v-if="!connectedBin" />

    <template v-else>
      <ConnectedBin />

      <p v-if="summaryError" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ summaryError }}</p>
      <div v-else class="grid grid-cols-2 gap-3">
        <RouterLink
          :to="{ name: 'admin-sorting', query: { bin: connectedBin, period: 'week' } }"
          class="rounded-2xl bg-white p-4 shadow-sm active:bg-gray-50"
        >
          <span class="flex size-10 items-center justify-center rounded-xl bg-brand-50 text-brand-600">
            <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
            </svg>
          </span>
          <p class="mt-3 text-sm text-gray-500">Sorted</p>
          <p class="text-2xl font-bold">{{ summary ? summary.sorted : '–' }}</p>
          <p class="mt-1 text-xs text-gray-500">{{ summary ? `of ${summary.total} scans this week →` : 'Loading…' }}</p>
        </RouterLink>
        <RouterLink
          :to="{ name: 'admin-rejected', query: { bin: connectedBin, period: 'week' } }"
          class="rounded-2xl bg-white p-4 shadow-sm active:bg-gray-50"
        >
          <span class="flex size-10 items-center justify-center rounded-xl bg-red-50 text-red-600">
            <svg viewBox="0 0 24 24" class="size-5" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
              <path d="M12 9v4M12 17h.01M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0z" />
            </svg>
          </span>
          <p class="mt-3 text-sm text-gray-500">Not accepted</p>
          <p class="text-2xl font-bold">{{ summary ? summary.rejected : '–' }}</p>
          <p class="mt-1 text-xs text-gray-500">{{ summary ? `${percent(summary.rejected, summary.total)}% this week →` : 'Loading…' }}</p>
        </RouterLink>
      </div>

      <p
        v-if="message"
        class="rounded-xl px-4 py-3 text-sm"
        :class="message.ok ? 'bg-brand-50 text-brand-700' : 'bg-red-50 text-red-700'"
      >
        {{ message.text }}
      </p>

      <ul class="space-y-3">
        <li v-for="compartment in SORTED_CATEGORIES" :key="compartment" class="rounded-2xl bg-white p-4 shadow-sm">
          <div class="flex items-center gap-3">
            <span class="rounded-full px-3 py-1 text-sm font-semibold" :class="categoryInfo(compartment).color">
              {{ categoryInfo(compartment).label }}
            </span>
            <span class="flex-1 truncate text-xs text-gray-500">{{ categoryInfo(compartment).description }}</span>
          </div>
          <div class="mt-3 grid grid-cols-2 gap-3">
            <button
              v-for="action in ['open', 'close']"
              :key="action"
              type="button"
              :disabled="busy !== ''"
              class="rounded-xl py-2.5 text-sm font-semibold capitalize disabled:opacity-60"
              :class="action === 'open' ? 'bg-brand-600 text-white active:bg-brand-700' : 'border border-gray-300 bg-white text-gray-700'"
              @click="send(compartment, action)"
            >
              {{ busy === `${compartment}-${action}` ? 'Sending…' : action }}
            </button>
          </div>
        </li>
      </ul>
    </template>
  </section>
</template>
