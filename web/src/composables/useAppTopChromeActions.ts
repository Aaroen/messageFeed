type ReadableRef<T> = {
  readonly value: T
}

type AppTopChromeActionsOptions = {
  sourceReaderOpen: ReadableRef<boolean>
  sourceReaderScrollTop: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  feedScrollTop: ReadableRef<number>
  topChromeSettleDuration: number
  resolveDelay: (duration: number) => number
  setChromeVisible: (
    visible: boolean,
    options?: { settleDelayMS?: number; preserveContentCollapsed?: boolean },
  ) => void
  setChromeCollapsedHidden: (options?: { settleDelayMS?: number }) => void
  setChromeOverlayProgress: (progress: number) => void
  currentPageScrollTop: () => number
  settlePagePullOffset: (delayMS: number) => void
}

export function useAppTopChromeActions(options: AppTopChromeActionsOptions) {
  function topChromeSettleDelay() {
    return options.resolveDelay(options.topChromeSettleDuration)
  }

  function setTopChromeVisible(visible: boolean) {
    if (visible) {
      options.setChromeVisible(true, { settleDelayMS: topChromeSettleDelay() })
      return
    }
    options.setChromeCollapsedHidden({ settleDelayMS: topChromeSettleDelay() })
  }

  function showTopChromeOverlay() {
    options.setChromeVisible(true, {
      settleDelayMS: topChromeSettleDelay(),
      preserveContentCollapsed: true,
    })
  }

  function setTopChromeOverlayProgress(progress: number) {
    options.setChromeOverlayProgress(progress)
  }

  function collapseTopChrome() {
    options.setChromeCollapsedHidden({ settleDelayMS: topChromeSettleDelay() })
  }

  function currentContentScrollTop() {
    if (options.sourceReaderOpen.value) {
      return options.sourceReaderScrollTop.value
    }

    if (options.isFeedRoute.value) {
      return options.feedScrollTop.value
    }

    return options.currentPageScrollTop()
  }

  function settlePagePullOffset() {
    options.settlePagePullOffset(topChromeSettleDelay())
  }

  return {
    setTopChromeVisible,
    showTopChromeOverlay,
    setTopChromeOverlayProgress,
    collapseTopChrome,
    currentContentScrollTop,
    settlePagePullOffset,
  }
}
