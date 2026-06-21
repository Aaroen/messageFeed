import { readonly, ref } from 'vue'

function normalizeProgress(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? Math.min(Math.max(value, 0), 1) : 1
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
