<script setup>
import { computed, onMounted, ref } from 'vue'
import { createUser, deleteUser, getUsers } from '../../api.js'
import { session } from '../../auth.js'
import { percent, timeAgo } from '../../format.js'

const users = ref([])
const loaded = ref(false)
const error = ref('')
const search = ref('')

const showForm = ref(false)
const form = ref({ username: '', password: '', role: 'user' })
const formError = ref('')
const saving = ref(false)
const deleting = ref('')

async function load() {
  error.value = ''
  try {
    users.value = await getUsers()
  } catch {
    error.value = 'Could not load the users. Try again later.'
  } finally {
    loaded.value = true
  }
}

onMounted(load)

const totals = computed(() => {
  const admins = users.value.filter((user) => user.role === 'admin').length
  return [
    { label: 'Total', value: users.value.length },
    { label: 'Admins', value: admins },
    { label: 'Users', value: users.value.length - admins },
  ]
})

const filtered = computed(() => {
  const query = search.value.trim().toLowerCase()
  return query ? users.value.filter((user) => user.username.includes(query)) : users.value
})

async function submit() {
  formError.value = ''
  saving.value = true
  try {
    await createUser(form.value)
    form.value = { username: '', password: '', role: 'user' }
    showForm.value = false
    await load()
  } catch (e) {
    formError.value = e.message
  } finally {
    saving.value = false
  }
}

async function remove(username) {
  if (!window.confirm(`Delete ${username}? They are logged out at once and cannot log in again.`)) return
  error.value = ''
  deleting.value = username
  try {
    await deleteUser(username)
    users.value = users.value.filter((user) => user.username !== username)
  } catch (e) {
    error.value = e.message
  } finally {
    deleting.value = ''
  }
}
</script>

<template>
  <section class="space-y-4">
    <div class="flex items-start justify-between gap-3">
      <div>
        <h1 class="text-2xl font-bold">Users</h1>
        <p class="mt-1 text-sm text-gray-500">All accounts of every bin.</p>
      </div>
      <button
        v-if="!showForm"
        type="button"
        class="rounded-xl bg-brand-600 px-4 py-2 text-sm font-semibold text-white active:bg-brand-700"
        @click="showForm = true"
      >
        Add user
      </button>
    </div>

    <div class="grid grid-cols-3 gap-3">
      <div v-for="item in totals" :key="item.label" class="rounded-2xl bg-white p-4 text-center shadow-sm">
        <p class="text-2xl font-bold">{{ loaded ? item.value : '–' }}</p>
        <p class="text-xs text-gray-500">{{ item.label }}</p>
      </div>
    </div>

    <form v-if="showForm" class="space-y-3 rounded-2xl bg-white p-5 shadow-sm" @submit.prevent="submit">
      <p class="font-semibold">New user</p>

      <label class="block">
        <span class="text-sm font-medium text-gray-700">Username</span>
        <input
          v-model="form.username"
          type="text"
          autocomplete="off"
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
          v-model="form.password"
          type="password"
          autocomplete="new-password"
          minlength="12"
          maxlength="72"
          required
          class="mt-1 w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
        />
        <span class="mt-1 block text-xs text-gray-500">At least 12 characters, with letters and digits</span>
      </label>

      <fieldset>
        <legend class="text-sm font-medium text-gray-700">Role</legend>
        <div class="mt-1 grid grid-cols-2 gap-1 rounded-xl bg-gray-100 p-1 text-sm font-medium">
          <label
            v-for="role in ['user', 'admin']"
            :key="role"
            class="cursor-pointer rounded-lg py-2 text-center capitalize"
            :class="form.role === role ? 'bg-white text-gray-900 shadow-sm' : 'text-gray-500'"
          >
            <input v-model="form.role" type="radio" :value="role" class="sr-only" />
            {{ role }}
          </label>
        </div>
      </fieldset>

      <p v-if="formError" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ formError }}</p>

      <div class="grid grid-cols-2 gap-3">
        <button
          type="button"
          class="rounded-xl border border-gray-300 bg-white py-3 font-semibold text-gray-700"
          @click="showForm = false; formError = ''"
        >
          Cancel
        </button>
        <button
          type="submit"
          :disabled="saving"
          class="rounded-xl bg-brand-600 py-3 font-semibold text-white active:bg-brand-700 disabled:opacity-60"
        >
          {{ saving ? 'Creating…' : 'Create' }}
        </button>
      </div>
    </form>

    <input
      v-model="search"
      type="search"
      placeholder="Search by username"
      class="w-full rounded-xl border border-gray-300 bg-white px-4 py-3 outline-none focus:border-brand-600 focus:ring-2 focus:ring-brand-100"
    />

    <p v-if="error" class="rounded-xl bg-red-50 px-4 py-3 text-sm text-red-700">{{ error }}</p>

    <div class="rounded-2xl bg-white px-5 shadow-sm">
      <p v-if="!loaded" class="py-5 text-sm text-gray-500">Loading…</p>
      <p v-else-if="filtered.length === 0" class="py-5 text-sm text-gray-500">No users found.</p>
      <ul v-else class="divide-y divide-gray-100">
        <li v-for="user in filtered" :key="user.username" class="flex items-center gap-3 py-3">
          <div class="min-w-0 flex-1">
            <div class="flex flex-wrap items-center gap-2">
              <span class="truncate font-medium">{{ user.username }}</span>
              <span
                class="rounded-full px-2 py-0.5 text-xs font-medium"
                :class="user.role === 'admin' ? 'bg-brand-100 text-brand-700' : 'bg-gray-100 text-gray-600'"
              >
                {{ user.role }}
              </span>
              <span v-if="user.locked" class="rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-700">locked</span>
            </div>
            <p class="mt-1 text-xs text-gray-500">
              🔥 {{ user.current_streak }}-day streak ·
              <template v-if="user.total_scans">Accuracy {{ percent(user.sorted_scans, user.total_scans) }}%</template>
              <template v-else>No scans yet</template>
            </p>
            <p class="text-xs text-gray-400">
              {{ user.last_login_at ? `Last login ${timeAgo(user.last_login_at)}` : 'No login yet' }}
            </p>
          </div>

          <span v-if="user.username === session?.username" class="text-xs text-gray-400">you</span>
          <button
            v-else
            type="button"
            :disabled="deleting === user.username"
            class="rounded-lg px-3 py-1.5 text-sm font-medium text-red-600 active:bg-red-50 disabled:opacity-60"
            @click="remove(user.username)"
          >
            {{ deleting === user.username ? 'Deleting…' : 'Delete' }}
          </button>
        </li>
      </ul>
    </div>
  </section>
</template>
