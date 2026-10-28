<script setup>
import { onMounted, ref } from 'vue'
import { getMyScans, getMyStreak, startScan, waitForScanResult } from '../api.js'
import { categoryInfo } from '../categories.js'
import QrScanner from '../components/QrScanner.vue'
import RecentScans from '../components/RecentScans.vue'
import StreakCard from '../components/StreakCard.vue'
import { TEST_BIN_QR } from '../mock.js'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

const REASONS = {
  unsupported_item: 'This bin cannot sort this item.',
  low_confidence: 'The item could not be recognised clearly. Try placing it differently.',
  unreadable_image: 'The photo could not be processed.',
  classifier_unavailable: 'The classifier is unavailable right now.',
  timeout: 'The bin did not respond in time.',
}

const state = ref('idle')
const result = ref(null)
const error = ref('')

const streak = ref(null)
const recentScans = ref([])
const dashboardLoaded = ref(false)
const dashboardError = ref('')

async function loadDashboard() {
  try {
    const [streakData, scans] = await Promise.all([getMyStreak(), getMyScans({ limit: 5 })])
    streak.value = streakData
    recentScans.value = scans
    dashboardError.value = ''
  } catch {
    dashboardError.value = 'Could not load your activity. Try again later.'
  } finally {
    dashboardLoaded.value = true
  }
}

onMounted(loadDashboard)

async function runScan({ binId, qrToken }) {
  state.value = 'scanning'
  error.value = ''
  try {
    const { request_id } = await startScan(binId, qrToken)
    const response = await waitForScanResult(request_id)
    if (response.status === 'done') {
      result.value = response.result
      state.value = 'result'
    } else {
      error.value = REASONS[response.reason] ?? 'The scan failed. Please try again.'
      state.value = 'error'
    }
  } catch (e) {
    error.value = e.message === 'invalid qr code' ? 'This QR code does not belong to a bin.' : e.message
    state.value = 'error'
  }
  loadDashboard()
}

function reset() {
  state.value = 'idle'
  result.value = null
  error.value = ''
}
</script>

<template>
  <section class="space-y-4">
    <h1 class="text-2xl font-bold">Scan</h1>

    <div v-if="state === 'idle'" class="rounded-2xl bg-white p-6 text-center shadow-sm">
      <p class="font-medium">Ready to scan</p>
      <p class="mt-1 text-sm text-gray-500">Place one item in the scanning area, then scan the QR code on the bin.</p>
      <button
        type="button"
        class="mt-6 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white active:bg-brand-700"
        @click="state = 'qr'"
      >
        Start Scan
      </button>
    </div>

    <template v-else-if="state === 'qr'">
      <QrScanner @detected="runScan" @cancel="reset" />
      <button
        v-if="USE_MOCKS"
        type="button"
        class="w-full rounded-xl border border-dashed border-gray-400 py-3 text-sm text-gray-600"
        @click="runScan(TEST_BIN_QR)"
      >
        Use test QR (mock mode)
      </button>
    </template>

    <div v-else-if="state === 'scanning'" class="rounded-2xl bg-white p-6 text-center shadow-sm">
      <div class="mx-auto size-10 animate-spin rounded-full border-4 border-brand-100 border-t-brand-600"></div>
      <p class="mt-4 font-medium">Scanning…</p>
      <p class="mt-1 text-sm text-gray-500">The bin is taking a photo of your item.</p>
    </div>

    <div v-else-if="state === 'result' && result.can_sort" class="rounded-2xl bg-white p-6 text-center shadow-sm">
      <p class="text-sm text-gray-500">Put it in</p>
      <p class="mt-2 inline-block rounded-full px-4 py-1 text-lg font-semibold" :class="categoryInfo(result.category).color">
        {{ categoryInfo(result.category).label }}
      </p>
      <p class="mt-2 text-sm text-gray-500">{{ categoryInfo(result.category).description }}</p>
      <p class="mt-4 text-xs text-gray-400">Confidence {{ Math.round(result.confidence * 100) }}%</p>
      <button type="button" class="mt-6 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white" @click="reset">
        Scan another item
      </button>
    </div>

    <div v-else class="rounded-2xl bg-white p-6 text-center shadow-sm">
      <p class="font-semibold text-red-700">Item not accepted</p>
      <p class="mt-2 text-sm text-gray-600">{{ state === 'error' ? error : REASONS[result.reason] ?? 'Please try again.' }}</p>
      <p class="mt-1 text-sm text-gray-500">The lids stay closed. Take the item back and scan again.</p>
      <button type="button" class="mt-6 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white" @click="reset">
        Try again
      </button>
    </div>

    <template v-if="state !== 'qr' && dashboardLoaded">
      <p v-if="dashboardError" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ dashboardError }}</p>
      <template v-else>
        <StreakCard :streak="streak" />
        <RecentScans :scans="recentScans" />
      </template>
    </template>
  </section>
</template>
