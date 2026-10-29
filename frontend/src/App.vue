<script setup>
import { computed, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { logout } from './api.js'
import { isAdmin, session } from './auth.js'

const route = useRoute()
const router = useRouter()

watch(session, (value) => {
  if (!value && !route.meta.guest && !route.meta.public) {
    router.replace({ name: 'login' })
  } else if (value && route.meta.admin && !isAdmin()) {
    router.replace({ name: 'home' })
  }
})

async function onLogout() {
  await logout()
  router.replace({ name: 'login' })
}

const tabs = computed(() => [
  { to: '/', label: 'Scan', icon: 'M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2M7 12h10' },
  { to: '/stats', label: 'Statistics', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
  { to: '/settings', label: 'Settings', icon: 'M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6' },
  // The Admin tab is shown to admins only
  ...(isAdmin() ? [{ to: '/admin', label: 'Admin', icon: 'M12 3l8 4v5c0 5-3.5 8-8 9-4.5-1-8-4-8-9V7l8-4z' }] : []),
])
</script>

<template>
  <div class="mx-auto flex min-h-dvh max-w-md flex-col bg-gray-50">
    <header class="flex items-center justify-between px-4 pt-[max(1rem,env(safe-area-inset-top))] pb-3">
      <span class="text-lg font-semibold">Raspberry<span class="text-brand-600">Bin</span></span>
      <div v-if="session" class="flex items-center gap-3 text-sm">
        <span class="text-gray-500">{{ session.username }}</span>
        <button type="button" class="font-medium text-brand-600" @click="onLogout">Log out</button>
      </div>
    </header>

    <main class="flex-1 px-4 pb-24">
      <RouterView />
    </main>

    <nav
      v-if="!route.meta.guest"
      class="fixed inset-x-0 bottom-0 border-t border-gray-200 bg-white pb-[env(safe-area-inset-bottom)]"
    >
      <ul class="mx-auto flex max-w-md">
        <li v-for="tab in tabs" :key="tab.to" class="flex-1">
          <RouterLink
            :to="tab.to"
            class="flex flex-col items-center gap-1 py-2 text-xs text-gray-500"
            active-class="!text-brand-600 font-medium"
            exact-active-class="!text-brand-600 font-medium"
          >
            <svg viewBox="0 0 24 24" class="size-6" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
              <path :d="tab.icon" />
            </svg>
            {{ tab.label }}
          </RouterLink>
        </li>
      </ul>
    </nav>
  </div>
</template>
