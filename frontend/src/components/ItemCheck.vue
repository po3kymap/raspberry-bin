<script setup>
import { onBeforeUnmount, ref } from 'vue'
import { checkItem } from '../api.js'
import { categoryInfo } from '../categories.js'
import { prepareImage } from '../image.js'

const REASONS = {
  low_confidence: 'Not sure what this is. Try one item on a plain background in good light.',
  classifier_unavailable: 'The checker is unavailable right now. Try again later.',
  unreadable_image: 'The photo could not be read. Try again.',
}

const input = ref(null)
const state = ref('idle')
const photo = ref(null)
const preview = ref(null)
const result = ref(null)
const error = ref('')

function takePhoto() {
  input.value.value = ''
  input.value.click()
}

async function onPhoto(event) {
  const file = event.target.files?.[0]
  if (!file) return
  error.value = ''
  try {
    photo.value = await prepareImage(file)
    setPreview(URL.createObjectURL(photo.value))
    state.value = 'preview'
  } catch {
    error.value = 'Could not open this photo. Try taking it again.'
    state.value = 'error'
  }
}

async function check() {
  state.value = 'checking'
  try {
    result.value = await checkItem(photo.value)
    state.value = 'result'
  } catch (e) {
    error.value = e.message
    state.value = 'error'
  }
}

function setPreview(url) {
  if (preview.value) URL.revokeObjectURL(preview.value)
  preview.value = url
}

function reset() {
  setPreview(null)
  photo.value = null
  result.value = null
  error.value = ''
  state.value = 'idle'
}

onBeforeUnmount(() => setPreview(null))
</script>

<template>
  <div class="rounded-2xl bg-white p-6 shadow-sm">
    <input ref="input" type="file" accept="image/*" capture="environment" class="hidden" @change="onPhoto" />

    <div v-if="state === 'idle'" class="text-center">
      <p class="font-medium">Not sure where it goes?</p>
      <p class="mt-1 text-sm text-gray-500">Take a photo of an item, for example at home, to see which bin it belongs in.</p>
      <button
        type="button"
        class="mt-6 w-full rounded-xl border border-brand-600 py-3 font-semibold text-brand-700 active:bg-brand-50"
        @click="takePhoto"
      >
        Check an item
      </button>
      <p class="mt-3 text-xs text-gray-400">Photos are saved to help improve the model.</p>
    </div>

    <template v-else>
      <img v-if="preview" :src="preview" alt="Your photo" class="mx-auto max-h-64 rounded-xl object-contain" />

      <div v-if="state === 'preview'" class="mt-4 grid grid-cols-2 gap-3">
        <button type="button" class="rounded-xl border border-gray-300 bg-white py-3 font-semibold text-gray-700" @click="takePhoto">
          Retake
        </button>
        <button type="button" class="rounded-xl bg-brand-600 py-3 font-semibold text-white active:bg-brand-700" @click="check">
          Check
        </button>
      </div>

      <div v-else-if="state === 'checking'" class="mt-4 text-center">
        <div class="mx-auto size-8 animate-spin rounded-full border-4 border-brand-100 border-t-brand-600"></div>
        <p class="mt-3 text-sm text-gray-500">Checking…</p>
      </div>

      <div v-else-if="state === 'result'" class="mt-4 text-center">
        <template v-if="result.can_sort">
          <p class="text-sm text-gray-500">This goes in</p>
          <p class="mt-2 inline-block rounded-full px-4 py-1 text-lg font-semibold" :class="categoryInfo(result.category).color">
            {{ categoryInfo(result.category).label }}
          </p>
          <p class="mt-2 text-sm text-gray-500">{{ categoryInfo(result.category).description }}</p>
          <p class="mt-3 text-xs text-gray-400">Confidence {{ Math.round(result.confidence * 100) }}%</p>
        </template>
        <!-- The model is sure the item fits none of the bins: that is an answer, not a failure -->
        <template v-else-if="result.reason === 'unsupported_item'">
          <p class="text-sm text-gray-500">This item is</p>
          <p class="mt-2 inline-block rounded-full px-4 py-1 text-lg font-semibold" :class="categoryInfo('UNSUPPORTED').color">
            {{ categoryInfo('UNSUPPORTED').label }}
          </p>
          <p class="mt-2 text-sm text-gray-500">It does not belong in any of the bins.</p>
          <p class="mt-3 text-xs text-gray-400">Confidence {{ Math.round(result.confidence * 100) }}%</p>
        </template>
        <template v-else>
          <p class="font-semibold text-amber-700">No clear answer</p>
          <p class="mt-2 text-sm text-gray-600">{{ REASONS[result.reason] ?? 'Try another photo.' }}</p>
        </template>
        <button type="button" class="mt-5 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white" @click="reset">
          Check another item
        </button>
      </div>

      <div v-else-if="state === 'error'" class="mt-4 text-center">
        <p class="text-sm text-red-700">{{ error }}</p>
        <button type="button" class="mt-4 w-full rounded-xl bg-brand-600 py-3 font-semibold text-white" @click="reset">
          Try again
        </button>
      </div>
    </template>
  </div>
</template>
