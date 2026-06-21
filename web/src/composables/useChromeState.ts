import { readonly, ref } from 'vue'

export type ChromePhase =
  | 'hidden'
  | 'visible'
  | 'revealing'
  | 'hiding'
  | 'refreshing'
  | 'gesture-returning'

type SetChromeVisibleOptions = {
  settleDelayMS?: number
}

type SetChromeRefreshingProgressOptions = {
  contentCollapsed?: boolean
}

type RestoreChromeSnapshotOptions = {
  progress?: number
  contentCollapsed?: boolean
}

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
  let settlingTimerToken = 0

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

  function clearTimer() {
    if (typeof window !== 'undefined' && settlingTimer !== 0) {
      window.clearTimeout(settlingTimer)
    }
    settlingTimer = 0
    settlingTimerToken += 1
  }

  function scheduleSettlingEnd(delayMS: number) {
    clearTimer()
    const token = settlingTimerToken + 1
    settlingTimerToken = token
    settlingTimer = window.setTimeout(() => {
      if (token !== settlingTimerToken) {
        return
      }
      settlingTimer = 0
      setSettling(false)
    }, Math.max(0, delayMS))
  }

  function scheduleSettlingEndIfNeeded(delayMS?: number) {
    if (typeof delayMS !== 'number') {
      return
    }
    scheduleSettlingEnd(delayMS)
  }

  function setVisible(visible: boolean, options: SetChromeVisibleOptions = {}) {
    const nextProgress = visible ? 1 : 0
    if (progress.value === nextProgress) {
      if (visible && contentCollapsed.value) {
        setContentCollapsed(false)
      }
      if (!visible && contentCollapsed.value) {
        setSettling(true, 'hiding')
        scheduleSettlingEndIfNeeded(options.settleDelayMS)
        return
      }
      setProgress(nextProgress, visible ? 'visible' : 'hidden')
      return
    }

    setSettling(true, visible ? 'revealing' : 'hiding')
    if (visible) {
      setContentCollapsed(false)
    }
    setProgress(nextProgress, visible ? 'revealing' : 'hiding')
    scheduleSettlingEndIfNeeded(options.settleDelayMS)
  }

  function setCollapsedHidden(options: SetChromeVisibleOptions = {}) {
    setContentCollapsed(true)
    setVisible(false, options)
  }

  function setStableVisible() {
    clearTimer()
    progress.value = 1
    phase.value = 'visible'
    contentCollapsed.value = false
    settling.value = false
  }

  function beginGestureReturn(options: SetChromeVisibleOptions = {}) {
    setSettling(true, 'gesture-returning')
    setContentCollapsed(false)
    setProgress(1, 'gesture-returning')
    scheduleSettlingEndIfNeeded(options.settleDelayMS)
  }

  function beginRefreshing() {
    clearTimer()
    settling.value = false
    phase.value = 'refreshing'
  }

  function setRefreshingProgress(nextProgress: number, options: SetChromeRefreshingProgressOptions = {}) {
    setProgress(nextProgress, 'refreshing')
    if (options.contentCollapsed !== undefined) {
      setContentCollapsed(options.contentCollapsed)
    }
  }

  function commitRefreshing(startedWithVisibleChrome: boolean) {
    setContentCollapsed(!startedWithVisibleChrome)
    if (startedWithVisibleChrome) {
      setRefreshingProgress(1)
    }
  }

  function restoreSnapshot(snapshot: RestoreChromeSnapshotOptions) {
    setProgress(typeof snapshot.progress === 'number' ? snapshot.progress : 1)
    setContentCollapsed(Boolean(snapshot.contentCollapsed))
  }

  return {
    progress: readonly(progress),
    phase: readonly(phase),
    contentCollapsed: readonly(contentCollapsed),
    settling: readonly(settling),
    setVisible,
    setCollapsedHidden,
    setStableVisible,
    beginGestureReturn,
    beginRefreshing,
    setRefreshingProgress,
    commitRefreshing,
    restoreSnapshot,
    clearTimer,
  }
}
