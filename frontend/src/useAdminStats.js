import { computed, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { getAdminStats } from './api.js'
import { PERIODS, periodRange } from './periods.js'

const BIN_ID_PATTERN = /^[A-Za-z0-9_-]{1,32}$/

// Statistics for the overview pages: all bins, or one bin with ?bin=bin-27
// (opened from the Bins tab). The period lives in the URL too (?period=month),
// so the detail pages open with the same period.
export function useAdminStats() {
  const route = useRoute()
  const router = useRouter()

  const period = computed({
    get: () => (PERIODS.some((p) => p.id === route.query.period) ? route.query.period : 'week'),
    set: (id) => router.replace({ query: { ...route.query, period: id } }),
  })

  const binId = computed(() =>
    typeof route.query.bin === 'string' && BIN_ID_PATTERN.test(route.query.bin) ? route.query.bin : null,
  )

  const data = ref(null)
  const loading = ref(false)
  const error = ref('')
  let latestRequest = 0

  // Another period keeps the old numbers dimmed while loading
  async function load() {
    const requestId = ++latestRequest
    loading.value = true
    error.value = ''
    try {
      const response = await getAdminStats({ ...periodRange(period.value), binId: binId.value })
      if (requestId === latestRequest) data.value = response
    } catch {
      if (requestId === latestRequest) error.value = 'Could not load the statistics. Try again later.'
    } finally {
      if (requestId === latestRequest) loading.value = false
    }
  }

  watch([period, binId], load, { immediate: true })

  return { period, binId, data, summary: computed(() => data.value?.summary), loading, error }
}
