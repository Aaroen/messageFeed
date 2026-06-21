import { nextTick } from 'vue'

import type { ReaderSessionSnapshot, ReaderSource } from '@/composables/useReaderSession'

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
  function readerSessionSnapshot(): ReaderSessionSnapshot {
    return {
      savedAt: Date.now(),
      routeFullPath: options.getRouteFullPath(),
      feedScrollTop: options.feedScrollTop.value,
      topChromeProgress: options.topChromeProgress.value,
      feedContentCollapsed: options.feedContentCollapsed.value,
      ...options.createReaderStackSessionSnapshot(),
    }
  }

  function restoreSavedScrollPositions(snapshot: ReaderSessionSnapshot) {
    const apply = () => {
      options.scrollFeedContentTo(snapshot.feedScrollTop)
      options.scrollSourceReaderContentTo(snapshot.sourceReaderScrollTop)
      if (options.scrollDetailContentTo(snapshot.detailScrollTop)) {
        options.syncDetailContainerMetrics()
      }
    }

    nextTick(() => {
      apply()
      window.setTimeout(apply, options.scrollRestoreRetryDelay)
      window.setTimeout(apply, options.scrollRestoreSettledDelay)
    })
  }

  function applyReaderSessionSnapshot(snapshot: ReaderSessionSnapshot) {
    options.restoreFeedScrollTop(snapshot.feedScrollTop)
    options.restoreChromeSnapshot({
      progress: snapshot.topChromeProgress,
      contentCollapsed: snapshot.feedContentCollapsed,
    })
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
  }
}
