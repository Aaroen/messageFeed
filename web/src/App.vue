<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

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
import { useAppNavigationRuntime } from '@/composables/useAppNavigationRuntime'
import { useAppChromeRuntime } from '@/composables/useAppChromeRuntime'
import { useAppElementRefs } from '@/composables/useAppElementRefs'
import { useAppShellRuntime } from '@/composables/useAppShellRuntime'
import { useAppRuntimeEffects } from '@/composables/useAppRuntimeEffects'
import { useAppReaderSessionRuntime } from '@/composables/useAppReaderSessionRuntime'
import { useAppReaderRouteSyncBinding } from '@/composables/useAppReaderRouteSyncBinding'
import { useAppSwipeGestureRuntime } from '@/composables/useAppSwipeGestureRuntime'
import { useAppReaderStackRuntime } from '@/composables/useAppReaderStackRuntime'
import { useAppReaderPresentationRuntime } from '@/composables/useAppReaderPresentationRuntime'
import { useAppVirtualBackGuard } from '@/composables/useAppVirtualBackGuard'
import { useAppFeedChromeScrollRuntime } from '@/composables/useAppFeedChromeScrollRuntime'
import { useAppReaderNavigationRuntime } from '@/composables/useAppReaderNavigationRuntime'
import { useAppReaderStackOutletRuntime } from '@/composables/useAppReaderStackOutletRuntime'
import { useAppGestureInteractionRuntime } from '@/composables/useAppGestureInteractionRuntime'
import { useAppMainOutletRuntime } from '@/composables/useAppMainOutletRuntime'
import { useAppRuntimeCleanup } from '@/composables/useAppRuntimeCleanup'
import { useAppFeedPullInteractionRuntime } from '@/composables/useAppFeedPullInteractionRuntime'

type SwipeSurface =
  | 'feed:subscriptions'
  | 'feed:recommendations'
  | 'reader:detail'
  | 'reader:source'
  | 'page:management'

const route = useRoute()
const router = useRouter()
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
const feedPullInteraction = useAppFeedPullInteractionRuntime()
const feedTopPull = feedPullInteraction.topPull
const finishFeedTopPull = feedPullInteraction.finishTopPull
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

const routePageRuntime = useAppRoutePageRuntime(route)
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
} = routePageRuntime
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
  feedPullBusy: feedPullInteraction.feedPullBusy,
})
const chromeRuntime = useAppChromeRuntime({
  feed: {
    pullActivity: {
      isFeedRoute,
      pagePullRefreshing,
      pagePullOffset,
      sourceReaderOpen,
    },
    layout: {
      windowWidth,
      isFeedRoute,
      topChromeProgress,
      feedContentCollapsed,
      detailFeedHeaderReturnProgress,
    },
    shellMotion: {
      detailSurfaceProgress,
      feedChromeSettling,
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
  feedPull: feedPullInteraction,
  visual: {
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
  },
  feedRefreshCompletion: {
    isFeedRoute,
    detailReaderOpen,
    sourceReaderOpen,
    sourceReaderVisible,
    sourceReaderUnderDetail,
    navigationVisible,
  },
  mainClass: {
    isFeedRoute,
  },
  topChromeActions: {
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
  },
})
const feedPullActive = chromeRuntime.feedPullActive
const feedPullRefreshing = chromeRuntime.feedPullRefreshing
const feedRefreshCompletionRuntime = chromeRuntime.feedRefreshCompletion
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
const pullStatusText = chromeRuntime.pullStatusText
const pullStatusMeta = chromeRuntime.pullStatusMeta
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
const topChromeActions = chromeRuntime.topChromeActions
const setTopChromeVisible = topChromeActions.setTopChromeVisible
const hideTopChromeForScroll = topChromeActions.hideTopChromeForScroll
const showTopChromeOverlay = topChromeActions.showTopChromeOverlay
const setTopChromeOverlayProgress = topChromeActions.setTopChromeOverlayProgress
const collapseTopChrome = topChromeActions.collapseTopChrome
const currentContentScrollTop = topChromeActions.currentContentScrollTop
const settlePagePullOffset = topChromeActions.settlePagePullOffset
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

const readerNavigationRuntime = useAppReaderNavigationRuntime({
  transitionRects: {
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
  },
  stackActions: {
    quickDuration: motionQuickDuration,
    restoreMorphingItemContentWithDelay,
    scheduleHiddenSourceReaderCleanupWithDelay,
    restorePreviousParkedDetail: restoreReaderStackPreviousParkedDetail,
    rememberDetailScrollTop,
  },
  open: {
    sourceOpen: {
      openSourceReaderState,
      getReaderSource: () => readerSource.value,
      clearHiddenSourceCleanupTimer,
      setTopChromeVisible,
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
      finishFeedTopPull,
      rememberDetailScrollTop,
      scrollDetailContentElementTo,
      scheduleReaderSessionSave,
    },
  },
  sourceClose: {
    parkedRestore: {
      detailReaderOpen,
      readerDuration: motionReaderDuration,
      resolveDelay: motionDelay,
      suppressFollowingClick,
      restoreDetailFromParkedSourceWithDelay,
      clearMorphingHeightUnlockTimer,
      setTopChromeVisible,
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
    },
  },
  close: {
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
      suppressFollowingClick,
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
    },
  },
})
const openSourceReader = readerNavigationRuntime.openSourceReader
const showSourceReaderUnderDetail = readerNavigationRuntime.showSourceReaderUnderDetail
const openItemReader = readerNavigationRuntime.openItemReader

const readerRouteQueryRestore = readerNavigationRuntime.installRouteQueryRestore({
  route,
  restoreReaderSession,
})
const restoreReaderStateOnLoad = readerRouteQueryRestore.restoreReaderStateOnLoad

const restoreDetailFromParkedSource = readerNavigationRuntime.restoreDetailFromParkedSource
const restoreSourceReaderBackTarget = readerNavigationRuntime.restoreSourceReaderBackTarget
const closeSourceReader = readerNavigationRuntime.closeSourceReader
const finishCommittedListReturnForGesture = readerNavigationRuntime.finishCommittedListReturnForGesture
const closeItemReader = readerNavigationRuntime.closeItemReader
const collapseItemReader = readerNavigationRuntime.collapseItemReader
const resetBackSwipeOffset = readerNavigationRuntime.resetBackSwipeOffset
const clearBackSwipeStretchAnchorTimer = readerNavigationRuntime.clearBackSwipeStretchAnchorTimer
const restoreParkedSourceReader = readerNavigationRuntime.restoreParkedSourceReader
const restoreItemReaderExpansion = readerNavigationRuntime.restoreItemReaderExpansion
const restoreDetailFromSourceSwipe = readerNavigationRuntime.restoreDetailFromSourceSwipe
const completeDetailToSourceReader = readerNavigationRuntime.completeDetailToSourceReader
const refreshDetailFeedOriginRect = readerNavigationRuntime.refreshDetailFeedOriginRect
const captureDetailSourceTransitionRects = readerNavigationRuntime.captureDetailSourceTransitionRects
const captureVisibleSourceReturnTarget = readerNavigationRuntime.captureVisibleSourceReturnTarget
const clearDetailSourceTransitionRectCapture =
  readerNavigationRuntime.clearDetailSourceTransitionRectCapture

const gestureInteractionRuntime = useAppGestureInteractionRuntime<SwipeSurface, typeof activeFeedSurface.value>({
  feedView: {
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
  },
  readerBackSwipe: {
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
      finishResult: readerBackSwipeFinishResult,
      cancelResult: readerBackSwipeCancelResult,
      settleTransition: settleSwipeTransition,
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
    transitionResetDuration: motionReaderDuration,
  },
  pointer: {
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
      gestureOrigin,
      navigationGesture,
      navigationDrawer,
      feedPagerTransition,
      finishCommittedListReturnForGesture,
      resetGestureTracking,
      detailBlocksGestures,
      beginReaderBackSwipeCandidateState,
      resetReaderBackSwipeCandidateState,
      canStartNavigationOpen,
      canStartViewSwipe,
      isHorizontalSwipe,
      isViewHorizontalSwipe,
      isNavigationDrag,
      settleNavigation,
      suppressFollowingClick,
    },
    navigationPointer: {
      navigationOpen,
      navigationProgress,
      viewSwipeActive,
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
      gestureOrigin,
      navigationGesture,
      feedPagerTransition,
      canStartViewSwipe,
      finishCommittedListReturnForGesture,
      isViewHorizontalSwipe,
      suppressFollowingClick,
    },
  },
})
const beginDetailGestureCandidate = gestureInteractionRuntime.beginDetailGestureCandidate
const updateBackSwipe = gestureInteractionRuntime.updateBackSwipe
const finishBackSwipe = gestureInteractionRuntime.finishBackSwipe
const cancelBackSwipe = gestureInteractionRuntime.cancelBackSwipe
const handleTouchStart = gestureInteractionRuntime.handleTouchStart
const handleTouchMove = gestureInteractionRuntime.handleTouchMove
const handleTouchEnd = gestureInteractionRuntime.handleTouchEnd
const handleTouchCancel = gestureInteractionRuntime.handleTouchCancel
const handleWindowPointerDown = gestureInteractionRuntime.handleWindowPointerDown
const handleWindowPointerMove = gestureInteractionRuntime.handleWindowPointerMove
const handleWindowPointerUp = gestureInteractionRuntime.handleWindowPointerUp
const handleWindowPointerCancel = gestureInteractionRuntime.handleWindowPointerCancel
const handleFeedPointerDown = gestureInteractionRuntime.handleFeedPointerDown
const handleFeedPointerMove = gestureInteractionRuntime.handleFeedPointerMove
const handleFeedPointerUp = gestureInteractionRuntime.handleFeedPointerUp
const handleFeedPointerCancel = gestureInteractionRuntime.handleFeedPointerCancel

const feedChromeScrollRuntime = useAppFeedChromeScrollRuntime({
  feedChrome: {
    topPull: {
      isFeedRoute,
      topPull: feedTopPull,
      topChromeProgress,
      feedContentCollapsed,
      feedHeaderHeight,
      currentContentScrollTop,
      beginRefreshingChrome: chromeState.beginRefreshing,
      setRefreshingProgress: chromeState.setRefreshingProgress,
      commitRefreshingChrome: chromeState.commitRefreshing,
      recordRefreshStartedWithChrome: feedRefreshCompletionRuntime.recordRefreshStartedWithChrome,
      collapseTopChrome,
      setTopChromeVisible,
    },
    feedPull: feedPullInteraction,
    scroll: {
      topChromeProgress,
      feedPullActive,
      sourcePullActive: foregroundSourcePullActive,
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
    refreshCompletion: {
      installWatcher: feedRefreshCompletionRuntime.installRefreshCompletionWatcher,
      feedOrSourcePullActive,
      settleDelayMS: () => motionDelay(topChromeSettleDuration),
      settleSourceContentAfterRefresh: () => {
        readerMotionState.settleSourceContentAfterRefresh(topChromeSettleDuration)
      },
    },
  },
  scroll: {
    scrollHistory,
    updateFeedScrollTop: feedScroll.update,
    updateSourceReaderScrollTop: updateSourceReaderScrollTopState,
    updateDetailScrollMetrics: updateDetailScrollMetricsState,
    scheduleReaderSessionSave,
  },
})
const handleFeedTopPullStart = feedChromeScrollRuntime.handleFeedTopPullStart
const handleFeedTopPullMove = feedChromeScrollRuntime.handleFeedTopPullMove
const handleFeedTopPullEnd = feedChromeScrollRuntime.handleFeedTopPullEnd
const handleFeedContentScroll = feedChromeScrollRuntime.handleFeedContentScroll
const handlePageContentScroll = feedChromeScrollRuntime.handlePageContentScroll
const handleSourceReaderScroll = feedChromeScrollRuntime.handleSourceReaderScroll
const handleDetailContentScroll = feedChromeScrollRuntime.handleDetailContentScroll

const readerStackOutletRuntime = useAppReaderStackOutletRuntime({
  detail: {
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
  },
  outlet: {
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
  },
})
const syncDetailContainerMetrics = readerStackOutletRuntime.syncDetailContainerMetrics
const handleMessage = readerStackOutletRuntime.handleMessage
const clearReaderDetailFrames = readerStackOutletRuntime.clearReaderDetailFrames
const loadReaderSettings = readerStackOutletRuntime.loadReaderSettings
const handleReaderSettingsChanged = readerStackOutletRuntime.handleReaderSettingsChanged
const readerStackOutletProps = readerStackOutletRuntime.props
const readerStackOutletListeners = readerStackOutletRuntime.listeners

const pagePullInteractions = routePageRuntime.installPagePullInteractions({
  topRefreshNoticeDelay,
  currentRefreshPage: pageOutlet.currentRefreshPage,
  clearCurrentPageNotice: pageOutlet.clearCurrentNotice,
  hasRefreshPage: pageOutlet.hasRefreshPage,
  currentContentScrollTop,
  shouldCancelTopPull,
  shouldWaitForTopPull,
  setTopChromeVisible,
  finishFeedTopPull,
  settlePullOffset: settlePagePullOffset,
  collapseTopChrome,
})
const resetPageTopPullTracking = pagePullInteractions.resetPageTopPullTracking
const invalidatePagePullRefreshCompletion = pagePullInteractions.invalidateRefreshCompletion
const handlePageTouchStart = pagePullInteractions.handlePageTouchStart
const handlePageTouchMove = pagePullInteractions.handlePageTouchMove
const handlePageTouchEnd = pagePullInteractions.handlePageTouchEnd
const handlePageTouchCancel = pagePullInteractions.handlePageTouchCancel
const mainOutletRuntime = useAppMainOutletRuntime({
  topChrome: {
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
    feedPullRefreshing,
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
  },
  mainOutlet: {
    mainClass,
    mainStyle,
    swipePhase,
    swipeDirection,
    swipeProgress,
    swipeIsBlocked,
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
  },
})
const mainOutletProps = mainOutletRuntime.props
const mainOutletListeners = mainOutletRuntime.listeners

routeRuntime.installRouteSessionWatchers({
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
  finishFeedTopPull,
  resetRefreshCompletion: feedRefreshCompletionRuntime.resetRefreshCompletion,
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
})

const runtimeCleanup = useAppRuntimeCleanup({
  swipe: {
    clearFeedPagerTimers,
    clearSwipeTransitionTimer,
  },
  navigation: {
    clearTimer: navigationDrawer.clearTimer,
  },
  feedRefresh: {
    resetRefreshCompletion: feedRefreshCompletionRuntime.resetRefreshCompletion,
  },
  chrome: {
    clearTimer: chromeState.clearTimer,
  },
  route: {
    clearTimer: routeRuntime.clearTimer,
  },
  readerRouteSync: {
    clearTimer: readerRouteSync.clearTimer,
  },
  readerMotion: {
    clearSourceContentTimer: readerMotionState.clearSourceContentTimer,
  },
  detailSourceTransition: {
    clearRectCapture: clearDetailSourceTransitionRectCapture,
  },
  pagePull: {
    invalidateRefreshCompletion: invalidatePagePullRefreshCompletion,
    clearTimers: pagePullState.clearTimers,
  },
  shell: {
    clearClickSuppressionTimer,
  },
  sourceSubscription: {
    clearRuntime: clearSourceSubscriptionRuntime,
  },
  readerDetailFrames: {
    clear: clearReaderDetailFrames,
  },
  readerSessionScrollRestore: {
    clearTimers: clearReaderSessionScrollRestoreTimers,
  },
  readerStack: {
    clearTimers: clearReaderStackTimers,
    clearBackSwipeStretchAnchorTimer,
  },
  readerSession: {
    clearTimer: readerSession.clearTimer,
  },
})

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
    clearRuntimeTimers: runtimeCleanup.clearRuntimeTimers,
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
