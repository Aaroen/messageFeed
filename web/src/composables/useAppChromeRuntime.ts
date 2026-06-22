import { computed } from 'vue'

import { useAppChromeVisualState } from '@/composables/useAppChromeVisualState'
import { useAppFeedChromeState } from '@/composables/useAppFeedChromeState'

type ReadableRef<T> = {
  readonly value: T
}

type AppChromeVisualLayerOptions = Parameters<typeof useAppChromeVisualState>[0]['layer']

type AppChromeRuntimeOptions = {
  feed: Parameters<typeof useAppFeedChromeState>[0]
  visual: Omit<
    AppChromeVisualLayerOptions,
    | 'feedPullActive'
    | 'pagePullActive'
    | 'pullProgress'
    | 'sourcePullActive'
    | 'sourcePullRefreshing'
    | 'sourcePullProgress'
    | 'feedHeaderProgress'
    | 'feedHeaderReturnProgress'
    | 'feedHeaderHeight'
    | 'feedCornerHidden'
    | 'detailHeaderVisible'
    | 'headerDetailLayoutActive'
  > & {
    sourceReaderVisible: ReadableRef<boolean>
    sourceReaderUnderDetail: ReadableRef<boolean>
  }
  mainClass: Parameters<typeof useAppChromeVisualState>[0]['mainClass']
}

export function useAppChromeRuntime(options: AppChromeRuntimeOptions) {
  const feedChrome = useAppFeedChromeState(options.feed)
  const sourceReaderInteractive = computed(
    () => options.visual.sourceReaderVisible.value && !options.visual.sourceReaderUnderDetail.value,
  )
  const foregroundSourcePullActive = computed(
    () => feedChrome.sourcePullActive.value && sourceReaderInteractive.value,
  )
  const sourcePullRefreshing = computed(
    () => foregroundSourcePullActive.value && options.visual.feedPullRefreshing(),
  )
  const chromeVisual = useAppChromeVisualState({
    layer: {
      feedPullActive: feedChrome.feedPullActive,
      feedPullRefreshing: options.visual.feedPullRefreshing,
      pullProgress: feedChrome.pullProgress,
      pagePullActive: feedChrome.pagePullActive,
      pagePullRefreshing: options.visual.pagePullRefreshing,
      pagePullProgress: options.visual.pagePullProgress,
      detailReaderOpen: options.visual.detailReaderOpen,
      feedHeaderReturnProgress: feedChrome.feedHeaderReturnProgress,
      readerBackDragging: options.visual.readerBackDragging,
      detailBlocksGestures: options.visual.detailBlocksGestures,
      feedHeaderProgress: feedChrome.feedHeaderProgress,
      viewSwipeActive: options.visual.viewSwipeActive,
      viewSwipeTargetVisible: options.visual.viewSwipeTargetVisible,
      viewSwipeTargetProgress: options.visual.viewSwipeTargetProgress,
      sourcePullActive: foregroundSourcePullActive,
      sourcePullRefreshing: () => sourcePullRefreshing.value,
      sourcePullProgress: feedChrome.sourcePullProgress,
      topChromeProgress: options.visual.topChromeProgress,
      feedHeaderHeight: feedChrome.feedHeaderHeight,
      feedChromeSettling: options.visual.feedChromeSettling,
      feedRefreshSettling: options.visual.feedRefreshSettling,
      feedTopPulling: options.visual.feedTopPulling,
      feedCornerHidden: feedChrome.feedCornerHidden,
      detailHeaderVisible: feedChrome.detailHeaderVisible,
      headerDetailLayoutActive: feedChrome.headerDetailLayoutActive,
    },
    mainClass: options.mainClass,
  })

  return {
    feedPullActive: feedChrome.feedPullActive,
    pagePullActive: feedChrome.pagePullActive,
    foregroundSourcePullActive,
    sourcePullRefreshing,
    feedOrSourcePullActive: feedChrome.feedOrSourcePullActive,
    pullProgress: feedChrome.pullProgress,
    sourcePullProgress: feedChrome.sourcePullProgress,
    feedHeaderHeight: feedChrome.feedHeaderHeight,
    feedHeaderProgress: feedChrome.feedHeaderProgress,
    freezeFeedBodyDuringTopRefresh: feedChrome.freezeFeedBodyDuringTopRefresh,
    feedHeaderReturnProgress: feedChrome.feedHeaderReturnProgress,
    mainStyle: feedChrome.mainStyle,
    feedContentStyle: feedChrome.feedContentStyle,
    pageContentStyle: feedChrome.pageContentStyle,
    detailHeaderVisible: feedChrome.detailHeaderVisible,
    pullStatusStyle: chromeVisual.pullStatusStyle,
    pullIconStyle: chromeVisual.pullIconStyle,
    pagePullStatusStyle: chromeVisual.pagePullStatusStyle,
    pagePullIconStyle: chromeVisual.pagePullIconStyle,
    feedTabsLayerStyle: chromeVisual.feedTabsLayerStyle,
    feedTabsTargetLayerStyle: chromeVisual.feedTabsTargetLayerStyle,
    sourcePullStatusStyle: chromeVisual.sourcePullStatusStyle,
    sourcePullIconStyle: chromeVisual.sourcePullIconStyle,
    sourceHeaderStyle: chromeVisual.sourceHeaderStyle,
    detailHeaderLayerStyle: chromeVisual.detailHeaderLayerStyle,
    pageTitleLayerStyle: chromeVisual.pageTitleLayerStyle,
    sourceMainLayerStyle: chromeVisual.sourceMainLayerStyle,
    headerClass: chromeVisual.headerClass,
    headerStyle: chromeVisual.headerStyle,
    navOpenButtonStyle: chromeVisual.navOpenButtonStyle,
    mainClass: chromeVisual.mainClass,
  }
}
