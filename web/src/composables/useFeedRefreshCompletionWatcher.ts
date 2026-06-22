import { watch } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type RefreshCompletionController = {
  readonly wasActive: ReadableRef<boolean>
  begin: (payload: { viewKey: string; startedWithVisibleChrome: boolean }) => void
  finish: (delayMS: number) => { wasActive: boolean; wasSource: boolean }
  resetInactive: () => void
}

type TopPullCompletionController = {
  readonly startedWithChrome: ReadableRef<boolean>
  resetStartedWithChrome: () => void
}

type FeedRefreshCompletionWatcherOptions = {
  pullRefreshing: () => boolean
  pullViewKey: () => string
  feedOrSourcePullActive: ReadableRef<boolean>
  refreshCompletion: RefreshCompletionController
  topPull: TopPullCompletionController
  settleDelayMS: () => number
  settleSourceContentAfterRefresh: () => void
  collapseTopChrome: () => void
}

export function useFeedRefreshCompletionWatcher(options: FeedRefreshCompletionWatcherOptions) {
  function finishRefreshCompletionIfInactive(active: boolean) {
    if (active) {
      return
    }

    if (options.refreshCompletion.wasActive.value) {
      const refreshResult = options.refreshCompletion.finish(options.settleDelayMS())
      if (refreshResult.wasSource) {
        options.settleSourceContentAfterRefresh()
      }
      options.topPull.resetStartedWithChrome()
      options.collapseTopChrome()
      return
    }

    options.refreshCompletion.resetInactive()
    options.topPull.resetStartedWithChrome()
  }

  watch(
    () => options.pullRefreshing(),
    (refreshing) => {
      if (refreshing) {
        options.refreshCompletion.begin({
          viewKey: options.pullViewKey(),
          startedWithVisibleChrome: options.topPull.startedWithChrome.value,
        })
        return
      }

      finishRefreshCompletionIfInactive(options.feedOrSourcePullActive.value)
    },
  )

  watch(
    () => options.feedOrSourcePullActive.value,
    (active) => {
      finishRefreshCompletionIfInactive(active)
    },
  )
}
