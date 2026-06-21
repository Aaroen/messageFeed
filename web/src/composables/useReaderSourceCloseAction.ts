type ReadableRef<T> = {
  readonly value: T
}

type RestoreParkedDetailOptions = {
  onDetailScrollTop?: (scrollTop: number) => void
}

type RestoreSourceReaderBackTargetResult =
  | {
      action: 'restore-detail'
    }
  | {
      action: 'close-source'
    }

type ReaderSourceCloseActionOptions = {
  sourceReaderOpen: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  isFeedRoute: ReadableRef<boolean>
  sourceReaderShouldReturnToDetail: () => boolean
  hasDetailParkedBehindSource: () => boolean
  restorePreviousParkedDetailIfReaderClosed: (options?: RestoreParkedDetailOptions) => boolean
  restoreSourceReaderBackTargetState: (
    options?: RestoreParkedDetailOptions,
  ) => RestoreSourceReaderBackTargetResult
  closeVisibleSourceReaderState: () => void
  clearSourceReaderState: () => void
  resetSourceSubscriptionState: () => void
  rememberDetailScrollTop: (scrollTop: number) => void
  restoreDetailFromParkedSource: () => void
  setTopChromeVisible: (visible: boolean) => void
  scheduleHiddenSourceReaderCleanup: (delay?: number) => void
}

export function useReaderSourceCloseAction(options: ReaderSourceCloseActionOptions) {
  function restoreSourceReaderBackTarget() {
    const result = options.restoreSourceReaderBackTargetState({
      onDetailScrollTop: options.rememberDetailScrollTop,
    })
    if (result.action === 'restore-detail') {
      options.restoreDetailFromParkedSource()
      return
    }

    closeSourceReader()
  }

  function closeSourceReader() {
    if (options.sourceReaderShouldReturnToDetail()) {
      restoreSourceReaderBackTarget()
      return
    }

    if (options.hasDetailParkedBehindSource()) {
      options.restoreDetailFromParkedSource()
      return
    }

    if (
      options.restorePreviousParkedDetailIfReaderClosed({
        onDetailScrollTop: options.rememberDetailScrollTop,
      })
    ) {
      options.restoreDetailFromParkedSource()
      return
    }

    if (options.sourceReaderOpen.value) {
      options.closeVisibleSourceReaderState()
      if (options.isFeedRoute.value && !options.detailReaderOpen.value) {
        options.setTopChromeVisible(true)
      }
      options.scheduleHiddenSourceReaderCleanup(340)
      return
    }

    options.clearSourceReaderState()
    options.resetSourceSubscriptionState()
    if (options.isFeedRoute.value && !options.detailReaderOpen.value) {
      options.setTopChromeVisible(true)
    }
  }

  return {
    restoreSourceReaderBackTarget,
    closeSourceReader,
  }
}
