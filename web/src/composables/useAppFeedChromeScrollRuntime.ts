import { useAppFeedChromeInteractions } from '@/composables/useAppFeedChromeInteractions'
import { useAppScrollHandlers } from '@/composables/useAppScrollHandlers'

type AppFeedChromeScrollRuntimeOptions = {
  feedChrome: Parameters<typeof useAppFeedChromeInteractions>[0]
  scroll: Omit<Parameters<typeof useAppScrollHandlers>[0], 'updateTopTabsByScroll'>
}

export function useAppFeedChromeScrollRuntime(options: AppFeedChromeScrollRuntimeOptions) {
  const feedChromeInteractions = useAppFeedChromeInteractions(options.feedChrome)
  const scrollHandlers = useAppScrollHandlers({
    ...options.scroll,
    updateTopTabsByScroll: feedChromeInteractions.updateTopTabsByScroll,
  })

  return {
    handleFeedTopPullStart: feedChromeInteractions.handleFeedTopPullStart,
    handleFeedTopPullMove: feedChromeInteractions.handleFeedTopPullMove,
    handleFeedTopPullEnd: feedChromeInteractions.handleFeedTopPullEnd,
    handleFeedContentScroll: scrollHandlers.handleFeedContentScroll,
    handlePageContentScroll: scrollHandlers.handlePageContentScroll,
    handleSourceReaderScroll: scrollHandlers.handleSourceReaderScroll,
    handleDetailContentScroll: scrollHandlers.handleDetailContentScroll,
  }
}
