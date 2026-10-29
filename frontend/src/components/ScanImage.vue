<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { getScanImageUrl } from '../api.js'

const props = defineProps({
  imageId: { type: String, required: true },
})

const url = ref(null)
const state = ref('loading')
const large = ref(false)

onMounted(async () => {
  try {
    url.value = await getScanImageUrl(props.imageId)
    state.value = url.value ? 'ready' : 'missing'
  } catch {
    state.value = 'missing'
  }
})

onBeforeUnmount(() => {
  if (url.value) URL.revokeObjectURL(url.value)
})
</script>

<template>
  <button
    v-if="state === 'ready'"
    type="button"
    class="mt-1 block overflow-hidden rounded-lg border border-gray-200"
    :class="large ? 'w-full' : 'size-16'"
    :aria-label="large ? 'Make the photo smaller' : 'Show the photo larger'"
    @click="large = !large"
  >
    <img :src="url" :alt="`Scan photo ${imageId}`" class="size-full object-cover" />
  </button>
  <p v-else class="mt-1 text-gray-400">{{ state === 'loading' ? 'loading photo…' : 'photo not available' }}</p>
</template>
