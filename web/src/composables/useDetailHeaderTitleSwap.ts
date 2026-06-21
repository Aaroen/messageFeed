import { ref } from 'vue'

import type { FeedItem } from '@/api/feed'

export function useDetailHeaderTitleSwap() {
  const previousTitle = ref('')
  const progress = ref(1)
  let timer = 0
  let frame = 0

  function begin(nextItem: FeedItem, currentItem: FeedItem | null) {
    if (!currentItem || currentItem.id === nextItem.id) {
      previousTitle.value = ''
      progress.value = 1
      return false
    }

    previousTitle.value = currentItem.title
    progress.value = 0
    return true
  }

  function commit() {
    progress.value = 1
  }

  function finish() {
    previousTitle.value = ''
  }

  function clearTimer() {
    window.clearTimeout(timer)
    if (frame) {
      cancelAnimationFrame(frame)
      frame = 0
    }
  }

  function reset() {
    clearTimer()
    previousTitle.value = ''
    progress.value = 1
  }

  function start(nextItem: FeedItem, currentItem: FeedItem | null, delay: number) {
    const shouldAnimate = begin(nextItem, currentItem)
    clearTimer()
    if (!shouldAnimate) {
      return
    }

    frame = requestAnimationFrame(() => {
      frame = 0
      commit()
    })
    timer = window.setTimeout(() => {
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
