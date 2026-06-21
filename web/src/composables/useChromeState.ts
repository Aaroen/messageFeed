import { ref } from 'vue'

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
    clearSettlingTimer()
    progress.value = 1
    phase.value = 'visible'
    contentCollapsed.value = false
    settling.value = false
  }

  function beginRefreshing() {
    clearSettlingTimer()
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
    progress,
    phase,
    contentCollapsed,
    settling,
    setVisible,
    setCollapsedHidden,
    setStableVisible,
    beginRefreshing,
    setRefreshingProgress,
    commitRefreshing,
    restoreSnapshot,
    clearSettlingTimer,
  }
}
