import { computed, ref } from 'vue'

import type { FeedItem } from '@/api/feed'

type DetailHeaderTitleSwapPhase = 'idle' | 'swapping'

export function useDetailHeaderTitleSwap() {
  const previousTitleText = ref('')
  const progress = ref(1)
  const phase = ref<DetailHeaderTitleSwapPhase>('idle')
  const previousTitle = computed(() => (phase.value === 'swapping' ? previousTitleText.value : ''))
  let timer = 0
  let frame = 0
  let swapToken = 0

  function begin(nextItem: FeedItem, currentItem: FeedItem | null) {
    if (!currentItem || currentItem.id === nextItem.id) {
      previousTitleText.value = ''
      progress.value = 1
      phase.value = 'idle'
      return false
    }

    previousTitleText.value = currentItem.title
    progress.value = 0
    phase.value = 'swapping'
    return true
  }

  function commit() {
    progress.value = 1
  }

  function finish() {
    previousTitleText.value = ''
    phase.value = 'idle'
  }

  function clearTimer() {
    swapToken += 1
    if (typeof window !== 'undefined' && timer !== 0) {
      window.clearTimeout(timer)
    }
    timer = 0
    if (frame) {
      cancelAnimationFrame(frame)
      frame = 0
    }
  }

  function reset() {
    clearTimer()
    previousTitleText.value = ''
    progress.value = 1
    phase.value = 'idle'
  }

  function start(nextItem: FeedItem, currentItem: FeedItem | null, delay: number) {
    const shouldAnimate = begin(nextItem, currentItem)
    clearTimer()
    if (!shouldAnimate) {
      return
    }

    const token = swapToken
    frame = requestAnimationFrame(() => {
      if (token !== swapToken) {
        return
      }
      frame = 0
      commit()
    })
    timer = window.setTimeout(() => {
      if (token !== swapToken) {
        return
      }
      timer = 0
      finish()
    }, delay)
  }

  return {
    previousTitle,
    progress,
    start,
    reset,
    clearTimer,
  }
}
