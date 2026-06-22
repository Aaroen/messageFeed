import { ref } from 'vue'

export function useClickSuppression(durationMS: number) {
  const suppressNextClick = ref(false)
  let timer = 0
  let timerToken = 0

  function clearTimer() {
    timerToken += 1
    if (typeof window !== 'undefined' && timer !== 0) {
      window.clearTimeout(timer)
    }
    timer = 0
  }

  function suppressNext() {
    suppressNextClick.value = true
    clearTimer()
    const token = timerToken
    timer = window.setTimeout(() => {
      if (token !== timerToken) {
        return
      }
      timer = 0
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
