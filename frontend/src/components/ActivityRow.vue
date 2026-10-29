<script setup>
import { computed } from 'vue'
import ScanImage from './ScanImage.vue'

const props = defineProps({
  event: { type: Object, required: true },
})

const BADGES = {
  login: 'bg-blue-50 text-blue-700',
  logout: 'bg-blue-50 text-blue-700',
  login_failed: 'bg-red-50 text-red-700',
  register: 'bg-violet-50 text-violet-700',
  user_created: 'bg-violet-50 text-violet-700',
  user_deleted: 'bg-red-50 text-red-700',
  lid: 'bg-brand-50 text-brand-700',
  bin_connect: 'bg-gray-100 text-gray-700',
}

// "2026-10-29 17:08:47" in local time
const time = computed(() => new Date(props.event.at).toLocaleString('sv-SE'))

const badge = computed(() => {
  const e = props.event
  if (e.type === 'scan') return e.can_sort ? 'bg-brand-50 text-brand-700' : 'bg-amber-50 text-amber-700'
  return BADGES[e.type] ?? 'bg-gray-100 text-gray-700'
})

// key=value pairs in a fixed order per type; empty values are left out
const fields = computed(() => {
  const e = props.event
  const admin = ['user_created', 'user_deleted', 'lid', 'bin_connect'].includes(e.type)
  const pairs = [
    [admin ? 'by' : 'user', e.username],
    ['target', e.target],
    ['role', e.role],
    ['bin', e.bin_id],
    ['compartment', e.compartment],
    ['action', e.action],
    ['scan', e.scan_id],
    ['image', e.image_id],
    ['result', e.type === 'scan' ? e.category : null],
    ['confidence', e.type === 'scan' && e.confidence ? e.confidence.toFixed(2) : null],
    ['sorted', e.type === 'scan' ? String(e.can_sort) : null],
    ['reason', e.reason],
    ['ip', e.ip],
  ]
  return pairs.filter(([, value]) => value)
})
</script>

<template>
  <li class="py-2.5 font-mono text-[11px] leading-relaxed">
    <div class="flex flex-wrap items-center gap-x-2">
      <span class="text-gray-400">{{ time }}</span>
      <span class="rounded px-1.5 font-semibold uppercase" :class="badge">{{ event.type }}</span>
    </div>
    <p class="break-all text-gray-700">
      <span v-for="[key, value] in fields" :key="key" class="mr-2 inline-block">
        <span class="text-gray-400">{{ key }}=</span>{{ value }}
      </span>
    </p>
    <p v-if="event.id" class="break-all text-gray-400">event={{ event.id }}</p>
    <ScanImage v-if="event.image_id" :image-id="event.image_id" />
  </li>
</template>
