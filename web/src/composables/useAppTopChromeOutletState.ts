import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { AppReaderDetailHeaderState } from '@/composables/useAppReaderDetailHeaderState'
import type { ChromePhase } from '@/composables/useChromeState'

type ReadableRef<T> = {
  readonly value: T
}

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

type FeedTab = {
  key: string
  label: string
  path: string
}

type AppTopChromeOutletStateOptions = {
  phase: ReadableRef<ChromePhase>
  progress: ReadableRef<number>
  interactive: ReadableRef<boolean>
  rootClass: ReadableRef<ClassValue>
  rootStyle: ReadableRef<StyleValue>
  isFeedRoute: ReadableRef<boolean>
  readerDetailHeader: AppReaderDetailHeaderState
  feedTabs: FeedTab[]
  activeKey: () => string | symbol | null | undefined
  feedTabsLayerStyle: ReadableRef<StyleValue>
  feedTabsTargetLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetKey: ReadableRef<string | null>
  feedPullActive: ReadableRef<boolean>
  feedPullRefreshing: ReadableRef<boolean>
  pullStatusStyle: ReadableRef<StyleValue>
  pullIconStyle: ReadableRef<StyleValue>
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  pageTitle: ReadableRef<string>
  pagePullActive: ReadableRef<boolean>
  pageTitleLayerStyle: ReadableRef<StyleValue>
  pagePullStatusStyle: ReadableRef<StyleValue>
  pagePullRefreshing: ReadableRef<boolean>
  pagePullIconStyle: ReadableRef<StyleValue>
  pagePullStatusText: ReadableRef<string>
  pagePullStatusMeta: ReadableRef<string>
}

export type AppTopChromeOutletState = {
  chrome: {
    phase: ReadableRef<ChromePhase>
    progress: ReadableRef<number>
    interactive: ReadableRef<boolean>
    rootClass: ReadableRef<ClassValue>
    rootStyle: ReadableRef<StyleValue>
  }
  feed: {
    active: ReadableRef<boolean>
    detailReaderOpen: ReadableRef<boolean>
    detailHeaderLayerStyle: ReadableRef<StyleValue>
    detailTitle: ReadableRef<string>
    detailHeaderTitleStyle: ReadableRef<StyleValue>
    detailHeaderPreviousTitle: ReadableRef<string>
    detailHeaderPreviousTextStyle: ReadableRef<StyleValue>
    detailHeaderCurrentTextStyle: ReadableRef<StyleValue>
    isFeedRoute: ReadableRef<boolean>
    feedTabs: FeedTab[]
    activeKey: ReadableRef<string | symbol | null>
    feedTabsLayerStyle: ReadableRef<StyleValue>
    feedTabsTargetLayerStyle: ReadableRef<StyleValue>
    viewSwipeTargetKey: ReadableRef<string | null>
    feedPullActive: ReadableRef<boolean>
    pullRefreshing: ReadableRef<boolean>
    pullStatusStyle: ReadableRef<StyleValue>
    pullIconStyle: ReadableRef<StyleValue>
    pullStatusText: ReadableRef<string>
    pullStatusMeta: ReadableRef<string>
  }
  page: {
    title: ReadableRef<string>
    pullActive: ReadableRef<boolean>
    titleLayerStyle: ReadableRef<StyleValue>
    pullStatusStyle: ReadableRef<StyleValue>
    pullRefreshing: ReadableRef<boolean>
    pullIconStyle: ReadableRef<StyleValue>
    pullStatusText: ReadableRef<string>
    pullStatusMeta: ReadableRef<string>
  }
}

export function useAppTopChromeOutletState(
  options: AppTopChromeOutletStateOptions,
): AppTopChromeOutletState {
  const readerDetailHeader = options.readerDetailHeader
  const feedHeaderActive = computed(
    () => options.isFeedRoute.value || readerDetailHeader.chromeVisible.value,
  )
  const activeKey = computed(() => options.activeKey() ?? null)

  return {
    chrome: {
      phase: options.phase,
      progress: options.progress,
      interactive: options.interactive,
      rootClass: options.rootClass,
      rootStyle: options.rootStyle,
    },
    feed: {
      active: feedHeaderActive,
      detailReaderOpen: readerDetailHeader.readerOpen,
      detailHeaderLayerStyle: readerDetailHeader.layerStyle,
      detailTitle: readerDetailHeader.title,
      detailHeaderTitleStyle: readerDetailHeader.titleStyle,
      detailHeaderPreviousTitle: readerDetailHeader.previousTitle,
      detailHeaderPreviousTextStyle: readerDetailHeader.previousTextStyle,
      detailHeaderCurrentTextStyle: readerDetailHeader.currentTextStyle,
      isFeedRoute: options.isFeedRoute,
      feedTabs: options.feedTabs,
      activeKey,
      feedTabsLayerStyle: options.feedTabsLayerStyle,
      feedTabsTargetLayerStyle: options.feedTabsTargetLayerStyle,
      viewSwipeTargetKey: options.viewSwipeTargetKey,
      feedPullActive: options.feedPullActive,
      pullRefreshing: options.feedPullRefreshing,
      pullStatusStyle: options.pullStatusStyle,
      pullIconStyle: options.pullIconStyle,
      pullStatusText: options.pullStatusText,
      pullStatusMeta: options.pullStatusMeta,
    },
    page: {
      title: options.pageTitle,
      pullActive: options.pagePullActive,
      titleLayerStyle: options.pageTitleLayerStyle,
      pullStatusStyle: options.pagePullStatusStyle,
      pullRefreshing: options.pagePullRefreshing,
      pullIconStyle: options.pagePullIconStyle,
      pullStatusText: options.pagePullStatusText,
      pullStatusMeta: options.pagePullStatusMeta,
    },
  }
}
