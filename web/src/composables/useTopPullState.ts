import { readonly, ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

function normalizeProgress(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? clampProgress(value) : 1
}

export function useTopPullState() {
  const pulling = ref(false)
  const startedWithChrome = ref(false)
  const startProgress = ref(1)

  function begin(nextStartedWithChrome: boolean, nextStartProgress = 1) {
    pulling.value = true
    startedWithChrome.value = nextStartedWithChrome
    startProgress.value = normalizeProgress(nextStartProgress)
  }

  function finish() {
    pulling.value = false
  }

  function markStartedWithChrome() {
    startedWithChrome.value = true
  }

  function resetStartedWithChrome() {
    startedWithChrome.value = false
  }

  function reset() {
    pulling.value = false
    startedWithChrome.value = false
  }

  return {
    pulling: readonly(pulling),
    startedWithChrome: readonly(startedWithChrome),
    startProgress: readonly(startProgress),
    begin,
    finish,
    markStartedWithChrome,
    resetStartedWithChrome,
    reset,
  }
}
