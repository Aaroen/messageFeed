import type { PageRefreshOptions } from '@/composables/usePageOutletState'

type ReadableRef<T> = {
  readonly value: T
}

type PageRefreshAction = (options?: PageRefreshOptions) => Promise<void> | void

type PagePullRefreshActionOptions = {
  refreshing: ReadableRef<boolean>
  noticeDelayMS: number
  currentRefreshPage: () => PageRefreshAction | null
  clearCurrentPageNotice: () => void
  beginRefreshing: () => void
  holdCompletionRefreshing: () => void
  releaseCompletionRefreshing: () => void
  settleRefreshCompletion: (options: {
    releaseDelayMS?: number
    settleDelayMS?: number
    afterRelease?: () => void
    afterSettled?: () => void
  }) => void
  collapseTopChrome: () => void
}

export function usePagePullRefreshAction(options: PagePullRefreshActionOptions) {
  let refreshRunToken = 0

  function nextRefreshRunToken() {
    refreshRunToken += 1
    return refreshRunToken
  }

  function invalidateRefreshCompletion() {
    refreshRunToken += 1
    options.releaseCompletionRefreshing()
  }

  function refreshRunIsCurrent(token: number) {
    return token === refreshRunToken
  }

  async function refreshCurrentPageFromPull() {
    const refreshPage = options.currentRefreshPage()
    if (!refreshPage || options.refreshing.value) {
      return
    }

    const token = nextRefreshRunToken()
    const afterReleaseCallbacks: Array<() => void> = []
    const afterSettledCallbacks: Array<() => void> = []
    let releaseCallbacksReleased = false
    let callbacksReleased = false
    function releaseAfterReleaseCallbacks() {
      if (releaseCallbacksReleased) {
        return
      }
      releaseCallbacksReleased = true
      for (const callback of afterReleaseCallbacks) {
        callback()
      }
    }
    function releaseAfterSettledCallbacks() {
      if (callbacksReleased) {
        return
      }
      callbacksReleased = true
      for (const callback of afterSettledCallbacks) {
        callback()
      }
    }

    options.clearCurrentPageNotice()
    options.beginRefreshing()
    options.holdCompletionRefreshing()
    try {
      await refreshPage({
        noticeDelayMS: options.noticeDelayMS,
        suppressStartNotice: true,
        onRefreshReleased: (callback) => {
          afterReleaseCallbacks.push(callback)
        },
        onRefreshSettled: (callback) => {
          afterSettledCallbacks.push(callback)
        },
      })
    } finally {
      if (!refreshRunIsCurrent(token)) {
        options.releaseCompletionRefreshing()
        releaseAfterReleaseCallbacks()
        releaseAfterSettledCallbacks()
        return
      }
      options.settleRefreshCompletion({
        afterRelease: () => {
          options.collapseTopChrome()
          releaseAfterReleaseCallbacks()
        },
        afterSettled: () => {
          options.releaseCompletionRefreshing()
          releaseAfterSettledCallbacks()
        },
      })
    }
  }

  return {
    refreshCurrentPageFromPull,
    invalidateRefreshCompletion,
  }
}
