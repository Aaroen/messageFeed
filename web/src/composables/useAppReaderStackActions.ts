type RestoreParkedDetailOptions = {
  onDetailScrollTop?: (scrollTop: number) => void
}

type AppReaderStackActionsOptions = {
  quickDuration: number
  restoreMorphingItemContentWithDelay: (unlockDelay?: number) => void
  scheduleHiddenSourceReaderCleanupWithDelay: (delay?: number) => void
  restorePreviousParkedDetail: (options?: RestoreParkedDetailOptions) => boolean
  rememberDetailScrollTop: (scrollTop: number) => void
}

export function useAppReaderStackActions(options: AppReaderStackActionsOptions) {
  function restoreMorphingItemContent(unlockDelay = options.quickDuration) {
    options.restoreMorphingItemContentWithDelay(unlockDelay)
  }

  function scheduleHiddenSourceReaderCleanup(delay = options.quickDuration) {
    options.scheduleHiddenSourceReaderCleanupWithDelay(delay)
  }

  function restorePreviousParkedDetail() {
    return options.restorePreviousParkedDetail({
      onDetailScrollTop: options.rememberDetailScrollTop,
    })
  }

  return {
    restoreMorphingItemContent,
    scheduleHiddenSourceReaderCleanup,
    restorePreviousParkedDetail,
  }
}
