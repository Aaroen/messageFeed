import { computed, ref } from 'vue'

export type SwipePhase = 'idle' | 'dragging' | 'settling' | 'canceled' | 'committed'
export type SwipeDirection = 'left' | 'right' | 'up' | 'down' | null

export type SwipeTransitionSnapshot<TSurface extends string = string> = {
  from: TSurface | null
  to: TSurface | null
  phase: SwipePhase
  direction: SwipeDirection
  progress: number
  isBlocked: boolean
}

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

export function useSwipeTransition<TSurface extends string = string>() {
  const from = ref<TSurface | null>(null)
  const to = ref<TSurface | null>(null)
  const phase = ref<SwipePhase>('idle')
  const direction = ref<SwipeDirection>(null)
  const progress = ref(0)
  const isBlocked = ref(false)
  let resetTimer = 0

  const snapshot = computed<SwipeTransitionSnapshot<TSurface>>(() => ({
    from: from.value,
    to: to.value,
    phase: phase.value,
    direction: direction.value,
    progress: progress.value,
    isBlocked: isBlocked.value,
  }))

  function begin(payload: {
    from: TSurface | null
    to?: TSurface | null
    direction: SwipeDirection
    progress?: number
    isBlocked?: boolean
  }) {
    from.value = payload.from
    to.value = payload.to ?? null
    direction.value = payload.direction
    progress.value = clampProgress(payload.progress ?? 0)
    isBlocked.value = Boolean(payload.isBlocked)
    phase.value = 'dragging'
  }

  function update(payload: {
    to?: TSurface | null
    direction?: SwipeDirection
    progress?: number
    isBlocked?: boolean
  }) {
    if (payload.to !== undefined) {
      to.value = payload.to
    }
    if (payload.direction !== undefined) {
      direction.value = payload.direction
    }
    if (payload.progress !== undefined) {
      progress.value = clampProgress(payload.progress)
    }
    if (payload.isBlocked !== undefined) {
      isBlocked.value = payload.isBlocked
    }
    if (phase.value === 'idle') {
      phase.value = 'dragging'
    }
  }

  function settle(committed: boolean, payload: { progress?: number; isBlocked?: boolean } = {}) {
    phase.value = committed ? 'committed' : 'canceled'
    progress.value = clampProgress(payload.progress ?? (committed ? 1 : 0))
    if (payload.isBlocked !== undefined) {
      isBlocked.value = payload.isBlocked
    }
  }

  function reset() {
    from.value = null
    to.value = null
    phase.value = 'idle'
    direction.value = null
    progress.value = 0
    isBlocked.value = false
  }

  function clearTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(resetTimer)
    }
  }

  function scheduleReset(delayMS: number) {
    clearTimer()
    resetTimer = window.setTimeout(() => {
      reset()
    }, Math.max(0, delayMS))
  }

  return {
    from,
    to,
    phase,
    direction,
    progress,
    isBlocked,
    snapshot,
    begin,
    update,
    settle,
    reset,
    scheduleReset,
    clearTimer,
  }
}
