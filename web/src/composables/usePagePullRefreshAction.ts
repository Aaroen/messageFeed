import type { PageRefreshOptions } from '@/composables/usePageOutletState'

type ReadableRef<T> = {
  readonly value: T
}

type PageRefreshAction = (options?: PageRefreshOptions) => Promise<void> | void

type PagePullRefreshActionOptions = {
  refreshing: ReadableRef<boolean>
  noticeDelayMS: number
  releaseDelayMS: number
  settleDelayMS: number
  currentRefreshPage: () => PageRefreshAction | null
  beginRefreshing: () => void
  settleRefreshCompletion: (options: {
    releaseDelayMS?: number
    settleDelayMS: number
    afterRelease?: () => void
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
        releaseDelayMS: options.releaseDelayMS,
        settleDelayMS: options.settleDelayMS,
        afterRelease: options.collapseTopChrome,
      })
    }
  }

  return {
    refreshCurrentPageFromPull,
  }
}
