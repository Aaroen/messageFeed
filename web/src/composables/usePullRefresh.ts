import { computed, ref } from 'vue'

export type PullRefreshOptions = {
  threshold?: number
  maxOffset?: number
  emptyMaxOffset?: number
}

export function usePullRefresh(options: PullRefreshOptions = {}) {
  const threshold = options.threshold ?? 76
  const maxOffset = options.maxOffset ?? 116
  const emptyMaxOffset = options.emptyMaxOffset ?? 88
  const offset = ref(0)
  const dragging = ref(false)
  const settling = ref(false)
  const refreshing = ref(false)
  const startedWithVisibleChrome = ref(false)

  const progress = computed(() => Math.min(offset.value / threshold, 1))
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

  function setOffset(nextOffset: number) {
    offset.value = Math.max(0, nextOffset)
  }

  function setRefreshing(nextRefreshing: boolean) {
    refreshing.value = nextRefreshing
  }

  function setSettling(nextSettling: boolean) {
    settling.value = nextSettling
  }

  function reset() {
    offset.value = 0
    dragging.value = false
    settling.value = false
    refreshing.value = false
    startedWithVisibleChrome.value = false
  }

  function resetGesture() {
    dragging.value = false
    startedWithVisibleChrome.value = false
  }

  return {
    threshold,
    maxOffset,
    emptyMaxOffset,
    offset,
    dragging,
    settling,
    refreshing,
    startedWithVisibleChrome,
    progress,
    active,
    rubberBandDistance,
    begin,
    startDragging,
    stopDragging,
    setOffset,
    setRefreshing,
    setSettling,
    reset,
    resetGesture,
  }
}
