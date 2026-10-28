<script setup>
import { ref } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { login } from '../api.js'

const route = useRoute()
const router = useRouter()

const username = ref('')
const password = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  loading.value = true
  try {
    await login(username.value, password.value)
    const redirect = typeof route.query.redirect === 'string' && route.query.redirect.startsWith('/')
      ? route.query.redirect
      : '/'
    router.replace(redirect)
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="pt-8">
    <h1 class="text-2xl font-bold">Welcome back</h1>
    <p class="mt-1 text-gray-500">Log in to scan items and keep your streak going.</p>

    <form class="mt-8 space-y-4" @submit.prevent="submit">
      <label class="block">
        <span class="text-sm font-medium text-gray-700">Username</span>
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          autocapitalize="none"
          required
          class="mt-1 w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
        />
      </label>

      <label class="block">
        <span class="text-sm font-medium text-gray-700">Password</span>
        <input
          v-model="password"
          type="password"
          autocomplete="current-password"
          required
          class="mt-1 w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
        />
      </label>

      <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>

      <button
        type="submit"
        :disabled="loading"
        class="w-full rounded-xl bg-brand-600 py-3 font-semibold text-white active:bg-brand-700 disabled:opacity-60"
      >
        {{ loading ? 'Logging in…' : 'Log in' }}
      </button>
    </form>

    <p class="mt-6 text-center text-sm text-gray-500">
      No account yet?
      <RouterLink to="/register" class="font-medium text-brand-600">Create one</RouterLink>
    </p>
  </section>
</template>
