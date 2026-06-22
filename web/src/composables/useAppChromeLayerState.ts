import { computed } from 'vue'

import { useChromeLayerMotion } from '@/composables/useChromeLayerMotion'

type ReadableRef<T> = {
  readonly value: T
}

type AppChromeLayerStateOptions = {
  feedPullActive: ReadableRef<boolean>
  feedPullRefreshing: () => boolean
  pullProgress: ReadableRef<number>
  pagePullActive: ReadableRef<boolean>
  pagePullRefreshing: ReadableRef<boolean>
  pagePullProgress: ReadableRef<number>
  detailReaderOpen: ReadableRef<boolean>
  feedHeaderReturnProgress: ReadableRef<number>
  readerBackDragging: ReadableRef<boolean>
  detailBlocksGestures: () => boolean
  feedHeaderProgress: ReadableRef<number>
  viewSwipeTargetVisible: ReadableRef<boolean>
  viewSwipeTargetProgress: ReadableRef<number>
  sourcePullActive: ReadableRef<boolean>
  sourcePullRefreshing: () => boolean
  sourcePullProgress: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  feedChromeSettling: ReadableRef<boolean>
  feedRefreshSettling: ReadableRef<boolean>
  feedTopPulling: ReadableRef<boolean>
  feedCornerHidden: ReadableRef<boolean>
  detailHeaderVisible: ReadableRef<boolean>
  headerDetailLayoutActive: ReadableRef<boolean>
}

export function useAppChromeLayerState(options: AppChromeLayerStateOptions) {
  const chromeLayerMotion = useChromeLayerMotion({
    isSettling: () =>
      (options.feedChromeSettling.value || options.feedRefreshSettling.value) &&
      !options.readerBackDragging.value,
  })

  const pullStatusStyle = computed(() =>
    chromeLayerMotion.refreshStatusStyle(options.feedPullActive.value, options.pullProgress.value),
  )
  const pullIconStyle = computed(() =>
    chromeLayerMotion.refreshIconStyle(options.feedPullRefreshing(), options.pullProgress.value),
  )
  const pagePullStatusStyle = computed(() =>
    chromeLayerMotion.refreshStatusStyle(options.pagePullActive.value, options.pagePullProgress.value),
  )
  const pagePullIconStyle = computed(() =>
    chromeLayerMotion.refreshIconStyle(options.pagePullRefreshing.value, options.pagePullProgress.value),
  )
  const feedTabsLayerStyle = computed(() =>
    chromeLayerMotion.feedTabsStyle({
      detailReaderOpen: options.detailReaderOpen.value,
      returnProgress: options.feedHeaderReturnProgress.value,
      readerBackDragging: options.readerBackDragging.value,
      detailBlocksGestures: options.detailBlocksGestures(),
      feedPullActive: options.feedPullActive.value,
      headerProgress: options.feedHeaderProgress.value,
    }),
  )
  const feedTabsTargetLayerStyle = computed(() =>
    chromeLayerMotion.feedTabsTargetStyle({
      visible: options.viewSwipeTargetVisible.value,
      feedPullActive: options.feedPullActive.value,
      headerProgress: options.feedHeaderProgress.value,
      targetProgress: options.viewSwipeTargetProgress.value,
    }),
  )
  const sourcePullStatusStyle = computed(() =>
    chromeLayerMotion.refreshStatusStyle(options.sourcePullActive.value, options.sourcePullProgress.value),
  )
  const sourcePullIconStyle = computed(() =>
    chromeLayerMotion.refreshIconStyle(options.sourcePullRefreshing(), options.sourcePullProgress.value),
  )
  const sourceHeaderStyle = computed(() =>
    chromeLayerMotion.sourceHeaderStyle(
      options.topChromeProgress.value,
      options.feedHeaderHeight.value,
      options.feedChromeSettling.value && !options.readerBackDragging.value,
    ),
  )
  const detailHeaderLayerStyle = computed(() =>
    chromeLayerMotion.layerStyle(options.detailHeaderVisible.value, options.topChromeProgress.value),
  )
  const pageTitleLayerStyle = computed(() =>
    chromeLayerMotion.layerStyle(!options.pagePullActive.value, options.feedHeaderProgress.value),
  )
  const sourceMainLayerStyle = computed(() =>
    chromeLayerMotion.layerStyle(!options.sourcePullActive.value, options.topChromeProgress.value),
  )
  const headerClass = computed(() => ({
    'app-header--feed-inactive':
      options.feedHeaderProgress.value <= 0.01 &&
      !options.feedChromeSettling.value &&
      !options.feedTopPulling.value,
    'app-header--detail': options.headerDetailLayoutActive.value,
  }))
  const headerStyle = computed(() =>
    chromeLayerMotion.headerStyle(
      options.feedHeaderProgress.value,
      options.feedHeaderHeight.value,
      options.feedPullActive.value || options.pagePullActive.value,
      options.headerDetailLayoutActive.value,
    ),
  )
  const navOpenButtonStyle = computed(() =>
    chromeLayerMotion.navOpenButtonStyle(
      options.feedHeaderProgress.value,
      options.feedHeaderHeight.value,
      !options.feedCornerHidden.value,
    ),
  )

  return {
    pullStatusStyle,
    pullIconStyle,
    pagePullStatusStyle,
    pagePullIconStyle,
    feedTabsLayerStyle,
    feedTabsTargetLayerStyle,
    sourcePullStatusStyle,
    sourcePullIconStyle,
    sourceHeaderStyle,
    detailHeaderLayerStyle,
    pageTitleLayerStyle,
    sourceMainLayerStyle,
    headerClass,
    headerStyle,
    navOpenButtonStyle,
  }
}
