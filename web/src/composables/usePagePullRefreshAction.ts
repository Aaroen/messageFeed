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
    const afterSettledCallbacks: Array<() => void> = []
    options.clearCurrentPageNotice()
    options.beginRefreshing()
    try {
      await refreshPage({
        noticeDelayMS: options.noticeDelayMS,
        suppressStartNotice: true,
        onRefreshSettled: (callback) => {
          afterSettledCallbacks.push(callback)
        },
      })
    } finally {
      if (!refreshRunIsCurrent(token)) {
        return
      }
      options.settleRefreshCompletion({
        afterRelease: options.collapseTopChrome,
        afterSettled: () => {
          for (const callback of afterSettledCallbacks) {
            callback()
          }
        },
      })
    }
  }

  return {
    refreshCurrentPageFromPull,
    invalidateRefreshCompletion,
  }
}
