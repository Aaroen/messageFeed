type ReadableRef<T> = {
  readonly value: T
}

type CloseItemReaderResult = {
  shouldScheduleHiddenSourceCleanup: boolean
}

type BeginCollapseItemReaderResult = {
  shouldRefreshFeedOrigin: boolean
  shouldRestorePreviousParkedDetail: boolean
}

type CollapseItemReaderTransitionOptions = {
  afterBegin?: (result: BeginCollapseItemReaderResult) => void
  afterFinish?: (result: BeginCollapseItemReaderResult) => void
}

type ReaderItemCloseActionOptions = {
  detailReaderOpen: ReadableRef<boolean>
  isFeedRoute: ReadableRef<boolean>
  readerDuration: number
  resolveDelay: (duration: number) => number
  detailCommittedListReturn: () => boolean
  hasDetailParkedBehindSource: () => boolean
  clearDetailEntryTimer: () => void
  closeItemReaderWithTransition: () => CloseItemReaderResult
  collapseItemReaderWithDelay: (delay: number, options?: CollapseItemReaderTransitionOptions) => void
  setTopChromeVisible: (visible: boolean) => void
  scheduleHiddenSourceReaderCleanup: () => void
  suppressFollowingClick: () => void
  refreshDetailFeedOriginRect: (lock?: boolean) => void
  restorePreviousParkedDetail: () => boolean
  scheduleReaderURLAndHistorySync: (forcePush?: boolean) => void
}

export function useReaderItemCloseAction(options: ReaderItemCloseActionOptions) {
  function closeItemReader() {
    const result = options.closeItemReaderWithTransition()
    if (options.isFeedRoute.value) {
      options.setTopChromeVisible(true)
    }
    if (result.shouldScheduleHiddenSourceCleanup) {
      options.scheduleHiddenSourceReaderCleanup()
    }
  }

  function finishCommittedListReturnForGesture() {
    if (!options.detailCommittedListReturn()) {
      return
    }
    if (options.hasDetailParkedBehindSource()) {
      return
    }

    options.clearDetailEntryTimer()
    closeItemReader()
  }

  function collapseItemReader(duration = options.readerDuration) {
    if (!options.detailReaderOpen.value) {
      return
    }

    options.suppressFollowingClick()
    options.collapseItemReaderWithDelay(options.resolveDelay(duration), {
      afterBegin: (result) => {
        if (result.shouldRefreshFeedOrigin) {
          options.refreshDetailFeedOriginRect(true)
        }
      },
      afterFinish: (result) => {
        if (result.shouldRestorePreviousParkedDetail && options.restorePreviousParkedDetail()) {
          options.scheduleReaderURLAndHistorySync(true)
          return
        }
        closeItemReader()
        options.scheduleReaderURLAndHistorySync(true)
      },
    })
  }

  return {
    finishCommittedListReturnForGesture,
    closeItemReader,
    collapseItemReader,
  }
}
