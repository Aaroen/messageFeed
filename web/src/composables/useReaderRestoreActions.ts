type CompleteDetailToSourceReaderTransitionOptions = {
  afterBegin?: () => void
  afterFinish?: () => void
}

type ReaderRestoreActionsOptions = {
  normalDuration: number
  readerDuration: number
  resolveDelay: (duration: number) => number
  restoreParkedSourceReaderWithDelay: (delay: number) => boolean
  restoreItemReaderExpansionWithDelay: (delay: number) => void
  restoreDetailFromSourceSwipeWithDelay: (delay: number) => void
  completeDetailToSourceReaderWithDelay: (
    delay: number,
    options?: CompleteDetailToSourceReaderTransitionOptions,
  ) => void
  resetBackSwipeOffset: () => void
  setTopChromeVisible: (visible: boolean) => void
  captureDetailSourceTransitionRects: (retry?: number, options?: { force?: boolean; lock?: boolean }) => void
  restoreMorphingItemContent: (unlockDelay?: number) => void
}

export function useReaderRestoreActions(options: ReaderRestoreActionsOptions) {
  function restoreParkedSourceReader(duration = options.normalDuration) {
    if (!options.restoreParkedSourceReaderWithDelay(options.resolveDelay(duration))) {
      options.resetBackSwipeOffset()
    }
  }

  function restoreItemReaderExpansion(duration = options.readerDuration) {
    options.restoreItemReaderExpansionWithDelay(options.resolveDelay(duration))
  }

  function restoreDetailFromSourceSwipe(duration = options.readerDuration) {
    options.restoreDetailFromSourceSwipeWithDelay(options.resolveDelay(duration))
  }

  function completeDetailToSourceReader(duration = options.readerDuration) {
    options.completeDetailToSourceReaderWithDelay(options.resolveDelay(duration), {
      afterBegin: () => {
        options.setTopChromeVisible(true)
        options.captureDetailSourceTransitionRects(12, { lock: true })
      },
      afterFinish: () => {
        options.restoreMorphingItemContent()
      },
    })
  }

  return {
    restoreParkedSourceReader,
    restoreItemReaderExpansion,
    restoreDetailFromSourceSwipe,
    completeDetailToSourceReader,
  }
}
