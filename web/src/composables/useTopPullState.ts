import { readonly, ref } from 'vue'

export function useTopPullState() {
  const pulling = ref(false)
  const startedWithChrome = ref(false)

  function begin(nextStartedWithChrome: boolean) {
    pulling.value = true
    startedWithChrome.value = nextStartedWithChrome
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
    begin,
    finish,
    markStartedWithChrome,
    resetStartedWithChrome,
    reset,
  }
}
