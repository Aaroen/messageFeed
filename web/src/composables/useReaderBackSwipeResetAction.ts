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
  function clearStretchAnchors(delay = options.stretchAnchorClearDuration) {
    window.setTimeout(() => {
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
    clearStretchAnchors,
    resetBackSwipeOffset,
  }
}
