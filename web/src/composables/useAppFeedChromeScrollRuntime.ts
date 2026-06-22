import { useAppFeedChromeInteractions } from '@/composables/useAppFeedChromeInteractions'
import { useAppScrollHandlers } from '@/composables/useAppScrollHandlers'
import type { useFeedRefreshCompletionWatcher } from '@/composables/useFeedRefreshCompletionWatcher'

type FeedChromeInteractionOptions = Parameters<typeof useAppFeedChromeInteractions>[0]
type FeedPullRuntimeOptions = {
  getPullRefreshing: () => boolean
  getPullViewKey: () => string
}
type FeedChromeTopPullOptions = Omit<
  FeedChromeInteractionOptions['topPull'],
  'feedPullRefreshing'
>
type RefreshCompletionWatcherOptions = Omit<
  Parameters<typeof useFeedRefreshCompletionWatcher>[0],
  'refreshCompletion' | 'canApplyCompletionEffects'
>
type FeedChromeRefreshCompletionOptions = Omit<
  RefreshCompletionWatcherOptions,
  'topPull' | 'collapseTopChrome' | 'pullRefreshing' | 'pullViewKey'
> & {
  installWatcher: (options: RefreshCompletionWatcherOptions) => void
}

type AppFeedChromeScrollRuntimeOptions = {
  feedChrome: Omit<FeedChromeInteractionOptions, 'topPull'> & {
    topPull: FeedChromeTopPullOptions
    feedPull: FeedPullRuntimeOptions
    refreshCompletion?: FeedChromeRefreshCompletionOptions
  }
  scroll: Omit<Parameters<typeof useAppScrollHandlers>[0], 'updateTopTabsByScroll'>
}

export function useAppFeedChromeScrollRuntime(options: AppFeedChromeScrollRuntimeOptions) {
  const feedChromeInteractions = useAppFeedChromeInteractions({
    ...options.feedChrome,
    topPull: {
      ...options.feedChrome.topPull,
      feedPullRefreshing: options.feedChrome.feedPull.getPullRefreshing,
    },
  })
  const refreshCompletion = options.feedChrome.refreshCompletion
  if (refreshCompletion) {
    refreshCompletion.installWatcher({
      pullRefreshing: options.feedChrome.feedPull.getPullRefreshing,
      pullViewKey: options.feedChrome.feedPull.getPullViewKey,
      feedOrSourcePullActive: refreshCompletion.feedOrSourcePullActive,
      topPull: options.feedChrome.topPull.topPull,
      settleDelayMS: refreshCompletion.settleDelayMS,
      settleSourceContentAfterRefresh: refreshCompletion.settleSourceContentAfterRefresh,
      collapseTopChrome: options.feedChrome.topPull.collapseTopChrome,
    })
  }
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
