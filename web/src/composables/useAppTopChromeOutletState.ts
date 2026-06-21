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
  rootClass: ReadableRef<ClassValue>
  rootStyle: ReadableRef<StyleValue>
  isFeedRoute: ReadableRef<boolean>
  readerDetailHeader: AppReaderDetailHeaderState
  feedTabs: FeedTab[]
  activeKey: () => string | symbol | null | undefined
  feedTabsLayerHidden: ReadableRef<boolean>
  feedTabsLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetVisible: ReadableRef<boolean>
  feedTabsTargetLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetKey: ReadableRef<string | null>
  feedPullActive: ReadableRef<boolean>
  feedPullRefreshing: () => boolean
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
  phase: ReadableRef<ChromePhase>
  progress: ReadableRef<number>
  rootClass: ReadableRef<ClassValue>
  rootStyle: ReadableRef<StyleValue>
  feedHeaderActive: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailHeaderVisible: ReadableRef<boolean>
  detailHeaderLayerStyle: ReadableRef<StyleValue>
  detailTitle: ReadableRef<string>
  detailHeaderTitleStyle: ReadableRef<StyleValue>
  detailHeaderPreviousTitle: ReadableRef<string>
  detailHeaderPreviousTextStyle: ReadableRef<StyleValue>
  detailHeaderCurrentTextStyle: ReadableRef<StyleValue>
  isFeedRoute: ReadableRef<boolean>
  feedTabs: FeedTab[]
  activeKey: ReadableRef<string | symbol | null>
  feedTabsLayerHidden: ReadableRef<boolean>
  feedTabsLayerStyle: ReadableRef<StyleValue>
  viewSwipeTargetVisible: ReadableRef<boolean>
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

export function useAppTopChromeOutletState(
  options: AppTopChromeOutletStateOptions,
): AppTopChromeOutletState {
  const readerDetailHeader = options.readerDetailHeader
  const feedHeaderActive = computed(
    () => options.isFeedRoute.value || readerDetailHeader.chromeVisible.value,
  )
  const activeKey = computed(() => options.activeKey() ?? null)
  const feedPullRefreshing = computed(() => options.feedPullRefreshing())

  return {
    phase: options.phase,
    progress: options.progress,
    rootClass: options.rootClass,
    rootStyle: options.rootStyle,
    feedHeaderActive,
    detailReaderOpen: readerDetailHeader.readerOpen,
    detailHeaderVisible: readerDetailHeader.visible,
    detailHeaderLayerStyle: readerDetailHeader.layerStyle,
    detailTitle: readerDetailHeader.title,
    detailHeaderTitleStyle: readerDetailHeader.titleStyle,
    detailHeaderPreviousTitle: readerDetailHeader.previousTitle,
    detailHeaderPreviousTextStyle: readerDetailHeader.previousTextStyle,
    detailHeaderCurrentTextStyle: readerDetailHeader.currentTextStyle,
    isFeedRoute: options.isFeedRoute,
    feedTabs: options.feedTabs,
    activeKey,
    feedTabsLayerHidden: options.feedTabsLayerHidden,
    feedTabsLayerStyle: options.feedTabsLayerStyle,
    viewSwipeTargetVisible: options.viewSwipeTargetVisible,
    feedTabsTargetLayerStyle: options.feedTabsTargetLayerStyle,
    viewSwipeTargetKey: options.viewSwipeTargetKey,
    feedPullActive: options.feedPullActive,
    feedPullRefreshing,
    pullStatusStyle: options.pullStatusStyle,
    pullIconStyle: options.pullIconStyle,
    pullStatusText: options.pullStatusText,
    pullStatusMeta: options.pullStatusMeta,
    pageTitle: options.pageTitle,
    pagePullActive: options.pagePullActive,
    pageTitleLayerStyle: options.pageTitleLayerStyle,
    pagePullStatusStyle: options.pagePullStatusStyle,
    pagePullRefreshing: options.pagePullRefreshing,
    pagePullIconStyle: options.pagePullIconStyle,
    pagePullStatusText: options.pagePullStatusText,
    pagePullStatusMeta: options.pagePullStatusMeta,
  }
}
