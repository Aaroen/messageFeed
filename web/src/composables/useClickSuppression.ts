import { ref } from 'vue'

export function useClickSuppression(durationMS = 420) {
  const suppressNextClick = ref(false)
  let timer = 0

  function clearTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(timer)
    }
  }

  function suppressNext() {
    suppressNextClick.value = true
    clearTimer()
    timer = window.setTimeout(() => {
      suppressNextClick.value = false
    }, durationMS)
  }

  function consume(event: MouseEvent) {
    if (!suppressNextClick.value) {
      return false
    }
    event.preventDefault()
    event.stopPropagation()
    suppressNextClick.value = false
    clearTimer()
    return true
  }

  return {
    suppressNextClick,
    suppressNext,
    consume,
    clearTimer,
  }
}
