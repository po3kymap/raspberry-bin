<script setup>
import { PERIODS } from '../periods.js'

defineProps({
  modelValue: { type: String, required: true },
})
const emit = defineEmits(['update:modelValue'])
</script>

<template>
  <div class="grid grid-cols-4 gap-1 rounded-xl bg-gray-100 p-1 text-xs font-medium">
    <button
      v-for="item in PERIODS"
      :key="item.id"
      type="button"
      class="rounded-lg py-2"
      :class="modelValue === item.id ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500'"
      :aria-pressed="modelValue === item.id"
      @click="emit('update:modelValue', item.id)"
    >
      {{ item.label }}
    </button>
  </div>
</template>
