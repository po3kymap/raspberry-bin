<script setup>
import { connectedBin, disconnectBin } from '../bin.js'
</script>

<template>
  <div class="flex items-center gap-3 rounded-2xl bg-white px-4 py-3 shadow-sm">
    <span class="size-2.5 rounded-full bg-brand-500" aria-hidden="true"></span>
    <p class="flex-1 text-sm">Connected to <span class="font-semibold">{{ connectedBin }}</span></p>
    <button type="button" class="text-sm font-medium text-brand-600" @click="disconnectBin">Disconnect</button>
  </div>
</template>
