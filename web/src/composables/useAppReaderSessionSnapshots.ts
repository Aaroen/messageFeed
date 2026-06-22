import { nextTick } from 'vue'

import type { ReaderSessionSnapshot, ReaderSource } from '@/composables/useReaderSession'
import { normalizeReaderRouteFullPath } from '@/composables/useReaderRouteSync'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderStackSessionSnapshotPart = Omit<
  ReaderSessionSnapshot,
  'savedAt' | 'routeFullPath' | 'feedScrollTop' | 'topChromeProgress' | 'feedContentCollapsed'
>

type ChromeSnapshot = {
  progress: number
  contentCollapsed: boolean
}

type ApplyReaderStackSessionOptions = {
  onSourceScrollTop?: (scrollTop: number) => void
  onDetailScrollTop?: (scrollTop: number) => void
  onReaderSourceRestored?: (source: ReaderSource) => void
}

type AppReaderSessionSnapshotsOptions = {
  feedScrollTop: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  scrollRestoreRetryDelay: number
  scrollRestoreSettledDelay: number
  createReaderStackSessionSnapshot: () => ReaderStackSessionSnapshotPart
  restoreFeedScrollTop: (scrollTop: number) => void
  restoreChromeSnapshot: (snapshot: ChromeSnapshot) => void
  applyReaderStackSessionSnapshot: (
    snapshot: ReaderSessionSnapshot,
    options?: ApplyReaderStackSessionOptions,
  ) => void
  rememberSourceScrollTop: (scrollTop: number) => void
  rememberDetailScrollTop: (scrollTop: number) => void
  loadSourceReaderSubscription: (source: ReaderSource) => unknown
  scrollFeedContentTo: (scrollTop: number) => void
  scrollSourceReaderContentTo: (scrollTop: number) => void
  scrollDetailContentTo: (scrollTop: number) => boolean
  syncDetailContainerMetrics: () => void
  getRouteFullPath: () => string
}

export function useAppReaderSessionSnapshots(options: AppReaderSessionSnapshotsOptions) {
  let scrollRestoreToken = 0
  let scrollRestoreRetryTimer = 0
  let scrollRestoreSettledTimer = 0

  function stableChromeSnapshot(snapshot: ChromeSnapshot): ChromeSnapshot {
    if (snapshot.contentCollapsed) {
      return {
        progress: 0,
        contentCollapsed: true,
      }
    }

    const progress = Number.isFinite(snapshot.progress) ? snapshot.progress : 1
    const stableProgress = progress >= 0.5 ? 1 : 0
    return {
      progress: stableProgress,
      contentCollapsed: stableProgress <= 0.01 || snapshot.contentCollapsed,
    }
  }

  function clearScrollRestoreHandles() {
    if (typeof window !== 'undefined' && scrollRestoreRetryTimer !== 0) {
      window.clearTimeout(scrollRestoreRetryTimer)
    }
    if (typeof window !== 'undefined' && scrollRestoreSettledTimer !== 0) {
      window.clearTimeout(scrollRestoreSettledTimer)
    }
    scrollRestoreRetryTimer = 0
    scrollRestoreSettledTimer = 0
  }

  function clearScrollRestoreTimers() {
    scrollRestoreToken += 1
    clearScrollRestoreHandles()
  }

  function nextScrollRestoreToken() {
    scrollRestoreToken += 1
    clearScrollRestoreHandles()
    return scrollRestoreToken
  }

  function scrollRestoreIsCurrent(token: number, snapshotRouteFullPath: string) {
    return (
      token === scrollRestoreToken &&
      normalizeReaderRouteFullPath(options.getRouteFullPath()) ===
        normalizeReaderRouteFullPath(snapshotRouteFullPath)
    )
  }

  function readerSessionSnapshot(): ReaderSessionSnapshot {
    const chromeSnapshot = stableChromeSnapshot({
      progress: options.topChromeProgress.value,
      contentCollapsed: options.feedContentCollapsed.value,
    })
    return {
      savedAt: Date.now(),
      routeFullPath: options.getRouteFullPath(),
      feedScrollTop: options.feedScrollTop.value,
      topChromeProgress: chromeSnapshot.progress,
      feedContentCollapsed: chromeSnapshot.contentCollapsed,
      ...options.createReaderStackSessionSnapshot(),
    }
  }

  function restoreSavedScrollPositions(snapshot: ReaderSessionSnapshot) {
    const token = nextScrollRestoreToken()
    const apply = () => {
      if (!scrollRestoreIsCurrent(token, snapshot.routeFullPath)) {
        return
      }
      options.scrollFeedContentTo(snapshot.feedScrollTop)
      options.scrollSourceReaderContentTo(snapshot.sourceReaderScrollTop)
      if (options.scrollDetailContentTo(snapshot.detailScrollTop)) {
        options.syncDetailContainerMetrics()
      }
    }

    nextTick(() => {
      apply()
      if (!scrollRestoreIsCurrent(token, snapshot.routeFullPath)) {
        return
      }
      scrollRestoreRetryTimer = window.setTimeout(() => {
        scrollRestoreRetryTimer = 0
        apply()
      }, options.scrollRestoreRetryDelay)
      scrollRestoreSettledTimer = window.setTimeout(() => {
        scrollRestoreSettledTimer = 0
        apply()
      }, options.scrollRestoreSettledDelay)
    })
  }

  function applyReaderSessionSnapshot(snapshot: ReaderSessionSnapshot) {
    const chromeSnapshot = stableChromeSnapshot({
      progress: snapshot.topChromeProgress,
      contentCollapsed: snapshot.feedContentCollapsed,
    })
    options.restoreFeedScrollTop(snapshot.feedScrollTop)
    options.restoreChromeSnapshot(chromeSnapshot)
    options.applyReaderStackSessionSnapshot(snapshot, {
      onSourceScrollTop: options.rememberSourceScrollTop,
      onDetailScrollTop: options.rememberDetailScrollTop,
      onReaderSourceRestored: (source) => {
        void options.loadSourceReaderSubscription(source)
      },
    })
    restoreSavedScrollPositions(snapshot)
  }

  return {
    readerSessionSnapshot,
    applyReaderSessionSnapshot,
    clearScrollRestoreTimers,
  }
}
