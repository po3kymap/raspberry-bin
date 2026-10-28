<script setup>
import { ref } from 'vue'
import { RouterLink, useRouter } from 'vue-router'
import { login, register } from '../api.js'

const router = useRouter()

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')
const loading = ref(false)

async function submit() {
  error.value = ''
  if (password.value !== confirmPassword.value) {
    error.value = 'Passwords do not match.'
    return
  }

  loading.value = true
  try {
    await register(username.value, password.value)
    await login(username.value, password.value)
    router.replace('/')
  } catch (e) {
    error.value = e.message
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <section class="pt-8">
    <h1 class="text-2xl font-bold">Create account</h1>
    <p class="mt-1 text-gray-500">Track your sorting and build a streak.</p>

    <form class="mt-8 space-y-4" @submit.prevent="submit">
      <label class="block">
        <span class="text-sm font-medium text-gray-700">Username</span>
        <input
          v-model="username"
          type="text"
          autocomplete="username"
          autocapitalize="none"
          minlength="3"
          maxlength="32"
          required
          class="mt-1 w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
        />
        <span class="mt-1 block text-xs text-gray-500">3–32 characters: a–z, 0–9, ".", "_" or "-"</span>
      </label>

      <label class="block">
        <span class="text-sm font-medium text-gray-700">Password</span>
        <input
          v-model="password"
          type="password"
          autocomplete="new-password"
          minlength="12"
          maxlength="72"
          required
          class="mt-1 w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
        />
        <span class="mt-1 block text-xs text-gray-500">At least 12 characters, with letters and digits</span>
      </label>

      <label class="block">
        <span class="text-sm font-medium text-gray-700">Confirm password</span>
        <input
          v-model="confirmPassword"
          type="password"
          autocomplete="new-password"
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
        {{ loading ? 'Creating account…' : 'Create account' }}
      </button>
    </form>

    <p class="mt-6 text-center text-sm text-gray-500">
      Already have an account?
      <RouterLink to="/login" class="font-medium text-brand-600">Log in</RouterLink>
    </p>
  </section>
</template>
