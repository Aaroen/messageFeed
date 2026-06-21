import { readonly, ref } from 'vue'

export function useRefreshCompletionState() {
  const wasActive = ref(false)
  const wasSource = ref(false)
  const startedWithChrome = ref(false)
  const settling = ref(false)
  let settleTimer = 0

  function clearTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(settleTimer)
    }
  }

  function recordStartedWithChrome(startedWithVisibleChrome: boolean) {
    startedWithChrome.value = startedWithVisibleChrome
  }

  function begin(payload: { viewKey: string; startedWithVisibleChrome: boolean }) {
    wasActive.value = true
    wasSource.value = payload.viewKey.startsWith('source:')
    recordStartedWithChrome(payload.startedWithVisibleChrome)
  }

  function finish(delayMS: number) {
    const result = {
      wasActive: wasActive.value,
      wasSource: wasSource.value,
    }
    startedWithChrome.value = false
    wasActive.value = false
    wasSource.value = false
    clearTimer()
    settling.value = true
    settleTimer = window.setTimeout(() => {
      settling.value = false
    }, Math.max(0, delayMS))
    return result
  }

  function resetInactive() {
    startedWithChrome.value = false
    wasSource.value = false
  }

  return {
    wasActive: readonly(wasActive),
    wasSource: readonly(wasSource),
    startedWithChrome: readonly(startedWithChrome),
    settling: readonly(settling),
    recordStartedWithChrome,
    begin,
    finish,
    resetInactive,
    clearTimer,
  }
}
