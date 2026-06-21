type ReadableRef<T> = {
  readonly value: T
}

type ReaderBackSwipeResetActionOptions = {
  readerBackDragging: ReadableRef<boolean>
  stretchAnchorClearDuration: number
  resetReaderBackSwipeDragState: () => void
  resetPageSideMotion: () => void
  clearReaderStretchAnchorsIfIdle: () => void
  clearPageStretchAnchorIfIdle: (readerBackDragging: boolean) => void
}

export function useReaderBackSwipeResetAction(options: ReaderBackSwipeResetActionOptions) {
  let stretchAnchorTimer = 0

  function clearStretchAnchorTimer() {
    if (typeof window !== 'undefined' && stretchAnchorTimer !== 0) {
      window.clearTimeout(stretchAnchorTimer)
    }
    stretchAnchorTimer = 0
  }

  function clearStretchAnchors(delay = options.stretchAnchorClearDuration) {
    clearStretchAnchorTimer()
    stretchAnchorTimer = window.setTimeout(() => {
      stretchAnchorTimer = 0
      options.clearReaderStretchAnchorsIfIdle()
      options.clearPageStretchAnchorIfIdle(options.readerBackDragging.value)
    }, delay)
  }

  function resetBackSwipeOffset() {
    options.resetReaderBackSwipeDragState()
    options.resetPageSideMotion()
    clearStretchAnchors()
  }

  return {
    clearStretchAnchorTimer,
    clearStretchAnchors,
    resetBackSwipeOffset,
  }
}
