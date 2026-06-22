type ReadableRef<T> = {
  readonly value: T
}

type FeedPullStatePayload = {
  offset: number
  active: boolean
  refreshing: boolean
  statusText: string
  statusMeta: string
}

type FeedInteractionWriter = {
  pullViewKey: string
  setPullState: (
    payload: FeedPullStatePayload & {
      viewKey?: string
      lastUpdatedAt: string
    },
  ) => void
  resetPullState: () => void
}

type PullRefreshCompletionController = {
  finishRefreshing: () => void
  finishBackgroundRefresh: () => void
  clearTimers: () => void
  settleRefreshCompletion: (options: {
    releaseDelayMS?: number
    settleDelayMS?: number
    afterRelease?: () => void
    afterSettled?: () => void
  }) => void
}

type FeedPullRefreshCompletionActionOptions = {
  usesGlobalPullState: ReadableRef<boolean>
  active: ReadableRef<boolean>
  viewKey: ReadableRef<string>
  lastUpdatedAt: ReadableRef<string>
  pullOffset: ReadableRef<number>
  pullActive: ReadableRef<boolean>
  pullSettling: ReadableRef<boolean>
  refreshing: ReadableRef<boolean>
  pullStartedWithVisibleChrome: ReadableRef<boolean>
  trackingPull: ReadableRef<boolean>
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  isSourceMode: ReadableRef<boolean>
  feedInteraction: FeedInteractionWriter
  pullRefresh: PullRefreshCompletionController
}

export function useFeedPullRefreshCompletionAction(options: FeedPullRefreshCompletionActionOptions) {
  function ownsPullState(ownerViewKey = options.viewKey.value) {
    return options.feedInteraction.pullViewKey === ownerViewKey
  }

  function pullStateIsUnowned() {
    return !options.feedInteraction.pullViewKey
  }

  function canAccessPullState(force = false, ownerViewKey = options.viewKey.value) {
    if (!options.usesGlobalPullState.value && !force) {
      return false
    }
    if (!pullStateIsUnowned() && !ownsPullState(ownerViewKey)) {
      return false
    }
    return options.active.value || ownsPullState(ownerViewKey) || force
  }

  function setPullState(payload: FeedPullStatePayload, force = false) {
    if (!canAccessPullState(force)) {
      return
    }
    options.feedInteraction.setPullState({
      ...payload,
      viewKey: options.viewKey.value,
      lastUpdatedAt: options.lastUpdatedAt.value,
    })
  }

  function clearPullState(force = false, ownerViewKey = options.viewKey.value) {
    if (!canAccessPullState(force, ownerViewKey)) {
      return
    }
    options.feedInteraction.resetPullState()
  }

  function completeLoad(payload: {
    isRefresh: boolean
    isBackgroundRefresh: boolean
    afterSettled?: () => void
  }) {
    if (!payload.isRefresh) {
      options.pullRefresh.finishRefreshing()
      return
    }

    if (payload.isBackgroundRefresh) {
      options.pullRefresh.finishBackgroundRefresh()
      return
    }

    setPullState({
      offset: options.pullOffset.value,
      active: true,
      refreshing: true,
      statusText: options.isSourceMode.value ? '抓取中' : '正在刷新',
      statusMeta: options.pullStatusMeta.value,
    })
    options.pullRefresh.settleRefreshCompletion({
      afterRelease: clearPullState,
      afterSettled: payload.afterSettled,
    })
  }

  function syncPullState() {
    if (!options.usesGlobalPullState.value || !options.active.value || options.pullSettling.value) {
      return
    }

    if (!options.pullActive.value && !options.refreshing.value) {
      if (options.pullStartedWithVisibleChrome.value && options.trackingPull.value) {
        return
      }
      clearPullState()
      return
    }

    options.pullRefresh.clearTimers()
    setPullState({
      offset: options.pullOffset.value,
      active: options.pullActive.value,
      refreshing: options.refreshing.value,
      statusText: options.pullStatusText.value,
      statusMeta: options.pullStatusMeta.value,
    })
  }

  return {
    clearPullState,
    completeLoad,
    syncPullState,
  }
}
