import { computed } from 'vue'

import { useAppChromeVisualState } from '@/composables/useAppChromeVisualState'
import { useAppFeedRefreshCompletionRuntime } from '@/composables/useAppFeedRefreshCompletionRuntime'
import { useAppFeedChromeState } from '@/composables/useAppFeedChromeState'
import { useAppTopChromeActions } from '@/composables/useAppTopChromeActions'

type ReadableRef<T> = {
  readonly value: T
}

type AppChromeVisualLayerOptions = Parameters<typeof useAppChromeVisualState>[0]['layer']
type AppFeedChromeStateOptions = Parameters<typeof useAppFeedChromeState>[0]
type AppFeedRefreshCompletionOptions = Parameters<typeof useAppFeedRefreshCompletionRuntime>[0]
type AppTopChromeActionOptions = Parameters<typeof useAppTopChromeActions>[0]
type AppChromeFeedPullActivityOptions = Omit<
  AppFeedChromeStateOptions['pullActivity'],
  'getFeedPullActive' | 'getFeedPullRefreshing' | 'getFeedPullOffset' | 'getFeedPullViewKey'
>
type AppChromeFeedPullOptions = {
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  getPullActive: () => boolean
  getPullRefreshing: () => boolean
  getPullOffset: () => number
  getPullViewKey: () => string
}
type AppChromeFeedOptions = Omit<AppFeedChromeStateOptions, 'pullActivity' | 'layout' | 'shellMotion'> & {
  pullActivity: AppChromeFeedPullActivityOptions
  layout: Omit<AppFeedChromeStateOptions['layout'], 'refreshStartedWithChrome'>
  shellMotion: Omit<AppFeedChromeStateOptions['shellMotion'], 'feedRefreshSettling'>
}

type AppChromeRuntimeOptions = {
  feed: AppChromeFeedOptions
  feedPull: AppChromeFeedPullOptions
  feedRefreshCompletion: AppFeedRefreshCompletionOptions
  visual: Omit<
    AppChromeVisualLayerOptions,
    | 'feedPullRefreshing'
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
    | 'feedRefreshSettling'
  > & {
    sourceReaderVisible: ReadableRef<boolean>
    sourceReaderUnderDetail: ReadableRef<boolean>
  }
  mainClass: Parameters<typeof useAppChromeVisualState>[0]['mainClass']
  topChromeActions: AppTopChromeActionOptions
}

export function useAppChromeRuntime(options: AppChromeRuntimeOptions) {
  const feedRefreshCompletion = useAppFeedRefreshCompletionRuntime(options.feedRefreshCompletion)
  const feedChrome = useAppFeedChromeState({
    ...options.feed,
    pullActivity: {
      ...options.feed.pullActivity,
      getFeedPullActive: options.feedPull.getPullActive,
      getFeedPullRefreshing: options.feedPull.getPullRefreshing,
      getFeedPullOffset: options.feedPull.getPullOffset,
      getFeedPullViewKey: options.feedPull.getPullViewKey,
    },
    layout: {
      ...options.feed.layout,
      refreshStartedWithChrome: feedRefreshCompletion.refreshStartedWithChrome,
    },
    shellMotion: {
      ...options.feed.shellMotion,
      feedRefreshSettling: feedRefreshCompletion.feedRefreshSettling,
    },
  })
  const topChromeActions = useAppTopChromeActions(options.topChromeActions)
  const sourceReaderInteractive = computed(
    () => options.visual.sourceReaderVisible.value && !options.visual.sourceReaderUnderDetail.value,
  )
  const foregroundSourcePullActive = computed(
    () => feedChrome.sourcePullActive.value && sourceReaderInteractive.value,
  )
  const feedPullRefreshing = computed(() => options.feedPull.getPullRefreshing())
  const sourcePullRefreshing = computed(
    () => foregroundSourcePullActive.value && feedPullRefreshing.value,
  )
  const chromeVisual = useAppChromeVisualState({
    layer: {
      feedPullActive: feedChrome.feedPullActive,
      feedPullRefreshing: () => feedPullRefreshing.value,
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
      feedRefreshSettling: feedRefreshCompletion.feedRefreshSettling,
      feedTopPulling: options.visual.feedTopPulling,
      feedCornerHidden: feedChrome.feedCornerHidden,
      detailHeaderVisible: feedChrome.detailHeaderVisible,
      headerDetailLayoutActive: feedChrome.headerDetailLayoutActive,
    },
    mainClass: options.mainClass,
  })

  return {
    feedPullActive: feedChrome.feedPullActive,
    feedPullRefreshing,
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
    pullStatusText: options.feedPull.pullStatusText,
    pullStatusMeta: options.feedPull.pullStatusMeta,
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
    feedRefreshCompletion,
    topChromeActions,
  }
}
