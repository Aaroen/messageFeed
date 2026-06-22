import { computed, readonly, ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'
import { useMotionTimings } from '@/composables/useMotionTimings'

export type PullRefreshOptions = {
  threshold?: number
  maxOffset?: number
  emptyMaxOffset?: number
  completionReleaseDelayMS?: number
  completionSettleDelayMS?: number
}

type PullRefreshSettleCompletionOptions = {
  releaseDelayMS?: number
  settleDelayMS?: number
  afterRelease?: () => void
  afterSettled?: () => void
}

export function usePullRefresh(options: PullRefreshOptions = {}) {
  const motionTimings = useMotionTimings()
  const threshold = options.threshold ?? 76
  const maxOffset = options.maxOffset ?? 116
  const emptyMaxOffset = options.emptyMaxOffset ?? 88
  const completionReleaseDelayMS = options.completionReleaseDelayMS ?? motionTimings.topRefreshReleaseDelay
  const completionSettleDelayMS =
    options.completionSettleDelayMS ?? motionTimings.topRefreshSettleDuration + motionTimings.motionCleanupBuffer
  const offset = ref(0)
  const distance = ref(0)
  const dragging = ref(false)
  const settling = ref(false)
  const refreshing = ref(false)
  const startedWithVisibleChrome = ref(false)
  const gestureStartX = ref(0)
  const gestureStartY = ref(0)
  const gestureDistance = ref(0)
  const gestureCandidate = ref(false)
  const gestureTracking = ref(false)
  let settleTimer = 0
  let releaseTimer = 0
  let timerToken = 0

  const progress = computed(() => clampProgress(offset.value / threshold))
  const distanceProgress = computed(() => clampProgress(distance.value / threshold))
  const active = computed(() => offset.value > 0 || refreshing.value)

  function nonNegativeDistance(value: number) {
    return Number.isFinite(value) ? Math.max(0, value) : 0
  }

  function rubberBandDistance(distance: number, hasItems: boolean) {
    const safeDistance = nonNegativeDistance(distance)
    const limit = Math.max(threshold, nonNegativeDistance(hasItems ? maxOffset : emptyMaxOffset))
    if (safeDistance <= threshold) {
      return safeDistance
    }
    return Math.min(threshold + (safeDistance - threshold) * 0.18, limit)
  }

  function begin(startedWithChrome: boolean) {
    startedWithVisibleChrome.value = startedWithChrome
    settling.value = false
  }

  function startDragging() {
    dragging.value = true
  }

  function beginGestureCandidate(startX: number, startY: number) {
    gestureStartX.value = Number.isFinite(startX) ? startX : 0
    gestureStartY.value = Number.isFinite(startY) ? startY : 0
    gestureDistance.value = 0
    gestureCandidate.value = true
    gestureTracking.value = false
  }

  function gestureDelta(clientX: number, clientY: number) {
    return {
      deltaX: clientX - gestureStartX.value,
      deltaY: clientY - gestureStartY.value,
    }
  }

  function beginGestureTracking() {
    gestureTracking.value = true
    gestureCandidate.value = false
    startDragging()
  }

  function updateGestureDistance(deltaY: number) {
    gestureDistance.value = Math.max(gestureDistance.value, Number.isFinite(deltaY) ? deltaY : 0)
    setDistance(gestureDistance.value)
    return gestureDistance.value
  }

  function resetGestureTracking(nextDistance = refreshing.value ? threshold : 0) {
    gestureStartX.value = 0
    gestureStartY.value = 0
    gestureDistance.value = 0
    gestureCandidate.value = false
    gestureTracking.value = false
    dragging.value = false
    setDistance(nextDistance)
  }

  function finishGestureTracking() {
    resetGestureTracking()
  }

  function setOffset(nextOffset: number) {
    offset.value = nonNegativeDistance(nextOffset)
  }

  function resetOffset() {
    setOffset(0)
  }

  function cancelGesture() {
    resetGestureTracking(0)
    resetOffset()
    resetGesture()
  }

  function commitRefreshOffset() {
    setOffset(threshold)
  }

  function setGestureOffset(nextOffset: number) {
    clearTimers()
    settling.value = false
    setOffset(nextOffset)
  }

  function setDistance(nextDistance: number) {
    distance.value = nonNegativeDistance(nextDistance)
  }

  function beginRefreshing() {
    refreshing.value = true
    setDistance(threshold)
  }

  function finishRefreshingState() {
    dragging.value = false
    refreshing.value = false
    setDistance(0)
  }

  function finishRefreshing() {
    clearTimers()
    finishRefreshingState()
  }

  function resetMotion() {
    clearTimers()
    setOffset(0)
    setDistance(0)
    settling.value = false
  }

  function finishBackgroundRefresh() {
    clearTimers()
    setOffset(0)
    finishRefreshingState()
    resetGesture()
    settling.value = false
  }

  function setSettling(nextSettling: boolean) {
    settling.value = nextSettling
  }

  function clearTimers() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(settleTimer)
      window.clearTimeout(releaseTimer)
    }
    settleTimer = 0
    releaseTimer = 0
    timerToken += 1
  }

  function settleOffset(delayMS: number) {
    clearTimers()
    const token = timerToken + 1
    timerToken = token
    settling.value = true
    offset.value = 0
    settleTimer = window.setTimeout(() => {
      if (token !== timerToken) {
        return
      }
      settleTimer = 0
      settling.value = false
    }, Math.max(0, delayMS))
  }

  function settleRefreshCompletion(options: PullRefreshSettleCompletionOptions) {
    clearTimers()
    const token = timerToken + 1
    timerToken = token
    dragging.value = false
    settling.value = true
    releaseTimer = window.setTimeout(() => {
      if (token !== timerToken) {
        return
      }
      releaseTimer = 0
      setOffset(0)
      finishRefreshingState()
      resetGesture()
      options.afterRelease?.()
      settleTimer = window.setTimeout(() => {
        if (token !== timerToken) {
          return
        }
        settleTimer = 0
        settling.value = false
        options.afterSettled?.()
      }, Math.max(0, options.settleDelayMS ?? completionSettleDelayMS))
    }, Math.max(0, options.releaseDelayMS ?? completionReleaseDelayMS))
  }

  function reset() {
    clearTimers()
    offset.value = 0
    distance.value = 0
    dragging.value = false
    settling.value = false
    refreshing.value = false
    startedWithVisibleChrome.value = false
    gestureStartX.value = 0
    gestureStartY.value = 0
    gestureDistance.value = 0
    gestureCandidate.value = false
    gestureTracking.value = false
  }

  function resetGesture() {
    dragging.value = false
    startedWithVisibleChrome.value = false
    gestureCandidate.value = false
    gestureTracking.value = false
  }

  return {
    threshold,
    maxOffset,
    emptyMaxOffset,
    offset: readonly(offset),
    distance: readonly(distance),
    dragging: readonly(dragging),
    settling: readonly(settling),
    refreshing: readonly(refreshing),
    startedWithVisibleChrome: readonly(startedWithVisibleChrome),
    gestureDistance: readonly(gestureDistance),
    gestureCandidate: readonly(gestureCandidate),
    gestureTracking: readonly(gestureTracking),
    progress,
    distanceProgress,
    active,
    rubberBandDistance,
    begin,
    beginGestureCandidate,
    gestureDelta,
    beginGestureTracking,
    updateGestureDistance,
    finishGestureTracking,
    cancelGesture,
    commitRefreshOffset,
    setGestureOffset,
    beginRefreshing,
    finishRefreshing,
    resetMotion,
    finishBackgroundRefresh,
    clearTimers,
    settleOffset,
    settleRefreshCompletion,
    reset,
  }
}
