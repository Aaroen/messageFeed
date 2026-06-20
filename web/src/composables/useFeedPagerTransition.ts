import { computed, ref } from 'vue'

type FeedPagerTransitionOptions = {
  getActiveKey: () => string | symbol | null | undefined
  getWindowWidth: () => number
  isFeedRoute: () => boolean
  isDetailReaderOpen: () => boolean
  dragThreshold?: number
}

function clamp(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

export function useFeedPagerTransition(options: FeedPagerTransitionOptions) {
  const dragThreshold = options.dragThreshold ?? 8
  const dragOffset = ref(0)
  const settling = ref(false)
  let settlingTimer = 0
  let delayedCommitTimer = 0

  const activeIndex = computed(() => (options.getActiveKey() === 'recommendations' ? 1 : 0))

  const trackStyle = computed(() => ({
    transform: `translate3d(calc(${-activeIndex.value * 100}% + ${cssPx(dragOffset.value)}), 0, 0)`,
  }))

  const swipeProgress = computed(() =>
    clamp(Math.abs(dragOffset.value) / Math.max(1, Math.min(options.getWindowWidth(), 320))),
  )

  const targetKey = computed(() => {
    if (dragOffset.value < -dragThreshold && activeIndex.value === 0) {
      return 'recommendations'
    }
    if (dragOffset.value > dragThreshold && activeIndex.value === 1) {
      return 'subscriptions'
    }
    return ''
  })

  const targetVisible = computed(() => options.isFeedRoute() && !options.isDetailReaderOpen() && Boolean(targetKey.value))
  const targetProgress = computed(() => (targetVisible.value ? clamp((swipeProgress.value - 0.26) / 0.48) : 0))

  function setDragOffset(nextOffset: number) {
    dragOffset.value = Number.isFinite(nextOffset) ? nextOffset : 0
  }

  function setSettling(nextSettling: boolean) {
    settling.value = nextSettling
  }

  function clearSettlingTimer() {
    window.clearTimeout(settlingTimer)
  }

  function clearDelayedCommitTimer() {
    window.clearTimeout(delayedCommitTimer)
  }

  function scheduleSettlingEnd(delay: number) {
    clearSettlingTimer()
    settlingTimer = window.setTimeout(() => {
      setSettling(false)
    }, Math.max(0, delay))
  }

  function scheduleDelayedCommit(delay: number, commit: () => void) {
    clearDelayedCommitTimer()
    delayedCommitTimer = window.setTimeout(commit, Math.max(0, delay))
  }

  function setDragDelta(deltaX: number) {
    if (activeIndex.value === 0) {
      setDragOffset(Math.min(0, Math.max(deltaX, -options.getWindowWidth())))
      return
    }
    setDragOffset(Math.max(0, Math.min(deltaX, options.getWindowWidth())))
  }

  function reset() {
    clearSettlingTimer()
    clearDelayedCommitTimer()
    dragOffset.value = 0
    settling.value = false
  }

  return {
    dragThreshold,
    dragOffset,
    settling,
    activeIndex,
    trackStyle,
    swipeProgress,
    targetKey,
    targetVisible,
    targetProgress,
    setDragOffset,
    setSettling,
    clearSettlingTimer,
    clearDelayedCommitTimer,
    scheduleSettlingEnd,
    scheduleDelayedCommit,
    setDragDelta,
    reset,
  }
}
