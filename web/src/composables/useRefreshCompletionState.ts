import { readonly, ref } from 'vue'

export function useRefreshCompletionState() {
  const wasActive = ref(false)
  const wasSource = ref(false)
  const startedWithChrome = ref(false)
  const settling = ref(false)
  const settlingSource = ref(false)
  let settleTimer = 0
  let settleTimerToken = 0

  function clearTimer() {
    if (typeof window !== 'undefined' && settleTimer !== 0) {
      window.clearTimeout(settleTimer)
    }
    settleTimer = 0
    settleTimerToken += 1
  }

  function recordStartedWithChrome(startedWithVisibleChrome: boolean) {
    startedWithChrome.value = startedWithVisibleChrome
  }

  function begin(payload: { viewKey: string; startedWithVisibleChrome: boolean }) {
    clearTimer()
    settling.value = false
    wasActive.value = true
    wasSource.value = payload.viewKey.startsWith('source:')
    recordStartedWithChrome(payload.startedWithVisibleChrome)
  }

  function finish(delayMS: number) {
    const result = {
      wasActive: wasActive.value,
      wasSource: wasSource.value,
    }
    settlingSource.value = wasSource.value
    wasActive.value = false
    wasSource.value = false
    clearTimer()
    const token = settleTimerToken + 1
    settleTimerToken = token
    settling.value = true
    settleTimer = window.setTimeout(() => {
      if (token !== settleTimerToken) {
        return
      }
      settleTimer = 0
      startedWithChrome.value = false
      settling.value = false
      settlingSource.value = false
    }, Math.max(0, delayMS))
    return result
  }

  function resetInactive() {
    if (!settling.value) {
      startedWithChrome.value = false
    }
    wasSource.value = false
    if (!settling.value) {
      settlingSource.value = false
    }
  }

  function reset() {
    clearTimer()
    wasActive.value = false
    wasSource.value = false
    startedWithChrome.value = false
    settling.value = false
    settlingSource.value = false
  }

  return {
    wasActive: readonly(wasActive),
    wasSource: readonly(wasSource),
    startedWithChrome: readonly(startedWithChrome),
    settling: readonly(settling),
    settlingSource: readonly(settlingSource),
    recordStartedWithChrome,
    begin,
    finish,
    resetInactive,
    reset,
    clearTimer,
  }
}
