<script setup>
import QrScanner from 'qr-scanner'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { parseBinQr } from '../qr.js'

const emit = defineEmits(['detected', 'cancel'])

const video = ref(null)
const error = ref('')
const hint = ref('')
let scanner = null

function cameraErrorMessage(e) {
  if (!window.isSecureContext) return 'The camera only works over HTTPS.'
  if (e?.name === 'NotAllowedError') return 'Camera access was denied. Allow it in your browser settings and try again.'
  if (e?.name === 'NotFoundError' || String(e).includes('Camera not found')) return 'No camera was found on this device.'
  return 'Could not start the camera.'
}

onMounted(async () => {
  scanner = new QrScanner(
    video.value,
    (result) => {
      const qr = parseBinQr(result.data)
      if (!qr) {
        hint.value = 'This is not a RaspberryBin QR code.'
        return
      }
      scanner.stop()
      emit('detected', qr)
    },
    { preferredCamera: 'environment', highlightScanRegion: true, maxScansPerSecond: 5 },
  )

  try {
    await scanner.start()
  } catch (e) {
    error.value = cameraErrorMessage(e)
  }
})

onBeforeUnmount(() => {
  scanner?.destroy()
})
</script>

<template>
  <div class="space-y-4">
    <p class="text-center text-gray-600">Point the camera at the QR code on the bin.</p>

    <div class="relative aspect-square overflow-hidden rounded-2xl bg-gray-900">
      <video ref="video" class="size-full object-cover" muted playsinline></video>
      <p v-if="error" class="absolute inset-0 flex items-center justify-center p-6 text-center text-white">
        {{ error }}
      </p>
    </div>

    <p v-if="hint && !error" class="rounded-xl bg-amber-50 px-4 py-3 text-sm text-amber-800">{{ hint }}</p>

    <button
      type="button"
      class="w-full rounded-xl border border-gray-300 bg-white py-3 font-semibold text-gray-700"
      @click="emit('cancel')"
    >
      Cancel
    </button>
  </div>
</template>
