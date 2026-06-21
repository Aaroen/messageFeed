export function useDoubleBackGuard(timeoutMS: number) {
  let lastAttemptAt = 0

  function reset() {
    lastAttemptAt = 0
  }

  function shouldConsumeBack() {
    const now = Date.now()
    if (now - lastAttemptAt <= timeoutMS) {
      reset()
      return false
    }
    lastAttemptAt = now
    return true
  }

  return {
    reset,
    shouldConsumeBack,
  }
}
