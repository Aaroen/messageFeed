import { nextTick, watch } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type AppRouteSessionWatchersOptions = {
  route: RouteLocationNormalizedLoaded
  isFeedRoute: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
  sourceReaderVisible: ReadableRef<boolean>
  readerSource: ReadableRef<ReaderSource | null>
  detailItem: ReadableRef<FeedItem | null>
  detailSourceKind: ReadableRef<FeedSourceKind>
  detailOpenedFromSourceReader: ReadableRef<boolean>
  detailListReturnCommitted: ReadableRef<boolean>
  sourceReaderReturnMode: ReadableRef<'detail' | null>
  sourceReaderBackDetailItemId: ReadableRef<number | null>
  parkedDetailStackDepth: ReadableRef<number>
  detailSourceExitProgress: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  feedScrollTop: ReadableRef<number>
  sourceReaderScrollTop: ReadableRef<number>
  detailScrollTop: ReadableRef<number>
  resetGestureTracking: () => void
  resetBackSwipeOffset: () => void
  resetPageTopPullTracking: () => void
  finishFeedTopPull: () => void
  resetRefreshCompletion: () => void
  resetPagePullMotion: () => void
  resetFeedViewDragOffset: () => void
  setTopChromeVisible: (visible: boolean) => void
  currentFeedScrollTop: () => number
  updateFeedScrollTop: (scrollTop: number) => void
  currentPageScrollTop: () => number
  rememberFeedScrollTop: (scrollTop: number) => void
  rememberPageScrollTop: (scrollTop: number) => void
  scheduleReaderSessionSave: () => void
  scheduleReaderURLAndHistorySync: () => void
}

export function useAppRouteSessionWatchers(options: AppRouteSessionWatchersOptions) {
  let routeChangeToken = 0

  watch(
    () => options.route.name,
    () => {
      routeChangeToken += 1
      const token = routeChangeToken
      options.resetGestureTracking()
      options.resetBackSwipeOffset()
      options.resetPageTopPullTracking()
      options.finishFeedTopPull()
      options.resetRefreshCompletion()
      options.resetPagePullMotion()
      options.resetFeedViewDragOffset()
      if (options.isFeedRoute.value) {
        options.setTopChromeVisible(true)
        nextTick(() => {
          if (token !== routeChangeToken || !options.isFeedRoute.value) {
            return
          }
          const current = options.currentFeedScrollTop()
          options.updateFeedScrollTop(current)
          options.rememberFeedScrollTop(current)
        })
      } else {
        options.setTopChromeVisible(true)
        nextTick(() => {
          if (token !== routeChangeToken || options.isFeedRoute.value) {
            return
          }
          options.rememberPageScrollTop(options.currentPageScrollTop())
        })
      }
      options.scheduleReaderSessionSave()
      options.scheduleReaderURLAndHistorySync()
    },
  )

  watch(
    () => [
      options.route.fullPath,
      options.navigationVisible.value,
      options.sourceReaderVisible.value,
      options.readerSource.value?.id ?? 0,
      options.readerSource.value?.kind ?? '',
      options.detailItem.value?.id ?? 0,
      options.detailSourceKind.value,
      options.detailOpenedFromSourceReader.value,
      options.detailListReturnCommitted.value,
      options.sourceReaderReturnMode.value,
      options.sourceReaderBackDetailItemId.value,
      options.parkedDetailStackDepth.value,
    ],
    () => {
      options.scheduleReaderSessionSave()
      options.scheduleReaderURLAndHistorySync()
    },
  )

  watch(
    () => [
      options.detailSourceExitProgress.value,
      options.topChromeProgress.value,
      options.feedContentCollapsed.value,
      options.feedScrollTop.value,
      options.sourceReaderScrollTop.value,
      options.detailScrollTop.value,
    ],
    () => {
      options.scheduleReaderSessionSave()
    },
  )
}
