import type { PageRefreshOptions } from '@/composables/usePageOutletState'

type ReadableRef<T> = {
  readonly value: T
}

type PageRefreshAction = (options?: PageRefreshOptions) => Promise<void> | void

type PagePullRefreshActionOptions = {
  refreshing: ReadableRef<boolean>
  noticeDelayMS: number
  currentRefreshPage: () => PageRefreshAction | null
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
  async function refreshCurrentPageFromPull() {
    const refreshPage = options.currentRefreshPage()
    if (!refreshPage || options.refreshing.value) {
      return
    }

    options.beginRefreshing()
    try {
      await refreshPage({ noticeDelayMS: options.noticeDelayMS, suppressStartNotice: true })
    } finally {
      options.settleRefreshCompletion({
        afterRelease: options.collapseTopChrome,
      })
    }
  }

  return {
    refreshCurrentPageFromPull,
  }
}
