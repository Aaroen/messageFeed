import { computed, readonly, ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

export type SwipePhase = 'idle' | 'dragging' | 'settling' | 'canceled' | 'committed'
export type SwipeDirection = 'left' | 'right' | 'up' | 'down' | null
type SwipeSettleOutcome = Extract<SwipePhase, 'canceled' | 'committed'>

export type SwipeTransitionSnapshot<TSurface extends string = string> = {
  from: TSurface | null
  to: TSurface | null
  phase: SwipePhase
  direction: SwipeDirection
  progress: number
  isBlocked: boolean
}

export function useSwipeTransition<TSurface extends string = string>() {
  const from = ref<TSurface | null>(null)
  const to = ref<TSurface | null>(null)
  const phase = ref<SwipePhase>('idle')
  const direction = ref<SwipeDirection>(null)
  const progress = ref(0)
  const isBlocked = ref(false)
  const settleOutcome = ref<SwipeSettleOutcome | null>(null)
  let resetTimer = 0
  let resetFrame = 0
  let resetTimerToken = 0

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
    clearTimer()
    from.value = payload.from
    to.value = payload.to ?? null
    direction.value = payload.direction
    progress.value = clampProgress(payload.progress ?? 0)
    isBlocked.value = Boolean(payload.isBlocked)
    settleOutcome.value = null
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
    clearTimer()
    settleOutcome.value = committed ? 'committed' : 'canceled'
    phase.value = 'settling'
    progress.value = clampProgress(payload.progress ?? (committed ? 1 : 0))
    if (payload.isBlocked !== undefined) {
      isBlocked.value = payload.isBlocked
    }
  }

  function reset() {
    clearTimer()
    from.value = null
    to.value = null
    phase.value = 'idle'
    direction.value = null
    progress.value = 0
    isBlocked.value = false
    settleOutcome.value = null
  }

  function clearTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(resetTimer)
      window.cancelAnimationFrame(resetFrame)
    }
    resetTimer = 0
    resetFrame = 0
    resetTimerToken += 1
  }

  function completeSettlement() {
    if (phase.value === 'settling' && settleOutcome.value) {
      phase.value = settleOutcome.value
    }
  }

  function scheduleReset(delayMS: number) {
    clearTimer()
    const token = resetTimerToken + 1
    resetTimerToken = token
    resetTimer = window.setTimeout(() => {
      if (token !== resetTimerToken) {
        return
      }
      resetTimer = 0
      completeSettlement()
      resetFrame = window.requestAnimationFrame(() => {
        if (token !== resetTimerToken) {
          return
        }
        resetFrame = 0
        reset()
      })
    }, Math.max(0, delayMS))
  }

  return {
    from: readonly(from),
    to: readonly(to),
    phase: readonly(phase),
    direction: readonly(direction),
    progress: readonly(progress),
    isBlocked: readonly(isBlocked),
    snapshot,
    begin,
    update,
    settle,
    reset,
    scheduleReset,
    clearTimer,
  }
}
