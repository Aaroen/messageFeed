import { useFeedTopPullHandlers } from '@/composables/useFeedTopPullHandlers'
import { useTopChromeScrollBehavior } from '@/composables/useTopChromeScrollBehavior'

type AppFeedChromeInteractionsOptions = {
  topPull: Parameters<typeof useFeedTopPullHandlers>[0]
  scroll: Parameters<typeof useTopChromeScrollBehavior>[0]
}

export function useAppFeedChromeInteractions(options: AppFeedChromeInteractionsOptions) {
  const topPullHandlers = useFeedTopPullHandlers(options.topPull)
  const scrollBehavior = useTopChromeScrollBehavior(options.scroll)

  return {
    handleFeedTopPullStart: topPullHandlers.handleFeedTopPullStart,
    handleFeedTopPullMove: topPullHandlers.handleFeedTopPullMove,
    handleFeedTopPullEnd: topPullHandlers.handleFeedTopPullEnd,
    updateTopTabsByScroll: scrollBehavior.updateByScroll,
  }
}
