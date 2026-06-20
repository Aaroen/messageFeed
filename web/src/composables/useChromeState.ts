import { ref } from 'vue'

export type ChromePhase =
  | 'hidden'
  | 'visible'
  | 'revealing'
  | 'hiding'
  | 'refreshing'
  | 'gesture-returning'

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

function phaseFromProgress(progress: number): ChromePhase {
  if (progress <= 0.01) {
    return 'hidden'
  }
  if (progress >= 0.99) {
    return 'visible'
  }
  return 'revealing'
}

export function useChromeState() {
  const progress = ref(1)
  const phase = ref<ChromePhase>('visible')
  const contentCollapsed = ref(false)
  const settling = ref(false)
  let settlingTimer = 0

  function setPhase(nextPhase: ChromePhase) {
    phase.value = nextPhase
  }

  function setProgress(nextProgress: number, nextPhase?: ChromePhase) {
    const safeProgress = clampProgress(nextProgress)
    progress.value = safeProgress
    phase.value = nextPhase ?? phaseFromProgress(safeProgress)
  }

  function setContentCollapsed(collapsed: boolean) {
    contentCollapsed.value = collapsed
    if (collapsed && progress.value <= 0.01) {
      phase.value = 'hidden'
    }
  }

  function setSettling(nextSettling: boolean, nextPhase?: ChromePhase) {
    settling.value = nextSettling
    if (nextPhase) {
      phase.value = nextPhase
    } else if (!nextSettling) {
      phase.value = phaseFromProgress(progress.value)
    }
  }

  function clearSettlingTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(settlingTimer)
    }
  }

  function scheduleSettlingEnd(delayMS: number) {
    clearSettlingTimer()
    settlingTimer = window.setTimeout(() => {
      setSettling(false)
    }, Math.max(0, delayMS))
  }

  return {
    progress,
    phase,
    contentCollapsed,
    settling,
    setPhase,
    setProgress,
    setContentCollapsed,
    setSettling,
    clearSettlingTimer,
    scheduleSettlingEnd,
  }
}
