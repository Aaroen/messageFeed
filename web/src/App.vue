<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import { useFeedInteractionStore } from '@/stores/feedInteraction'
import { getFeedItem } from '@/api/feed'
import AppMainOutlet from '@/components/AppMainOutlet.vue'
import AppNavigationLayer from '@/components/AppNavigationLayer.vue'
import AppReaderStackOutlet from '@/components/AppReaderStackOutlet.vue'
import { useChromeState } from '@/composables/useChromeState'
import {
  type FeedSourceKind,
  type ReaderSessionSnapshot,
  type ReaderSource,
} from '@/composables/useReaderSession'
import { useNavigationDrawer } from '@/composables/useNavigationDrawer'
import { useRefreshCompletionState } from '@/composables/useRefreshCompletionState'
import { useTopPullState } from '@/composables/useTopPullState'
import { useViewportSize } from '@/composables/useViewportSize'
import { useThemeState } from '@/composables/useThemeState'
import { useFeedScrollState } from '@/composables/useFeedScrollState'
import { usePageOutletState } from '@/composables/usePageOutletState'
import { useFeedContentState } from '@/composables/useFeedContentState'
import { useScrollHistory } from '@/composables/useScrollHistory'
import { useDoubleBackGuard } from '@/composables/useDoubleBackGuard'
import { useGestureOriginState } from '@/composables/useGestureOriginState'
import { useNavigationGestureState } from '@/composables/useNavigationGestureState'
import { useRouteRuntimeState } from '@/composables/useRouteRuntimeState'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { useAppRoutePageRuntime } from '@/composables/useAppRoutePageRuntime'
import { useAppScrollHandlers } from '@/composables/useAppScrollHandlers'
import { useAppNavigationRuntime } from '@/composables/useAppNavigationRuntime'
import { useAppTopChromeActions } from '@/composables/useAppTopChromeActions'
import { useAppChromeRuntime } from '@/composables/useAppChromeRuntime'
import { useAppElementRefs } from '@/composables/useAppElementRefs'
import { useAppReaderStackActions } from '@/composables/useAppReaderStackActions'
import { useAppInteractionTargetGuards } from '@/composables/useAppInteractionTargetGuards'
import { useAppShellRuntime } from '@/composables/useAppShellRuntime'
import { useAppRuntimeEffects } from '@/composables/useAppRuntimeEffects'
import { useAppReaderSessionRuntime } from '@/composables/useAppReaderSessionRuntime'
import { useAppReaderStackOutletBindings } from '@/composables/useAppReaderStackOutletBindings'
import { useAppMainOutletBindings } from '@/composables/useAppMainOutletBindings'
import { useAppPagePullInteractions } from '@/composables/useAppPagePullInteractions'
import { useAppReaderBackSwipeInteractions } from '@/composables/useAppReaderBackSwipeInteractions'
import { useAppReaderSourceCloseInteractions } from '@/composables/useAppReaderSourceCloseInteractions'
import { useAppReaderOpenInteractions } from '@/composables/useAppReaderOpenInteractions'
import { useAppReaderCloseInteractions } from '@/composables/useAppReaderCloseInteractions'
import { useAppReaderTransitionRects } from '@/composables/useAppReaderTransitionRects'
import { useAppReaderDetailInteractions } from '@/composables/useAppReaderDetailInteractions'
import { useAppReaderRouteSyncBinding } from '@/composables/useAppReaderRouteSyncBinding'
import { useAppFeedChromeInteractions } from '@/composables/useAppFeedChromeInteractions'
import { useAppPointerGestureInteractions } from '@/composables/useAppPointerGestureInteractions'
import { useAppFeedViewSwipeInteractions } from '@/composables/useAppFeedViewSwipeInteractions'
import { useAppSwipeGestureRuntime } from '@/composables/useAppSwipeGestureRuntime'
import { useReaderRouteQueryRestore } from '@/composables/useReaderRouteQueryRestore'
import { useAppReaderStackRuntime } from '@/composables/useAppReaderStackRuntime'
import { useAppReaderPresentationRuntime } from '@/composables/useAppReaderPresentationRuntime'
import { useAppTopChromeOutletState } from '@/composables/useAppTopChromeOutletState'
import { useAppVirtualBackGuard } from '@/composables/useAppVirtualBackGuard'

type SwipeSurface =
  | 'feed:subscriptions'
  | 'feed:recommendations'
  | 'reader:detail'
  | 'reader:source'
  | 'page:management'

const route = useRoute()
const router = useRouter()
const feedInteraction = useFeedInteractionStore()
const feedContent = useFeedContentState()
const pageOutlet = usePageOutletState()
const scrollHistory = useScrollHistory()
const gestureOrigin = useGestureOriginState()
const navigationGesture = useNavigationGestureState()
const routeRuntime = useRouteRuntimeState()
const readerStackRuntime = useAppReaderStackRuntime()
const {
  sourceReaderContentRef,
  detailContentRef,
  detailFrameRef,
  detailInlineSourceRef,
  sourceReaderScrollTop,
  readerBackDragging,
  readerSource,
  sourceReaderVisible,
  detailItem,
  detailLoading,
  detailError,
  detailSourceKind,
  morphingItemId,
  morphingHeightLockItemId,
  morphingItemHeight,
  detailOpenedFromSourceReader,
  detailHeaderPreviousTitle,
  detailSourceExitProgress,
  detailReturningToFeed,
  detailListReturnCommitted,
  sourceReaderReturnMode,
  detailScrollTop,
  parkedDetailStackDepth,
  sourceReaderBackDetailItemId,
  sourceNotice,
  sourceTimelinePreloadEnabled,
  detailTransitionRectsLocked,
  detailFeedOriginLocked,
  sourceReaderMounted,
  sourceReaderOpen,
  detailReaderOpen,
  sourceReaderUnderDetail,
  detailSurfaceProgress,
  detailScrollMax,
  detailReadingProgress,
  detailProgressVisible,
  feedItemPreviewProgress,
  detailFeedHeaderReturnProgress,
  detailParkedBehindSource,
  detailChromeVisible,
  detailCommittedListReturn,
  hasDetailParkedBehindSource,
  hasParkedDetailSourceState,
  sourceReaderShouldReturnToDetail,
  sourceReaderCanRestoreReturnOnCancel,
  createReaderStackSessionSnapshot,
  applyReaderStackSessionSnapshot,
  pushParkedDetailSnapshot,
  restorePreviousParkedDetail: restoreReaderStackPreviousParkedDetail,
  restorePreviousParkedDetailIfReaderClosed,
  restoreSourceReaderBackTargetState,
  prepareSourceReaderReturnDragState,
  clearHiddenSourceCleanupTimer,
  scheduleHiddenSourceReaderCleanupWithDelay,
  openSourceReaderState,
  closeVisibleSourceReaderState,
  clearSourceReaderState,
  clearDetailEntryTimer,
  openItemReaderWithTransition,
  finishOpenItemReaderLoad,
  applyDetailFeedOriginRectState,
  applyDetailSourceTransitionRectsState,
  applyVisibleSourceReturnTargetState,
  clearMorphingHeightUnlockTimer,
  restoreMorphingItemContentWithDelay,
  revealSourceReaderUnderDetailState,
  clearReaderStackTimers,
  updateSourceReaderScrollTopState,
  updateDetailScrollMetricsState,
  updateDetailScrollTopState,
  updateDetailFrameContentHeightState,
  setDetailProgressDraggingState,
  closeItemReaderWithTransition,
  collapseItemReaderWithDelay,
  restoreItemReaderExpansionWithDelay,
  restoreDetailFromSourceSwipeWithDelay,
  completeDetailToSourceReaderWithDelay,
  restoreParkedSourceReaderWithDelay,
  restoreDetailFromParkedSourceWithDelay,
  readerBackSwipeCandidateActive,
  readerBackSwipeTrackingActive,
  resetReaderBackSwipeDragState,
  resetReaderBackSwipeCandidateState,
  beginReaderBackSwipeCandidateState,
  updateReaderBackSwipeDragState,
  readerBackSwipeFinishResult,
  readerBackSwipeCancelResult,
  applyReaderBackSwipeAction,
  readerBackSwipeTransitionBeginPayload,
  readerBackSwipeTransitionUpdatePayload,
  beginReaderBackSwipeDragState,
  detailBlocksGestures,
  setSourceReaderContentElement: setSourceReaderContentElementState,
  setDetailContentElement: setDetailContentElementState,
  setDetailInlineSourceElement: setDetailInlineSourceElementState,
  setDetailFrameElement: setDetailFrameElementState,
  scrollSourceReaderContentTo: scrollSourceReaderContentElementTo,
  scrollDetailContentTo: scrollDetailContentElementTo,
  setSourceTimelinePreloadEnabledState,
  clearReaderStretchAnchorsIfIdle,
  setSourceCatalogEntryState,
  setSourceSubscriptionState,
  setSourceSubscriptionLoadingState,
  setSourceNoticeState,
  sourceToggleLabel,
  sourceToggleActive,
  sourceToggleDisabled,
  clearSourceSubscriptionRuntime,
  resetSourceSubscriptionState,
  loadSourceReaderSubscription,
  toggleSourceReaderSubscription,
} = readerStackRuntime
const feedScroll = useFeedScrollState()
const feedScrollTop = feedScroll.scrollTop
const chromeState = useChromeState()
const topChromeProgress = chromeState.progress
const topChromePhase = chromeState.phase
const feedContentCollapsed = chromeState.contentCollapsed
const feedChromeSettling = chromeState.settling
const viewportSize = useViewportSize({ defaultWidth: 1440, defaultHeight: 900 })
const windowWidth = viewportSize.width
const windowHeight = viewportSize.height
const motionTimings = useMotionTimings()
const clickSuppressionDuration = motionTimings.clickSuppressionDuration
const motionQuickDuration = motionTimings.quickDuration
const motionNormalDuration = motionTimings.normalDuration
const motionStretchAnchorClearDuration = motionTimings.stretchAnchorClearDuration
const motionHeaderSwapDuration = motionTimings.headerSwapDuration
const motionReaderDuration = motionTimings.readerDuration
const topChromeSettleDuration = motionTimings.topChromeSettleDuration
const navigationDrawerSettleDuration = motionTimings.navigationDrawerSettleDuration
const sourceReaderCloseCleanupDelay = motionTimings.sourceReaderCloseCleanupDelay
const topRefreshNoticeDelay = motionTimings.topRefreshNoticeDelay
const detailFrameMetricsInitialDelay = motionTimings.detailFrameMetricsInitialDelay
const detailFrameMetricsSettledDelay = motionTimings.detailFrameMetricsSettledDelay
const readerScrollRestoreRetryDelay = motionTimings.readerScrollRestoreRetryDelay
const readerScrollRestoreSettledDelay = motionTimings.readerScrollRestoreSettledDelay
const readerMorphDuration = motionTimings.readerMorphDuration
const readerRectRetryDelay = motionTimings.readerRectRetryDelay
const motionDelay = motionTimings.delay
const navigationDrawer = useNavigationDrawer({
  windowWidth,
  resolveDelay: motionDelay,
  settleDuration: navigationDrawerSettleDuration,
})
const navigationOpen = navigationDrawer.open
const navigationProgress = navigationDrawer.progress
const navigationWidth = navigationDrawer.width
const navigationVisible = navigationDrawer.visible
const navigationPanelStyle = navigationDrawer.panelStyle
const navigationScrimStyle = navigationDrawer.scrimStyle
const themeState = useThemeState()
const darkTheme = themeState.dark
const toggleTheme = themeState.toggle
const refreshCompletion = useRefreshCompletionState()
const refreshStartedWithChrome = refreshCompletion.startedWithChrome
const feedRefreshSettling = refreshCompletion.settling
const feedTopPull = useTopPullState()
const feedTopPulling = feedTopPull.pulling
const feedTopPullStartedWithChrome = feedTopPull.startedWithChrome
const homeBackGuard = useDoubleBackGuard(motionTimings.homeExitDoubleBackTimeout)
const {
  bindReaderRouteSync,
  scheduleReaderURLAndHistorySync,
  rememberSourceScrollTop,
  rememberDetailScrollTop,
  clearReaderSessionScrollRestoreTimers,
  readerSession,
  saveReaderSessionNow,
  scheduleReaderSessionSave,
  restoreReaderSession,
} = useAppReaderSessionRuntime({
  feedScrollTop,
  topChromeProgress,
  feedContentCollapsed,
  scrollRestoreRetryDelay: readerScrollRestoreRetryDelay,
  scrollRestoreSettledDelay: readerScrollRestoreSettledDelay,
  createReaderStackSessionSnapshot,
  restoreFeedScrollTop: (scrollTop) => {
    feedScroll.restore(scrollTop)
  },
  restoreChromeSnapshot: (snapshot) => {
    chromeState.restoreSnapshot(snapshot)
  },
  applyReaderStackSessionSnapshot,
  loadSourceReaderSubscription,
  scrollFeedContentTo: (scrollTop) => {
    feedContent.scrollTo(scrollTop)
  },
  scrollSourceReaderContentTo: scrollSourceReaderContentElementTo,
  scrollDetailContentTo: scrollDetailContentElementTo,
  syncDetailContainerMetrics: () => {
    syncDetailContainerMetrics()
  },
  getRouteFullPath: () => route.fullPath,
  rememberScrollTop: (surface, scrollTop) => {
    scrollHistory.set(surface, scrollTop)
  },
  canSaveReaderSession: routeRuntime.canSaveReaderSession,
})

const {
  selectedKeys,
  pageTitle,
  isFeedRoute,
  cornerButtonLabel,
  pagePullState,
  pagePullOffset,
  pagePullRefreshing,
  pagePullProgress,
  pageContentInnerStyle,
  pagePullStatusText,
  pagePullStatusMeta,
} = useAppRoutePageRuntime(route)
const virtualBackGuard = useAppVirtualBackGuard({
  route,
  router,
  navigationVisible,
  sourceReaderOpen,
  detailReaderOpen,
  detailOpenedFromSourceReader,
  isFeedRoute,
  homeBackGuard,
  hasParkedDetailSourceState,
  detailCommittedListReturn,
  sourceReaderShouldReturnToDetail,
  closeNavigation: () => closeNavigation(),
  collapseItemReader: () => collapseItemReader(),
  restoreSourceReaderBackTarget: () => restoreSourceReaderBackTarget(),
  closeSourceReader: () => closeSourceReader(),
  goHome: (options) => goHome(options),
  getRouteFullPath: () => route.fullPath,
  canHandleNavigation: routeRuntime.canHandleNavigation,
  onBackConsumed: () => {
    scheduleReaderSessionSave()
    if (
      !isFeedRoute.value &&
      !navigationVisible.value &&
      !sourceReaderOpen.value &&
      !detailReaderOpen.value &&
      !hasParkedDetailSourceState()
    ) {
      return
    }
    scheduleReaderURLAndHistorySync(true)
  },
})
const readerRouteSync = useAppReaderRouteSyncBinding({
  bindReaderRouteSync,
  route,
  router,
  canSync: () => routeRuntime.canSyncReaderRoute(readerSession.restoring.value),
  getReaderSource: () => readerSource.value,
  isSourceReaderOpen: () => sourceReaderOpen.value,
  getDetailItemID: () => detailItem.value?.id,
  getDetailSourceKind: () => detailSourceKind.value,
  setProgrammaticRouteNavigation: routeRuntime.setProgrammaticNavigation,
  syncVirtualHistoryState: virtualBackGuard.syncHistoryState,
})
const {
  swipePhase,
  swipeDirection,
  swipeProgress,
  swipeIsBlocked,
  beginSwipeTransition,
  updateSwipeTransition,
  settleSwipeTransition,
  scheduleSwipeReset,
  resetSwipeTransition,
  clearSwipeTransitionTimer,
  feedPagerTransition,
  viewDragOffset,
  viewSwipeCandidateActive,
  viewSwipeActive,
  activeFeedIndex,
  activeFeedSurface,
  feedTrackStyle,
  viewSwipeTargetKey,
  viewSwipeTargetVisible,
  viewSwipeTargetProgress,
  resetFeedViewSwipeTracking,
  clearFeedViewStartedWithHiddenChrome,
  resetFeedViewDragOffset,
  clearFeedPagerTimers,
  isHorizontalSwipe,
  isViewHorizontalSwipe,
  isNavigationDrag,
  isBackHorizontalSwipe,
  shouldCancelTopPull,
  shouldWaitForTopPull,
  canStartViewSwipe,
  canStartNavigationOpen,
} = useAppSwipeGestureRuntime<SwipeSurface>({
  getActiveKey: () => route.name,
  windowWidth,
  isFeedRoute,
  detailReaderOpen,
  navigationVisible,
  sourceReaderOpen,
  isSubscriptionsRoute: () => route.name === 'subscriptions',
  detailBlocksGestures,
  feedPullBusy: () =>
    feedInteraction.pullActive ||
    feedInteraction.pullRefreshing ||
    feedInteraction.pullOffset > 1 ||
    feedTopPulling.value,
})
const chromeRuntime = useAppChromeRuntime({
  feed: {
    pullActivity: {
      isFeedRoute,
      pagePullRefreshing,
      pagePullOffset,
      sourceReaderOpen,
      getFeedPullActive: () => feedInteraction.pullActive,
      getFeedPullRefreshing: () => feedInteraction.pullRefreshing,
      getFeedPullOffset: () => feedInteraction.pullOffset,
      getFeedPullViewKey: () => feedInteraction.pullViewKey,
    },
    layout: {
      windowWidth,
      isFeedRoute,
      topChromeProgress,
      feedTopPullStartedWithChrome,
      refreshStartedWithChrome,
      feedTopPulling,
      feedContentCollapsed,
      detailFeedHeaderReturnProgress,
    },
    shellMotion: {
      detailSurfaceProgress,
      feedRefreshSettling,
      feedChromeSettling,
      feedTopPulling,
      readerBackDragging,
      detailReaderOpen,
      detailReturningToFeed,
    },
    visibility: {
      isFeedRoute,
      topChromeProgress,
      detailReaderOpen,
      sourceReaderOpen,
      detailChromeVisible,
    },
  },
  visual: {
    feedPullRefreshing: () => feedInteraction.pullRefreshing,
    pagePullRefreshing,
    pagePullProgress,
    detailReaderOpen,
    readerBackDragging,
    detailBlocksGestures,
    viewSwipeActive,
    viewSwipeTargetVisible,
    viewSwipeTargetProgress,
    sourceReaderVisible,
    sourceReaderUnderDetail,
    topChromeProgress,
    feedChromeSettling,
    feedRefreshSettling,
    feedTopPulling,
  },
  mainClass: {
    isFeedRoute,
  },
})
const feedPullActive = chromeRuntime.feedPullActive
const pagePullActive = chromeRuntime.pagePullActive
const foregroundSourcePullActive = chromeRuntime.foregroundSourcePullActive
const sourcePullRefreshing = chromeRuntime.sourcePullRefreshing
const feedOrSourcePullActive = chromeRuntime.feedOrSourcePullActive
const pullProgress = chromeRuntime.pullProgress
const sourcePullProgress = chromeRuntime.sourcePullProgress
const feedHeaderHeight = chromeRuntime.feedHeaderHeight
const feedHeaderProgress = chromeRuntime.feedHeaderProgress
const freezeFeedBodyDuringTopRefresh = chromeRuntime.freezeFeedBodyDuringTopRefresh
const feedHeaderReturnProgress = chromeRuntime.feedHeaderReturnProgress
const mainStyle = chromeRuntime.mainStyle
const feedContentStyle = chromeRuntime.feedContentStyle
const pageContentStyle = chromeRuntime.pageContentStyle
const detailHeaderVisible = chromeRuntime.detailHeaderVisible
const { statusText: pullStatusText, statusMeta: pullStatusMeta } = storeToRefs(feedInteraction)
const pullStatusStyle = chromeRuntime.pullStatusStyle
const pullIconStyle = chromeRuntime.pullIconStyle
const pagePullStatusStyle = chromeRuntime.pagePullStatusStyle
const pagePullIconStyle = chromeRuntime.pagePullIconStyle
const feedTabsLayerStyle = chromeRuntime.feedTabsLayerStyle
const feedTabsTargetLayerStyle = chromeRuntime.feedTabsTargetLayerStyle
const sourcePullStatusStyle = chromeRuntime.sourcePullStatusStyle
const sourcePullIconStyle = chromeRuntime.sourcePullIconStyle
const sourceHeaderStyle = chromeRuntime.sourceHeaderStyle
const detailHeaderLayerStyle = chromeRuntime.detailHeaderLayerStyle
const pageTitleLayerStyle = chromeRuntime.pageTitleLayerStyle
const sourceMainLayerStyle = chromeRuntime.sourceMainLayerStyle
const headerClass = chromeRuntime.headerClass
const headerStyle = chromeRuntime.headerStyle
const navOpenButtonStyle = chromeRuntime.navOpenButtonStyle
const mainClass = chromeRuntime.mainClass
const readerPresentationRuntime = useAppReaderPresentationRuntime({
  readerStack: readerStackRuntime,
  viewport: {
    windowWidth,
    windowHeight,
  },
  chrome: {
    feedHeaderHeight,
    topChromeProgress,
    feedContentCollapsed,
    feedChromeSettling,
    detailHeaderVisible,
    detailHeaderLayerStyle,
  },
  theme: {
    darkTheme,
  },
  source: {
    foregroundPullActive: foregroundSourcePullActive,
  },
  timings: {
    resolveDelay: motionDelay,
    detailFrameMetricsInitialDelay,
    detailFrameMetricsSettledDelay,
  },
})
const readerMotionState = readerPresentationRuntime.readerMotion
const detailFrameId = readerMotionState.detailFrameId
const readerMorphVisibilityState = readerPresentationRuntime.readerMorph
const readerDetailHeaderState = readerPresentationRuntime.readerDetailHeader

const {
  managementItems,
  feedTabs,
  resetGestureTracking,
  pushRoute,
  replaceRoute,
  settleNavigation,
  openNavigation,
  closeNavigation,
  handleMenuClick,
  goHome,
  handleCornerButtonClick,
  navigateTo,
} = useAppNavigationRuntime({
  router,
  routeRuntime,
  navigationDrawer,
  feedPagerTransition,
  resetNavigationGesture: navigationGesture.reset,
  resetFeedViewSwipeTracking,
  clearFeedViewStartedWithHiddenChrome,
  resetReaderBackSwipeCandidate: resetReaderBackSwipeCandidateState,
  setChromeStableVisible: chromeState.setStableVisible,
  motionDelay,
  motionNormalDuration,
})

const navigationOpenDistance = 72
const viewSwitchDistance = 62
const appTopChromeActions = useAppTopChromeActions({
  sourceReaderOpen,
  sourceReaderScrollTop,
  isFeedRoute,
  feedScrollTop,
  topChromeSettleDuration,
  resolveDelay: motionDelay,
  setChromeVisible: chromeState.setVisible,
  setChromeCollapsedHidden: chromeState.setCollapsedHidden,
  setChromeOverlayProgress: chromeState.setOverlayProgress,
  currentPageScrollTop: pageOutlet.currentScrollTop,
  settlePagePullOffset: pagePullState.settleOffset,
})
const setTopChromeVisible = appTopChromeActions.setTopChromeVisible
const hideTopChromeForScroll = appTopChromeActions.hideTopChromeForScroll
const showTopChromeOverlay = appTopChromeActions.showTopChromeOverlay
const setTopChromeOverlayProgress = appTopChromeActions.setTopChromeOverlayProgress
const collapseTopChrome = appTopChromeActions.collapseTopChrome
const currentContentScrollTop = appTopChromeActions.currentContentScrollTop
const settlePagePullOffset = appTopChromeActions.settlePagePullOffset

const appInteractionTargetGuards = useAppInteractionTargetGuards()
const isPageTopPullControlTarget = appInteractionTargetGuards.isPageTopPullControlTarget

const {
  handleClickCapture,
  suppressFollowingClick,
  handleKeydown,
  handleResize,
  clearClickSuppressionTimer,
} = useAppShellRuntime({
  clickSuppressionDuration,
  closeNavigation,
  syncViewportSize: () => viewportSize.sync(),
})

const {
  setSourceReaderContentElement,
  setFeedContentElement,
  setPageContentElement,
  setPageViewInstance,
  detailFrameViewportOffset,
  setDetailContentElement,
  setDetailInlineSourceElement,
  setDetailFrameElement,
} = useAppElementRefs({
  detailFrameRef,
  setSourceReaderContentElement: setSourceReaderContentElementState,
  setFeedContentElement: feedContent.setContentElement,
  setPageContentElement: pageOutlet.setContentElement,
  setPageViewInstance: pageOutlet.setViewInstance,
  setDetailContentElement: setDetailContentElementState,
  setDetailInlineSourceElement: setDetailInlineSourceElementState,
  setDetailFrameElement: setDetailFrameElementState,
})

const readerTransitionRects = useAppReaderTransitionRects({
  sourceReaderContentRef,
  detailInlineSourceRef,
  detailItem,
  detailFeedOriginLocked,
  detailTransitionRectsLocked,
  retryDelay: readerRectRetryDelay,
  activeFeedIndex,
  findFeedItemElement: feedContent.findItemElement,
  applyDetailFeedOriginRectState,
  applyDetailSourceTransitionRectsState,
  applyVisibleSourceReturnTargetState,
})
const refreshDetailFeedOriginRect = readerTransitionRects.refreshDetailFeedOriginRect
const captureDetailSourceTransitionRects = readerTransitionRects.captureDetailSourceTransitionRects
const captureVisibleSourceReturnTarget = readerTransitionRects.captureVisibleSourceReturnTarget
const clearDetailSourceTransitionRectCapture =
  readerTransitionRects.clearDetailSourceTransitionRectCapture

const {
  restoreMorphingItemContent,
  scheduleHiddenSourceReaderCleanup,
  restorePreviousParkedDetail,
} = useAppReaderStackActions({
  quickDuration: motionQuickDuration,
  restoreMorphingItemContentWithDelay,
  scheduleHiddenSourceReaderCleanupWithDelay,
  restorePreviousParkedDetail: restoreReaderStackPreviousParkedDetail,
  rememberDetailScrollTop,
})

const readerOpenInteractions = useAppReaderOpenInteractions({
  sourceOpen: {
    openSourceReaderState,
    getReaderSource: () => readerSource.value,
    clearHiddenSourceCleanupTimer,
    setTopChromeVisible,
    captureDetailSourceTransitionRects,
    loadSourceReaderSubscription,
    resetSourceSubscriptionState,
    rememberSourceScrollTop,
    scrollSourceReaderContentElementTo,
  },
  sourceReveal: {
    detailItem,
    detailSourceKind,
    readerSource,
    setTopChromeVisible,
    revealSourceReaderUnderDetailState,
    captureDetailSourceTransitionRects,
  },
  itemOpen: {
    detailItem,
    detailSourceKind,
    sourceReaderOpen,
    readerSource,
    headerSwapDuration: motionHeaderSwapDuration,
    detailEntryDuration: readerMorphDuration,
    resolveDelay: motionDelay,
    openItemReaderWithTransition,
    loadFeedItem: getFeedItem,
    finishOpenItemReaderLoad,
    setChromeStableVisible: chromeState.setStableVisible,
    finishFeedTopPull: feedTopPull.finish,
    rememberDetailScrollTop,
    captureDetailSourceTransitionRects,
    scrollDetailContentElementTo,
    scheduleReaderSessionSave,
  },
})
const openSourceReader = readerOpenInteractions.openSourceReader
const showSourceReaderUnderDetail = readerOpenInteractions.showSourceReaderUnderDetail
const openItemReader = readerOpenInteractions.openItemReader

const readerRouteQueryRestore = useReaderRouteQueryRestore({
  route,
  openSourceReader,
  openItemReader,
  restoreReaderSession,
})
const restoreReaderStateOnLoad = readerRouteQueryRestore.restoreReaderStateOnLoad

const readerSourceCloseInteractions = useAppReaderSourceCloseInteractions({
  parkedRestore: {
    detailReaderOpen,
    readerDuration: motionReaderDuration,
    resolveDelay: motionDelay,
    suppressFollowingClick,
    restoreDetailFromParkedSourceWithDelay,
    clearMorphingHeightUnlockTimer,
    captureVisibleSourceReturnTarget,
    setTopChromeVisible,
    restoreMorphingItemContent,
    scheduleHiddenSourceReaderCleanup,
  },
  sourceClose: {
    sourceReaderOpen,
    detailReaderOpen,
    isFeedRoute,
    sourceReaderCloseCleanupDelay,
    sourceReaderShouldReturnToDetail,
    hasDetailParkedBehindSource,
    restorePreviousParkedDetailIfReaderClosed,
    restoreSourceReaderBackTargetState,
    closeVisibleSourceReaderState,
    clearSourceReaderState,
    resetSourceSubscriptionState,
    rememberDetailScrollTop,
    setTopChromeVisible,
    scheduleHiddenSourceReaderCleanup,
  },
})
const restoreDetailFromParkedSource = readerSourceCloseInteractions.restoreDetailFromParkedSource
const restoreSourceReaderBackTarget = readerSourceCloseInteractions.restoreSourceReaderBackTarget
const closeSourceReader = readerSourceCloseInteractions.closeSourceReader

const readerCloseInteractions = useAppReaderCloseInteractions({
  backSwipeReset: {
    readerBackDragging,
    stretchAnchorClearDuration: motionStretchAnchorClearDuration,
    resetReaderBackSwipeDragState,
    resetPageSideMotion: pagePullState.resetSideMotion,
    clearReaderStretchAnchorsIfIdle,
    clearPageStretchAnchorIfIdle: pagePullState.clearStretchAnchorIfIdle,
  },
  itemClose: {
    detailReaderOpen,
    isFeedRoute,
    readerDuration: motionReaderDuration,
    resolveDelay: motionDelay,
    detailCommittedListReturn,
    hasDetailParkedBehindSource,
    clearDetailEntryTimer,
    closeItemReaderWithTransition,
    collapseItemReaderWithDelay,
    setTopChromeVisible,
    scheduleHiddenSourceReaderCleanup,
    suppressFollowingClick,
    refreshDetailFeedOriginRect,
    restorePreviousParkedDetail,
    scheduleReaderURLAndHistorySync,
  },
  restoreActions: {
    normalDuration: motionNormalDuration,
    readerDuration: motionReaderDuration,
    resolveDelay: motionDelay,
    restoreParkedSourceReaderWithDelay,
    restoreItemReaderExpansionWithDelay,
    restoreDetailFromSourceSwipeWithDelay,
    completeDetailToSourceReaderWithDelay,
    setTopChromeVisible,
    captureDetailSourceTransitionRects,
    restoreMorphingItemContent,
  },
})
const finishCommittedListReturnForGesture = readerCloseInteractions.finishCommittedListReturnForGesture
const closeItemReader = readerCloseInteractions.closeItemReader
const collapseItemReader = readerCloseInteractions.collapseItemReader
const resetBackSwipeOffset = readerCloseInteractions.resetBackSwipeOffset
const clearBackSwipeStretchAnchorTimer = readerCloseInteractions.clearBackSwipeStretchAnchorTimer
const restoreParkedSourceReader = readerCloseInteractions.restoreParkedSourceReader
const restoreItemReaderExpansion = readerCloseInteractions.restoreItemReaderExpansion
const restoreDetailFromSourceSwipe = readerCloseInteractions.restoreDetailFromSourceSwipe
const completeDetailToSourceReader = readerCloseInteractions.completeDetailToSourceReader

const feedViewSwipeInteractions = useAppFeedViewSwipeInteractions({
  topChromeProgress,
  feedContentCollapsed,
  motionNormalDuration,
  resolveDelay: motionDelay,
  beginSwipeTransition,
  updateSwipeTransition,
  settleSwipeTransition,
  scheduleSwipeReset,
  swipeTransitionBeginPayload: feedPagerTransition.swipeTransitionBeginPayload,
  swipeTransitionUpdatePayload: feedPagerTransition.swipeTransitionUpdatePayload,
  finishSwipeResult: feedPagerTransition.finishSwipeResult,
  settleFinishedSwipe: feedPagerTransition.settleFinishedSwipe,
  markStartedWithHiddenChrome: feedPagerTransition.markStartedWithHiddenChrome,
  beginTopChromeGestureReturn: chromeState.beginGestureReturn,
  setTopChromeVisible,
  pushRoute,
  topChromeGestureSettleDelayMS: motionDelay(topChromeSettleDuration),
})
const scheduleSwipeTransitionReset = feedViewSwipeInteractions.scheduleSwipeTransitionReset
const beginViewSwipeTransition = feedViewSwipeInteractions.beginViewSwipeTransition
const syncViewSwipeTransition = feedViewSwipeInteractions.syncViewSwipeTransition

const readerBackSwipeInteractions = useAppReaderBackSwipeInteractions({
  pagePull: pagePullState,
  transition: {
    activeFeedSurface,
    pageReturnSurface: 'feed:recommendations',
    beginSwipeTransition,
    updateSwipeTransition,
    transitionBeginPayload: readerBackSwipeTransitionBeginPayload,
    transitionUpdatePayload: readerBackSwipeTransitionUpdatePayload,
  },
  drag: {
    topChromeProgress,
    feedContentCollapsed,
    navigationProgress,
    sourceTimelinePreloadEnabled,
    detailItem,
    readerSource,
    detailSourceKind,
    readerBackSwipeCandidateActive,
    readerBackSwipeTrackingActive,
    windowWidth,
    chromeSettleDuration: topChromeSettleDuration,
    resolveDelay: motionDelay,
    gestureOrigin,
    resetGestureTracking,
    beginReaderBackSwipeCandidateState,
    prepareSourceReaderReturnDragState,
    rememberDetailScrollTop,
    captureVisibleSourceReturnTarget,
    openSourceReader,
    beginReaderBackSwipeDragState,
    updateReaderBackSwipeDragState,
    cancelNavigationCandidates: navigationGesture.cancelCandidates,
    cancelViewSwipeCandidate: feedPagerTransition.cancelViewSwipeCandidate,
    isBackHorizontalSwipe,
    suppressFollowingClick,
    beginTopChromeGestureReturn: chromeState.beginGestureReturn,
    refreshDetailFeedOriginRect,
    captureDetailSourceTransitionRects,
    showSourceReaderUnderDetail,
  },
  completion: {
    switchDistance: viewSwitchDistance,
    finishResult: readerBackSwipeFinishResult,
    cancelResult: readerBackSwipeCancelResult,
    settleTransition: settleSwipeTransition,
    scheduleTransitionReset: () => {
      scheduleSwipeTransitionReset(motionReaderDuration)
    },
    suppressFollowingClick,
    applyAction: applyReaderBackSwipeAction,
    restoreItemExpansion: restoreItemReaderExpansion,
    restoreDetailFromSourceSwipe: restoreDetailFromSourceSwipe,
    restoreParkedSource: restoreParkedSourceReader,
    completeDetailToSource: completeDetailToSourceReader,
    collapseDetail: collapseItemReader,
    restoreDetailFromParkedSource: restoreDetailFromParkedSource,
    returnPage: () => goHome({ closePanel: false }),
    reset: resetBackSwipeOffset,
  },
})
const beginDetailGestureCandidate = readerBackSwipeInteractions.beginDetailGestureCandidate
const updateBackSwipe = readerBackSwipeInteractions.updateBackSwipe
const finishBackSwipe = readerBackSwipeInteractions.finishBackSwipe
const cancelBackSwipe = readerBackSwipeInteractions.cancelBackSwipe

const finishViewSwipe = feedViewSwipeInteractions.finishViewSwipe
const showTopChromeForViewSwipe = feedViewSwipeInteractions.showTopChromeForViewSwipe

const pointerGestureInteractions = useAppPointerGestureInteractions({
  touch: {
    navigationVisible,
    navigationOpen,
    navigationProgress,
    sourceReaderOpen,
    isFeedRoute,
    viewSwipeCandidateActive,
    viewSwipeActive,
    viewDragOffset,
    readerBackSwipeCandidateActive,
    readerBackSwipeTrackingActive,
    navigationOpenDistance,
    viewSwitchDistance,
    gestureOrigin,
    navigationGesture,
    navigationDrawer,
    feedPagerTransition,
    finishCommittedListReturnForGesture,
    resetGestureTracking,
    detailBlocksGestures,
    beginDetailGestureCandidate,
    beginReaderBackSwipeCandidateState,
    resetReaderBackSwipeCandidateState,
    updateBackSwipe,
    finishBackSwipe,
    cancelBackSwipe,
    canStartNavigationOpen,
    canStartViewSwipe,
    isHorizontalSwipe,
    isViewHorizontalSwipe,
    isNavigationDrag,
    settleNavigation,
    showTopChromeForViewSwipe,
    beginViewSwipeTransition,
    syncViewSwipeTransition,
    suppressFollowingClick,
    finishViewSwipe,
  },
  navigationPointer: {
    navigationOpen,
    navigationProgress,
    viewSwipeActive,
    navigationOpenDistance,
    gestureOrigin,
    navigationGesture,
    navigationDrawer,
    finishCommittedListReturnForGesture,
    canStartNavigationOpen,
    cancelViewSwipeCandidate: feedPagerTransition.cancelViewSwipeCandidate,
    isNavigationDrag,
    isHorizontalSwipe,
    isViewHorizontalSwipe,
    settleNavigation,
    resetGestureTracking,
  },
  feedPointer: {
    isFeedRoute,
    navigationVisible,
    navigationProgress,
    viewSwipeCandidateActive,
    viewSwipeActive,
    viewDragOffset,
    viewSwitchDistance,
    gestureOrigin,
    navigationGesture,
    feedPagerTransition,
    canStartViewSwipe,
    finishCommittedListReturnForGesture,
    isViewHorizontalSwipe,
    suppressFollowingClick,
    showTopChromeForViewSwipe,
    beginViewSwipeTransition,
    syncViewSwipeTransition,
    finishViewSwipe,
  },
})
const handleTouchStart = pointerGestureInteractions.handleTouchStart
const handleTouchMove = pointerGestureInteractions.handleTouchMove
const handleTouchEnd = pointerGestureInteractions.handleTouchEnd
const handleTouchCancel = pointerGestureInteractions.handleTouchCancel
const handleWindowPointerDown = pointerGestureInteractions.handleWindowPointerDown
const handleWindowPointerMove = pointerGestureInteractions.handleWindowPointerMove
const handleWindowPointerUp = pointerGestureInteractions.handleWindowPointerUp
const handleWindowPointerCancel = pointerGestureInteractions.handleWindowPointerCancel
const handleFeedPointerDown = pointerGestureInteractions.handleFeedPointerDown
const handleFeedPointerMove = pointerGestureInteractions.handleFeedPointerMove
const handleFeedPointerUp = pointerGestureInteractions.handleFeedPointerUp
const handleFeedPointerCancel = pointerGestureInteractions.handleFeedPointerCancel

const readerDetailInteractions = useAppReaderDetailInteractions({
  progress: {
    detailReaderOpen,
    detailItemID: computed(() => detailItem.value?.id ?? null),
    detailContentRef,
    detailScrollMax,
    detailScrollTop,
    updateDetailScrollMetrics: updateDetailScrollMetricsState,
    updateDetailScrollTop: updateDetailScrollTopState,
    rememberDetailScrollTop,
    scrollDetailContentElementTo,
    suppressFollowingClick,
    setDetailProgressDragging: setDetailProgressDraggingState,
  },
  message: {
    detailReaderOpen,
    detailFrameId,
    navigationVisible,
    readerBackSwipeTrackingActive,
    detailCommittedListReturn,
    isCurrentDetailFrameMessageSource: (source) => source === detailFrameRef.value?.contentWindow,
    updateDetailFrameContentHeight: updateDetailFrameContentHeightState,
    detailFrameViewportOffset,
    beginDetailGestureCandidate,
    updateBackSwipe,
    finishBackSwipe,
    cancelBackSwipe,
    resetGestureTracking,
  },
  settings: {
    setSourceTimelinePreloadEnabled: setSourceTimelinePreloadEnabledState,
  },
})
const syncDetailContainerMetrics = readerDetailInteractions.syncDetailContainerMetrics
const handleDetailProgressChange = readerDetailInteractions.handleDetailProgressChange
const handleDetailProgressDragStart = readerDetailInteractions.handleDetailProgressDragStart
const handleDetailProgressDragEnd = readerDetailInteractions.handleDetailProgressDragEnd
const handleDetailFrameLoad = readerDetailInteractions.handleDetailFrameLoad
const handleMessage = readerDetailInteractions.handleMessage
const clearReaderDetailFrames = readerDetailInteractions.clearReaderDetailFrames
const loadReaderSettings = readerDetailInteractions.loadReaderSettings
const handleReaderSettingsChanged = readerDetailInteractions.handleReaderSettingsChanged

const feedChromeInteractions = useAppFeedChromeInteractions({
  topPull: {
    isFeedRoute,
    topPull: feedTopPull,
    topChromeProgress,
    feedContentCollapsed,
    feedHeaderHeight,
    feedPullRefreshing: () => feedInteraction.pullRefreshing,
    currentContentScrollTop,
    beginRefreshingChrome: chromeState.beginRefreshing,
    setRefreshingProgress: chromeState.setRefreshingProgress,
    commitRefreshingChrome: chromeState.commitRefreshing,
    recordRefreshStartedWithChrome: refreshCompletion.recordStartedWithChrome,
    collapseTopChrome,
    setTopChromeVisible,
  },
  scroll: {
    topChromeProgress,
    feedPullActive,
    sourcePullActive: foregroundSourcePullActive,
    feedTopPulling,
    feedChromeSettling,
    feedContentCollapsed,
    detailReaderOpen,
    detailScrollMax,
    feedHeaderHeight,
    isFeedRoute,
    setTopChromeVisible,
    hideTopChromeForScroll,
    showTopChromeOverlay,
    setTopChromeOverlayProgress,
  },
})
const handleFeedTopPullStart = feedChromeInteractions.handleFeedTopPullStart
const handleFeedTopPullMove = feedChromeInteractions.handleFeedTopPullMove
const handleFeedTopPullEnd = feedChromeInteractions.handleFeedTopPullEnd
const updateTopTabsByScroll = feedChromeInteractions.updateTopTabsByScroll

const appScrollHandlers = useAppScrollHandlers({
  scrollHistory,
  updateFeedScrollTop: feedScroll.update,
  updateSourceReaderScrollTop: updateSourceReaderScrollTopState,
  updateDetailScrollMetrics: updateDetailScrollMetricsState,
  updateTopTabsByScroll,
  scheduleReaderSessionSave,
})
const handleFeedContentScroll = appScrollHandlers.handleFeedContentScroll
const handlePageContentScroll = appScrollHandlers.handlePageContentScroll
const handleSourceReaderScroll = appScrollHandlers.handleSourceReaderScroll
const handleDetailContentScroll = appScrollHandlers.handleDetailContentScroll

const readerStackOutletBindings = useAppReaderStackOutletBindings({
  sourceReaderMounted,
  sourceReaderUnderDetail,
  readerMotion: readerMotionState,
  readerSource,
  sourceToggleActive,
  detailItem,
  readerMorph: readerMorphVisibilityState,
  detailReaderOpen,
  detailParkedBehindSource,
  sourceNotice,
  topChromePhase,
  topChromeProgress,
  sourceHeaderStyle,
  sourceMainLayerStyle,
  sourcePullStatusStyle,
  sourcePullIconStyle,
  sourcePullActive: foregroundSourcePullActive,
  sourcePullRefreshing: () => sourcePullRefreshing.value,
  pullStatusText,
  pullStatusMeta,
  sourceToggleLabel,
  sourceToggleDisabled,
  sourceReaderScrollTop,
  feedHeaderHeight,
  morphingItemId,
  morphingHeightLockItemId,
  morphingItemHeight,
  feedItemPreviewProgress,
  sourceReaderVisible,
  detailLoading,
  detailError,
  detailProgressVisible,
  detailReadingProgress,
  setSourceReaderContentElement,
  handleSourceReaderScroll,
  openNavigation,
  toggleSourceReaderSubscription,
  handleFeedTopPullStart,
  handleFeedTopPullMove,
  handleFeedTopPullEnd,
  openItemReader,
  setDetailContentElement,
  handleDetailContentScroll,
  setDetailInlineSourceElement,
  setDetailFrameElement,
  handleDetailFrameLoad,
  handleDetailProgressDragStart,
  handleDetailProgressDragEnd,
  handleDetailProgressChange,
})
const readerStackOutletProps = readerStackOutletBindings.props
const readerStackOutletListeners = readerStackOutletBindings.listeners

const pagePullInteractions = useAppPagePullInteractions({
  pagePull: pagePullState,
  isFeedRoute,
  topRefreshNoticeDelay,
  currentRefreshPage: pageOutlet.currentRefreshPage,
  clearCurrentPageNotice: pageOutlet.clearCurrentNotice,
  hasRefreshPage: pageOutlet.hasRefreshPage,
  currentContentScrollTop,
  isControlTarget: isPageTopPullControlTarget,
  shouldCancelTopPull,
  shouldWaitForTopPull,
  setTopChromeVisible,
  finishFeedTopPull: () => {
    feedTopPull.finish()
  },
  settlePullOffset: settlePagePullOffset,
  collapseTopChrome,
})
const resetPageTopPullTracking = pagePullInteractions.resetPageTopPullTracking
const invalidatePagePullRefreshCompletion = pagePullInteractions.invalidateRefreshCompletion
const handlePageTouchStart = pagePullInteractions.handlePageTouchStart
const handlePageTouchMove = pagePullInteractions.handlePageTouchMove
const handlePageTouchEnd = pagePullInteractions.handlePageTouchEnd
const handlePageTouchCancel = pagePullInteractions.handlePageTouchCancel
const topChromeOutletState = useAppTopChromeOutletState({
  phase: topChromePhase,
  progress: feedHeaderProgress,
  rootClass: headerClass,
  rootStyle: headerStyle,
  isFeedRoute,
  readerDetailHeader: readerDetailHeaderState,
  feedTabs,
  activeKey: () => route.name,
  feedTabsLayerStyle,
  feedTabsTargetLayerStyle,
  viewSwipeTargetKey,
  feedPullActive,
  feedPullRefreshing: () => feedInteraction.pullRefreshing,
  pullStatusStyle,
  pullIconStyle,
  pullStatusText,
  pullStatusMeta,
  pageTitle,
  pagePullActive,
  pageTitleLayerStyle,
  pagePullStatusStyle,
  pagePullRefreshing,
  pagePullIconStyle,
  pagePullStatusText,
  pagePullStatusMeta,
})

const mainOutletBindings = useAppMainOutletBindings({
  mainClass,
  mainStyle,
  swipePhase,
  swipeDirection,
  swipeProgress,
  swipeIsBlocked,
  topChrome: topChromeOutletState,
  sourceReaderOpen,
  feedContentStyle,
  pageContentStyle,
  feedTrackStyle,
  feedScrollTop,
  topChromeProgress,
  feedContentCollapsed,
  feedHeaderHeight,
  freezeFeedBodyDuringTopRefresh,
  morphingItemId,
  morphingHeightLockItemId,
  morphingItemHeight,
  feedItemPreviewProgress,
  pageContentInnerStyle,
  navigateTo,
  setFeedContentElement,
  handleFeedContentScroll,
  handleFeedPointerDown,
  handleFeedPointerMove,
  handleFeedPointerUp,
  handleFeedPointerCancel,
  handleFeedTopPullStart,
  handleFeedTopPullMove,
  handleFeedTopPullEnd,
  openItemReader,
  setPageContentElement,
  setPageViewInstance,
  handlePageContentScroll,
  handlePageTouchStart,
  handlePageTouchMove,
  handlePageTouchEnd,
  handlePageTouchCancel,
  openSourceReader,
})
const mainOutletProps = mainOutletBindings.props
const mainOutletListeners = mainOutletBindings.listeners

useAppRuntimeEffects({
  windowEvents: {
    handleKeydown,
    handleResize,
    handleMessage,
    handleReaderSettingsChanged,
    handlePopState: virtualBackGuard.handlePopState,
    saveReaderSessionNow,
    handleWindowPointerDown,
    handleWindowPointerMove,
    handleWindowPointerUp,
    handleWindowPointerCancel,
    handleTouchStart,
    handleTouchMove,
    handleTouchEnd,
    handleTouchCancel,
  },
  routeSession: {
    route,
    isFeedRoute,
    navigationVisible,
    sourceReaderVisible,
    readerSource,
    detailItem,
    detailSourceKind,
    detailOpenedFromSourceReader,
    detailListReturnCommitted,
    sourceReaderReturnMode,
    sourceReaderBackDetailItemId,
    parkedDetailStackDepth,
    detailSourceExitProgress,
    topChromeProgress,
    feedContentCollapsed,
    feedScrollTop,
    sourceReaderScrollTop,
    detailScrollTop,
    resetGestureTracking,
    resetBackSwipeOffset,
    resetPageTopPullTracking,
    finishFeedTopPull: feedTopPull.finish,
    resetRefreshCompletion: refreshCompletion.reset,
    resetPagePullMotion: () => {
      invalidatePagePullRefreshCompletion()
      pagePullState.reset()
    },
    resetFeedViewDragOffset,
    resetSwipeTransition,
    setTopChromeVisible,
    currentFeedScrollTop: feedContent.currentScrollTop,
    updateFeedScrollTop: feedScroll.update,
    currentPageScrollTop: pageOutlet.currentScrollTop,
    rememberFeedScrollTop: (scrollTop) => {
      scrollHistory.set('feed', scrollTop)
    },
    rememberPageScrollTop: (scrollTop) => {
      scrollHistory.set('page', scrollTop)
    },
    scheduleReaderSessionSave,
    scheduleReaderURLAndHistorySync,
  },
  feedRefreshCompletion: {
    pullRefreshing: () => feedInteraction.pullRefreshing,
    pullViewKey: () => feedInteraction.pullViewKey,
    feedOrSourcePullActive,
    refreshCompletion,
    topPull: feedTopPull,
    settleDelayMS: () => motionDelay(topChromeSettleDuration),
    settleSourceContentAfterRefresh: () => {
      readerMotionState.settleSourceContentAfterRefresh(topChromeSettleDuration)
    },
    collapseTopChrome,
    canApplyCompletionEffects: ({ wasSource }) => {
      if (wasSource) {
        return sourceReaderVisible.value && !sourceReaderUnderDetail.value && !navigationVisible.value
      }

      return (
        isFeedRoute.value &&
        !detailReaderOpen.value &&
        !sourceReaderOpen.value &&
        !navigationVisible.value
      )
    },
  },
  lifecycle: {
    loadReaderSettings,
    loadTheme: () => themeState.load(),
    installVirtualBackGuard: () => virtualBackGuard.installRouterGuard(),
    uninstallVirtualBackGuard: () => virtualBackGuard.uninstallRouterGuard(),
    waitForRouterReady: () => router.isReady(),
    restoreReaderSession: restoreReaderStateOnLoad,
    markReaderSessionReady: () => routeRuntime.markReaderSessionReady(),
    scheduleReaderURLAndHistorySync,
    saveReaderSessionNow,
    clearRuntimeTimers: [
      () => clearFeedPagerTimers(),
      () => clearSwipeTransitionTimer(),
      () => navigationDrawer.clearTimer(),
      () => refreshCompletion.reset(),
      () => chromeState.clearTimer(),
      () => routeRuntime.clearTimer(),
      () => readerRouteSync.clearTimer(),
      () => readerMotionState.clearSourceContentTimer(),
      clearDetailSourceTransitionRectCapture,
      () => invalidatePagePullRefreshCompletion(),
      () => pagePullState.clearTimers(),
      clearClickSuppressionTimer,
      clearSourceSubscriptionRuntime,
      clearReaderDetailFrames,
      clearReaderSessionScrollRestoreTimers,
      clearReaderStackTimers,
      clearBackSwipeStretchAnchorTimer,
      () => readerSession.clearTimer(),
    ],
  },
})
</script>

<template>
  <div class="app-shell" @click.capture="handleClickCapture">
    <AppNavigationLayer
      :navigation-visible="navigationVisible"
      :detail-chrome-visible="detailChromeVisible"
      :nav-open-button-style="navOpenButtonStyle"
      :corner-button-label="cornerButtonLabel"
      :navigation-scrim-style="navigationScrimStyle"
      :navigation-panel-style="navigationPanelStyle"
      :management-items="managementItems"
      :selected-keys="selectedKeys"
      :dark-theme="darkTheme"
      :settings-active="route.name === 'settings'"
      @corner-click="handleCornerButtonClick"
      @close-navigation="closeNavigation"
      @go-home="goHome({ closePanel: true })"
      @menu-click="handleMenuClick"
      @toggle-theme="toggleTheme"
      @open-settings="pushRoute('/settings'); closeNavigation()"
    />

    <AppMainOutlet v-bind="mainOutletProps" v-on="mainOutletListeners" />

    <AppReaderStackOutlet v-bind="readerStackOutletProps" v-on="readerStackOutletListeners" />
  </div>
</template>
