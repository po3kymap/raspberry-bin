<script setup>
import { computed, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import { logout } from './api.js'
import { isAdmin, session } from './auth.js'
import { isAllowed, startPage } from './roles.js'

const route = useRoute()
const router = useRouter()

watch(session, (value) => {
  if (!value && !route.meta.guest) {
    router.replace({ name: 'login' })
  } else if (value && !route.meta.guest && !isAllowed(route)) {
    router.replace(startPage())
  }
})

async function onLogout() {
  await logout()
  router.replace({ name: 'login' })
}

const SETTINGS_ICON = 'M4 21v-7M4 10V3M12 21v-9M12 8V3M20 21v-5M20 12V3M1 14h6M9 8h6M17 16h6'

const USER_TABS = [
  { to: '/', label: 'Scan', icon: 'M3 7V5a2 2 0 0 1 2-2h2M17 3h2a2 2 0 0 1 2 2v2M21 17v2a2 2 0 0 1-2 2h-2M7 21H5a2 2 0 0 1-2-2v-2M7 12h10' },
  { to: '/stats', label: 'Statistics', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
  { to: '/settings', label: 'Settings', icon: SETTINGS_ICON },
]

const ADMIN_TABS = [
  { to: '/admin/overview', label: 'Overview', icon: 'M4 20V10M10 20V4M16 20v-7M22 20H2' },
  { to: '/admin/bins', label: 'Bins', icon: 'M3 6h18M8 6V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6' },
  { to: '/admin/users', label: 'Users', icon: 'M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0M23 21v-2a4 4 0 0 0-3-3.87M16 3.13a4 4 0 0 1 0 7.75' },
  { to: '/admin/settings', label: 'Settings', icon: SETTINGS_ICON },
]

// Admins get their own menu instead of the user one
const tabs = computed(() => (isAdmin() ? ADMIN_TABS : USER_TABS))

// A tab stays highlighted on its sub-pages, e.g. Overview on /admin/overview/sorting;
// a detail page for one bin (?bin=) is opened from the Bins tab, so Bins stays highlighted
function isActive(tab) {
  if (route.query.bin && route.path.startsWith('/admin/overview/')) return tab.to === '/admin/bins'
  return route.path === tab.to || (tab.to !== '/' && route.path.startsWith(`${tab.to}/`))
}
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
            class="flex flex-col items-center gap-1 py-2 text-xs"
            :class="isActive(tab) ? 'font-medium text-brand-600' : 'text-gray-500'"
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
