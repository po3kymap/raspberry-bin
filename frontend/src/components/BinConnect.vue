<script setup>
import { ref } from 'vue'
import { connectBin } from '../api.js'
import { setConnectedBin } from '../bin.js'
import { TEST_BIN_QR } from '../mock.js'
import QrScanner from './QrScanner.vue'

const USE_MOCKS = import.meta.env.VITE_USE_MOCKS === 'true'

const scanning = ref(false)
const connecting = ref(false)
const error = ref('')

async function connect({ binId, code }) {
  scanning.value = false
  connecting.value = true
  error.value = ''
  try {
    await connectBin(binId, code)
    setConnectedBin(binId)
  } catch (e) {
    error.value = e.message === 'invalid bin code' ? 'This QR code does not belong to a bin.' : e.message
  } finally {
    connecting.value = false
  }
}
</script>

<template>
  <div class="space-y-4">
    <template v-if="scanning">
      <QrScanner @detected="connect" @cancel="scanning = false" />
      <button
        v-if="USE_MOCKS"
        type="button"
        class="w-full rounded-xl border border-dashed border-gray-400 py-3 text-sm text-gray-600"
        @click="connect(TEST_BIN_QR)"
      >
        Use test QR (mock mode)
      </button>
    </template>

    <div v-else class="rounded-2xl bg-white p-6 text-center shadow-sm">
      <div class="mx-auto flex size-12 items-center justify-center rounded-xl bg-brand-50 text-brand-600">
        <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2M7 12h10" />
        </svg>
      </div>
      <p class="mt-4 font-medium">Connect to a bin</p>
      <p class="mt-1 text-sm text-gray-500">Scan the QR code on the bin to see its summary and open its compartments.</p>

      <p v-if="error" class="mt-4 rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>

      <button
        type="button"
        :disabled="connecting"
        class="mt-6 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white active:bg-brand-700 disabled:opacity-60"
        @click="scanning = true"
      >
        {{ connecting ? 'Connecting…' : 'Scan QR code' }}
      </button>
    </div>
  </div>
</template>
