type ReadableRef<T> = {
  readonly value: T
}

type RestoreDetailFromParkedSourceTransitionOptions = {
  beforeBegin?: () => void
  afterBegin?: () => void
  afterFinish?: () => void
}

type ReaderParkedDetailRestoreActionOptions = {
  detailReaderOpen: ReadableRef<boolean>
  readerDuration: number
  resolveDelay: (duration: number) => number
  closeSourceReader: () => void
  suppressFollowingClick: () => void
  restoreDetailFromParkedSourceWithDelay: (
    delay: number,
    options?: RestoreDetailFromParkedSourceTransitionOptions,
  ) => boolean
  clearMorphingHeightUnlockTimer: () => void
  captureVisibleSourceReturnTarget: () => boolean
  setTopChromeVisible: (visible: boolean) => void
  restoreMorphingItemContent: () => void
  scheduleHiddenSourceReaderCleanup: () => void
}

export function useReaderParkedDetailRestoreAction(options: ReaderParkedDetailRestoreActionOptions) {
  function restoreDetailFromParkedSource(duration = options.readerDuration) {
    if (!options.detailReaderOpen.value) {
      options.closeSourceReader()
      return
    }

    options.suppressFollowingClick()
    options.restoreDetailFromParkedSourceWithDelay(options.resolveDelay(duration), {
      beforeBegin: () => {
        options.clearMorphingHeightUnlockTimer()
        options.captureVisibleSourceReturnTarget()
      },
      afterBegin: () => {
        options.setTopChromeVisible(true)
      },
      afterFinish: () => {
        options.restoreMorphingItemContent()
        options.scheduleHiddenSourceReaderCleanup()
      },
    })
  }

  return {
    restoreDetailFromParkedSource,
  }
}
