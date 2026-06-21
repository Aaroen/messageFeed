import { computed, ref } from 'vue'

export type PullRefreshOptions = {
  threshold?: number
  maxOffset?: number
  emptyMaxOffset?: number
}

type PullRefreshSettleCompletionOptions = {
  releaseDelayMS?: number
  settleDelayMS: number
  afterRelease?: () => void
}

export function usePullRefresh(options: PullRefreshOptions = {}) {
  const threshold = options.threshold ?? 76
  const maxOffset = options.maxOffset ?? 116
  const emptyMaxOffset = options.emptyMaxOffset ?? 88
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

  const progress = computed(() => Math.min(offset.value / threshold, 1))
  const distanceProgress = computed(() => Math.min(distance.value / threshold, 1))
  const active = computed(() => offset.value > 0 || refreshing.value)

  function rubberBandDistance(distance: number, hasItems: boolean) {
    const limit = hasItems ? maxOffset : emptyMaxOffset
    if (distance <= threshold) {
      return Math.max(0, distance)
    }
    return Math.min(threshold + (distance - threshold) * 0.18, limit)
  }

  function begin(startedWithChrome: boolean) {
    startedWithVisibleChrome.value = startedWithChrome
    settling.value = false
  }

  function startDragging() {
    dragging.value = true
  }

  function stopDragging() {
    dragging.value = false
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
    stopDragging()
    setDistance(nextDistance)
  }

  function setOffset(nextOffset: number) {
    offset.value = Math.max(0, nextOffset)
  }

  function setDistance(nextDistance: number) {
    distance.value = Math.max(0, nextDistance)
  }

  function setRefreshing(nextRefreshing: boolean) {
    refreshing.value = nextRefreshing
  }

  function beginRefreshing() {
    refreshing.value = true
    setDistance(threshold)
  }

  function finishRefreshing() {
    refreshing.value = false
    setDistance(0)
  }

  function resetMotion() {
    setOffset(0)
    setDistance(0)
    settling.value = false
  }

  function finishBackgroundRefresh() {
    setOffset(0)
    finishRefreshing()
    resetGesture()
    settling.value = false
  }

  function setSettling(nextSettling: boolean) {
    settling.value = nextSettling
  }

  function clearSettleTimer() {
    if (typeof window !== 'undefined') {
      window.clearTimeout(settleTimer)
      window.clearTimeout(releaseTimer)
    }
  }

  function settleOffset(delayMS: number) {
    clearSettleTimer()
    settling.value = true
    offset.value = 0
    settleTimer = window.setTimeout(() => {
      settling.value = false
    }, Math.max(0, delayMS))
  }

  function settleRefreshCompletion(options: PullRefreshSettleCompletionOptions) {
    clearSettleTimer()
    settling.value = true
    releaseTimer = window.setTimeout(() => {
      setOffset(0)
      finishRefreshing()
      resetGesture()
      options.afterRelease?.()
      settleTimer = window.setTimeout(() => {
        settling.value = false
      }, Math.max(0, options.settleDelayMS))
    }, Math.max(0, options.releaseDelayMS ?? 0))
  }

  function reset() {
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
    offset,
    distance,
    dragging,
    settling,
    refreshing,
    startedWithVisibleChrome,
    gestureDistance,
    gestureCandidate,
    gestureTracking,
    progress,
    distanceProgress,
    active,
    rubberBandDistance,
    begin,
    stopDragging,
    beginGestureCandidate,
    gestureDelta,
    beginGestureTracking,
    updateGestureDistance,
    resetGestureTracking,
    setOffset,
    beginRefreshing,
    finishRefreshing,
    resetMotion,
    finishBackgroundRefresh,
    setSettling,
    clearSettleTimer,
    settleOffset,
    settleRefreshCompletion,
    reset,
    resetGesture,
  }
}
