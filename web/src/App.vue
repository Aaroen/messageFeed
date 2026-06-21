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
import { useVirtualBackGuard } from '@/composables/useVirtualBackGuard'
import { useClickSuppression } from '@/composables/useClickSuppression'
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
import { useAppRouteState } from '@/composables/useAppRouteState'
import { useAppScrollHandlers } from '@/composables/useAppScrollHandlers'
import { useFeedRefreshCompletionWatcher } from '@/composables/useFeedRefreshCompletionWatcher'
import { useAppNavigationActions } from '@/composables/useAppNavigationActions'
import { useAppNavigationConfig } from '@/composables/useAppNavigationConfig'
import { useAppVirtualBackActions } from '@/composables/useAppVirtualBackActions'
import { useAppReaderSessionSnapshots } from '@/composables/useAppReaderSessionSnapshots'
import { useAppRouteSessionWatchers } from '@/composables/useAppRouteSessionWatchers'
import { useAppTopChromeActions } from '@/composables/useAppTopChromeActions'
import { useAppReaderSessionPersistence } from '@/composables/useAppReaderSessionPersistence'
import { useAppFeedChromeState } from '@/composables/useAppFeedChromeState'
import { useAppElementRefs } from '@/composables/useAppElementRefs'
import { useAppGestureResetAction } from '@/composables/useAppGestureResetAction'
import { useAppReaderStackActions } from '@/composables/useAppReaderStackActions'
import { useAppWindowEventListeners } from '@/composables/useAppWindowEventListeners'
import { useAppInteractionTargetGuards } from '@/composables/useAppInteractionTargetGuards'
import { useAppLifecycle } from '@/composables/useAppLifecycle'
import { useAppShellEventActions } from '@/composables/useAppShellEventActions'
import { useAppReaderScrollMemoryActions } from '@/composables/useAppReaderScrollMemoryActions'
import { useAppReaderRouteSyncAction } from '@/composables/useAppReaderRouteSyncAction'
import { useAppReaderStackOutletBindings } from '@/composables/useAppReaderStackOutletBindings'
import { useAppMainOutletBindings } from '@/composables/useAppMainOutletBindings'
import { useAppPagePullState } from '@/composables/useAppPagePullState'
import { useAppPagePullInteractions } from '@/composables/useAppPagePullInteractions'
import { useAppReaderBackSwipeInteractions } from '@/composables/useAppReaderBackSwipeInteractions'
import { useAppReaderSourceCloseInteractions } from '@/composables/useAppReaderSourceCloseInteractions'
import { useAppReaderOpenInteractions } from '@/composables/useAppReaderOpenInteractions'
import { useAppReaderCloseInteractions } from '@/composables/useAppReaderCloseInteractions'
import { useAppReaderMotionState } from '@/composables/useAppReaderMotionState'
import { useAppReaderTransitionRects } from '@/composables/useAppReaderTransitionRects'
import { useAppReaderDetailInteractions } from '@/composables/useAppReaderDetailInteractions'
import { useAppReaderSession } from '@/composables/useAppReaderSession'
import { useAppReaderRouteSyncBinding } from '@/composables/useAppReaderRouteSyncBinding'
import { useAppFeedChromeInteractions } from '@/composables/useAppFeedChromeInteractions'
import { useAppChromeVisualState } from '@/composables/useAppChromeVisualState'
import { useAppPointerGestureInteractions } from '@/composables/useAppPointerGestureInteractions'
import { useAppFeedViewSwipeInteractions } from '@/composables/useAppFeedViewSwipeInteractions'
import { useAppGesturePolicy } from '@/composables/useAppGesturePolicy'
import { useAppSwipeNavigationState } from '@/composables/useAppSwipeNavigationState'
import { useReaderRouteQueryRestore } from '@/composables/useReaderRouteQueryRestore'
import { useAppReaderStackRuntime } from '@/composables/useAppReaderStackRuntime'
import { useAppReaderMorphVisibilityState } from '@/composables/useAppReaderMorphVisibilityState'
import { useAppReaderDetailHeaderState } from '@/composables/useAppReaderDetailHeaderState'
import { useAppTopChromeOutletState } from '@/composables/useAppTopChromeOutletState'

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
const {
  sourceReaderContentRef,
  detailContentRef,
  detailFrameRef,
  detailInlineSourceRef,
  sourceReaderScrollTop,
  detailReaderTouchOffset,
  detailReaderStretch,
  sourceReaderOffset,
  sourceReaderStretch,
  detailStretchAnchor,
  sourceStretchAnchor,
  readerBackDragging,
  sourceReaderBlockedBackSwipeActive,
  sourceReaderReturnTargetPending,
  readerMotionSettling,
  readerSource,
  sourceReaderRefreshNonce,
  sourceReaderVisible,
  detailItem,
  detailLoading,
  detailError,
  detailSourceKind,
  detailOriginRect,
  detailSourceItemTargetRect,
  detailSourceNameOriginRect,
  detailSourceNameTargetRect,
  morphingItemId,
  morphingHeightLockItemId,
  morphingItemHeight,
  detailOpenedFromSourceReader,
  detailEntryProgress,
  detailEntrySettling,
  detailHeaderPreviousTitle,
  detailHeaderSwapProgress,
  detailBackExitProgress,
  detailSourceExitProgress,
  detailReturningToFeed,
  detailListReturnCommitted,
  detailRestoringFromSourceReader,
  sourceReaderReturnMode,
  detailScrollTop,
  detailScrollHeight,
  detailScrollClientHeight,
  detailFrameContentHeight,
  detailProgressDragging,
  parkedDetailStackDepth,
  sourceReaderBackDetailItemId,
  sourceCatalogEntry,
  sourceSubscription,
  sourceSubscriptionLoading,
  sourceNotice,
  sourceTimelinePreloadEnabled,
  detailTransitionRectsLocked,
  detailFeedOriginLocked,
  sourceReaderMounted,
  sourceReaderOpen,
  detailReaderOpen,
  sourceReaderUnderDetail,
  sourceReaderRevealProgress,
  sourceNameMorphProgress,
  detailSurfaceProgress,
  detailScrollMax,
  detailReadingProgress,
  detailProgressVisible,
  feedItemPreviewProgress,
  sourceNameTransitionActive,
  sourceTitleProgress,
  sourceTitleRevealProgress,
  sourceTitleRevealReady,
  sourceNameMorphActive,
  sourceNameMorphVisible,
  detailMorphSummaryVisible,
  detailMorphTextVisible,
  detailHeaderTitleSwapping,
  detailSourceListTitleProgress,
  detailHeaderFeedTitleProgress,
  sourceNameMorphLabelOpacity,
  sourceNameMorphLabelBlur,
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
  clearNoticeTimer: clearSourceNoticeTimer,
  resetSourceSubscriptionState,
  loadSourceReaderSubscription,
  toggleSourceReaderSubscription,
} = useAppReaderStackRuntime()
const feedScroll = useFeedScrollState()
const feedScrollTop = feedScroll.scrollTop
const chromeState = useChromeState()
const topChromeProgress = chromeState.progress
const topChromePhase = chromeState.phase
const feedContentCollapsed = chromeState.contentCollapsed
const feedChromeSettling = chromeState.settling
const clickSuppression = useClickSuppression()
const viewportSize = useViewportSize({ defaultWidth: 1440, defaultHeight: 900 })
const windowWidth = viewportSize.width
const windowHeight = viewportSize.height
const motionTimings = useMotionTimings()
const motionQuickDuration = motionTimings.quickDuration
const motionNormalDuration = motionTimings.normalDuration
const motionStretchAnchorClearDuration = motionTimings.stretchAnchorClearDuration
const motionHeaderSwapDuration = motionTimings.headerSwapDuration
const motionReaderDuration = motionTimings.readerDuration
const motionChromeDuration = motionTimings.chromeDuration
const navigationDrawerSettleDuration = motionTimings.navigationDrawerSettleDuration
const sourceReaderCloseCleanupDelay = motionTimings.sourceReaderCloseCleanupDelay
const topRefreshNoticeDelay = motionTimings.topRefreshNoticeDelay
const viewSwipeChromeRevealDelay = motionTimings.viewSwipeChromeRevealDelay
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
const navigationSettling = navigationDrawer.settling
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
const appReaderRouteSyncAction = useAppReaderRouteSyncAction()
const scheduleReaderURLAndHistorySync = appReaderRouteSyncAction.scheduleReaderURLAndHistorySync
const appReaderScrollMemoryActions = useAppReaderScrollMemoryActions({
  rememberScrollTop: (surface, scrollTop) => {
    scrollHistory.set(surface, scrollTop)
  },
})
const rememberSourceScrollTop = appReaderScrollMemoryActions.rememberSourceScrollTop
const rememberDetailScrollTop = appReaderScrollMemoryActions.rememberDetailScrollTop
const appReaderSessionSnapshots = useAppReaderSessionSnapshots({
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
  rememberSourceScrollTop,
  rememberDetailScrollTop,
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
})
const readerSessionSnapshot = appReaderSessionSnapshots.readerSessionSnapshot
const applyReaderSessionSnapshot = appReaderSessionSnapshots.applyReaderSessionSnapshot
const clearReaderSessionScrollRestoreTimers = appReaderSessionSnapshots.clearScrollRestoreTimers
const readerSession = useAppReaderSession({
  createSnapshot: readerSessionSnapshot,
  getCurrentRouteFullPath: () => route.fullPath,
  restoreSnapshot: applyReaderSessionSnapshot,
  afterRestore: scheduleReaderURLAndHistorySync,
})
const appReaderSessionPersistence = useAppReaderSessionPersistence({
  restoring: readerSession.restoring,
  canSaveReaderSession: routeRuntime.canSaveReaderSession,
  saveNow: readerSession.saveNow,
  scheduleSave: readerSession.scheduleSave,
  restore: readerSession.restore,
})
const saveReaderSessionNow = appReaderSessionPersistence.saveReaderSessionNow
const scheduleReaderSessionSave = appReaderSessionPersistence.scheduleReaderSessionSave
const restoreReaderSession = appReaderSessionPersistence.restoreReaderSession

const appRouteState = useAppRouteState(route)
const selectedKeys = appRouteState.selectedKeys
const pageTitle = appRouteState.pageTitle
const isFeedRoute = appRouteState.isFeedRoute
const cornerButtonLabel = appRouteState.cornerButtonLabel
const pagePullState = useAppPagePullState({ pageTitle })
const pagePullOffset = pagePullState.offset
const pagePullSettling = pagePullState.settling
const pagePullRefreshing = pagePullState.refreshing
const pagePullProgress = pagePullState.progress
const pageContentInnerStyle = pagePullState.contentStyle
const pagePullStatusText = pagePullState.statusText
const pagePullStatusMeta = pagePullState.statusMeta
const appVirtualBackActions = useAppVirtualBackActions({
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
  goHome: (replace) => goHome(replace),
})
const hasVirtualBackTarget = appVirtualBackActions.hasVirtualBackTarget
const runVirtualBackAnimation = appVirtualBackActions.runVirtualBackAnimation
const virtualBackGuard = useVirtualBackGuard({
  route,
  router,
  getRouteFullPath: () => route.fullPath,
  getState: () => {
    const needsVirtualLayer = hasVirtualBackTarget()
    return {
      shouldGuard: needsVirtualLayer || isFeedRoute.value,
      needsVirtualLayer,
      needsHomeGuard: !needsVirtualLayer && isFeedRoute.value,
    }
  },
  canHandleNavigation: routeRuntime.canHandleNavigation,
  consumeBack: runVirtualBackAnimation,
  onBackConsumed: () => {
    scheduleReaderSessionSave()
    scheduleReaderURLAndHistorySync(true)
  },
})
useAppReaderRouteSyncBinding({
  bindReaderRouteSync: appReaderRouteSyncAction.bindReaderRouteSync,
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
const appSwipeNavigationState = useAppSwipeNavigationState<SwipeSurface>({
  feedPager: {
    getActiveKey: () => route.name,
    getWindowWidth: () => windowWidth.value,
    isFeedRoute: () => isFeedRoute.value,
    isDetailReaderOpen: () => detailReaderOpen.value,
  },
})
const swipePhase = appSwipeNavigationState.swipePhase
const swipeDirection = appSwipeNavigationState.swipeDirection
const swipeProgress = appSwipeNavigationState.swipeProgress
const swipeIsBlocked = appSwipeNavigationState.swipeIsBlocked
const beginSwipeTransition = appSwipeNavigationState.beginSwipeTransition
const updateSwipeTransition = appSwipeNavigationState.updateSwipeTransition
const settleSwipeTransition = appSwipeNavigationState.settleSwipeTransition
const scheduleSwipeReset = appSwipeNavigationState.scheduleSwipeReset
const clearSwipeTransitionTimer = appSwipeNavigationState.clearSwipeTransitionTimer
const feedPagerTransition = appSwipeNavigationState.feedPagerTransition
const gesturePolicy = useAppGesturePolicy({
  direction: {
    viewDragThreshold: appSwipeNavigationState.feedPagerDragThreshold,
  },
  startGuards: {
    isFeedRoute,
    navigationVisible,
    sourceReaderOpen,
    isSubscriptionsRoute: () => route.name === 'subscriptions',
    detailBlocksGestures,
  },
})
const isHorizontalSwipe = gesturePolicy.isHorizontalSwipe
const isViewHorizontalSwipe = gesturePolicy.isViewHorizontalSwipe
const isNavigationDrag = gesturePolicy.isNavigationDrag
const isBackHorizontalSwipe = gesturePolicy.isBackHorizontalSwipe
const shouldCancelTopPull = gesturePolicy.shouldCancelTopPull
const shouldWaitForTopPull = gesturePolicy.shouldWaitForTopPull
const canStartViewSwipe = gesturePolicy.canStartViewSwipe
const canStartNavigationOpen = gesturePolicy.canStartNavigationOpen
const viewDragOffset = appSwipeNavigationState.viewDragOffset
const viewSettling = appSwipeNavigationState.viewSettling
const viewSwipeCandidateActive = appSwipeNavigationState.viewSwipeCandidateActive
const viewSwipeActive = appSwipeNavigationState.viewSwipeActive
const activeFeedIndex = appSwipeNavigationState.activeFeedIndex
const feedChromeState = useAppFeedChromeState({
  pullActivity: {
    isFeedRoute,
    pagePullRefreshing,
    pagePullOffset,
    sourceReaderOpen,
    getFeedPullActive: () => feedInteraction.pullActive,
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
  },
  visibility: {
    isFeedRoute,
    topChromeProgress,
    detailReaderOpen,
    sourceReaderOpen,
    detailChromeVisible,
  },
})
const feedPullActive = feedChromeState.feedPullActive
const pagePullActive = feedChromeState.pagePullActive
const sourcePullActive = feedChromeState.sourcePullActive
const feedOrSourcePullActive = feedChromeState.feedOrSourcePullActive
const pullProgress = feedChromeState.pullProgress
const sourcePullProgress = feedChromeState.sourcePullProgress
const feedHeaderHeight = feedChromeState.feedHeaderHeight
const feedHeaderProgress = feedChromeState.feedHeaderProgress
const freezeFeedBodyDuringTopRefresh = feedChromeState.freezeFeedBodyDuringTopRefresh
const feedTopChromeIsVisiblyOpen = feedChromeState.feedTopChromeIsVisiblyOpen
const feedHeaderReturnProgress = feedChromeState.feedHeaderReturnProgress
const mainStyle = feedChromeState.mainStyle
const feedChromeHidden = feedChromeState.feedChromeHidden
const feedTabsLayerHidden = feedChromeState.feedTabsLayerHidden
const feedCornerHidden = feedChromeState.feedCornerHidden
const detailHeaderVisible = feedChromeState.detailHeaderVisible
const headerDetailLayoutActive = feedChromeState.headerDetailLayoutActive
const { statusText: pullStatusText, statusMeta: pullStatusMeta } = storeToRefs(feedInteraction)
const feedTrackStyle = appSwipeNavigationState.feedTrackStyle
const viewSwipeTargetKey = appSwipeNavigationState.viewSwipeTargetKey
const viewSwipeTargetVisible = appSwipeNavigationState.viewSwipeTargetVisible
const viewSwipeTargetProgress = appSwipeNavigationState.viewSwipeTargetProgress
const chromeVisualState = useAppChromeVisualState({
  layer: {
    feedPullActive,
    feedPullRefreshing: () => feedInteraction.pullRefreshing,
    pullProgress,
    pagePullActive,
    pagePullRefreshing,
    pagePullProgress,
    detailReaderOpen,
    feedHeaderReturnProgress,
    readerBackDragging,
    detailBlocksGestures,
    feedHeaderProgress,
    viewSwipeTargetVisible,
    viewSwipeTargetProgress,
    sourcePullActive,
    sourcePullProgress,
    topChromeProgress,
    feedHeaderHeight,
    feedChromeSettling,
    feedRefreshSettling,
    feedTopPulling,
    feedCornerHidden,
    detailHeaderVisible,
    headerDetailLayoutActive,
  },
  mainClass: {
    isFeedRoute,
    feedChromeHidden,
    feedPullActive,
    feedPullRefreshing: () => feedInteraction.pullRefreshing,
    pagePullActive,
    freezeFeedBodyDuringTopRefresh,
    feedRefreshSettling,
    feedChromeSettling,
    pagePullSettling,
    viewSettling,
    detailReaderOpen,
    detailReturningToFeed,
    detailChromeVisible,
  },
})
const pullStatusStyle = chromeVisualState.pullStatusStyle
const pullIconStyle = chromeVisualState.pullIconStyle
const pagePullStatusStyle = chromeVisualState.pagePullStatusStyle
const pagePullIconStyle = chromeVisualState.pagePullIconStyle
const feedTabsLayerStyle = chromeVisualState.feedTabsLayerStyle
const feedTabsTargetLayerStyle = chromeVisualState.feedTabsTargetLayerStyle
const sourcePullStatusStyle = chromeVisualState.sourcePullStatusStyle
const sourcePullIconStyle = chromeVisualState.sourcePullIconStyle
const sourceHeaderStyle = chromeVisualState.sourceHeaderStyle
const detailHeaderLayerStyle = chromeVisualState.detailHeaderLayerStyle
const pageTitleLayerStyle = chromeVisualState.pageTitleLayerStyle
const sourceMainLayerStyle = chromeVisualState.sourceMainLayerStyle
const headerClass = chromeVisualState.headerClass
const headerStyle = chromeVisualState.headerStyle
const navOpenButtonStyle = chromeVisualState.navOpenButtonStyle
const mainClass = chromeVisualState.mainClass
const readerMotionState = useAppReaderMotionState({
  layout: {
    windowWidth,
    windowHeight,
    feedHeaderHeight,
    topChromeProgress,
    feedContentCollapsed,
  },
  sourceSurface: {
    feedHeaderHeight,
    darkTheme,
    visible: sourceReaderVisible,
    underDetail: sourceReaderUnderDetail,
    revealProgress: sourceReaderRevealProgress,
    offset: sourceReaderOffset,
    stretch: sourceReaderStretch,
    stretchAnchor: sourceStretchAnchor,
    dragging: readerBackDragging,
    blocksGestures: detailBlocksGestures,
  },
  sourceContent: {
    headerHeight: feedHeaderHeight,
    isVisible: () => sourceReaderVisible.value,
    resolveDelay: motionDelay,
  },
  detailSurface: {
    stretch: detailReaderStretch,
    stretchAnchor: detailStretchAnchor,
    dragging: readerBackDragging,
    blockedBackSwipeActive: sourceReaderBlockedBackSwipeActive,
    returningToFeed: detailReturningToFeed,
    surfaceProgress: detailSurfaceProgress,
    committedListReturn: detailCommittedListReturn,
  },
  detailContent: {
    surfaceProgress: detailSurfaceProgress,
    sourceExitProgress: detailSourceExitProgress,
    frameContentHeight: detailFrameContentHeight,
    dragging: readerBackDragging,
    committedListReturn: detailCommittedListReturn,
  },
  detailProgress: {
    visible: detailProgressVisible,
    dragging: detailProgressDragging,
    readerBackDragging,
    readingProgress: detailReadingProgress,
  },
  detailText: {
    surfaceProgress: detailSurfaceProgress,
    sourceListTitleProgress: detailSourceListTitleProgress,
    headerFeedTitleProgress: detailHeaderFeedTitleProgress,
    feedHeaderReturnProgress,
    headerTitleSwapping: detailHeaderTitleSwapping,
    headerSwapProgress: detailHeaderSwapProgress,
    sourceLabelOpacity: sourceNameMorphLabelOpacity,
    sourceLabelBlur: sourceNameMorphLabelBlur,
    readerBackDragging,
    committedListReturn: detailCommittedListReturn,
  },
  sourceTitle: {
    revealReady: sourceTitleRevealReady,
    pullActive: sourcePullActive,
    titleProgress: sourceTitleProgress,
    revealProgress: sourceTitleRevealProgress,
    nameOriginRect: detailSourceNameOriginRect,
    nameTargetRect: detailSourceNameTargetRect,
    nameMorphProgress: sourceNameMorphProgress,
    windowWidth,
    headerHeight: feedHeaderHeight,
    readerBackDragging,
  },
  detailTransition: {
    originRect: detailOriginRect,
    sourceItemTargetRect: detailSourceItemTargetRect,
    restoringFromSourceReader: detailRestoringFromSourceReader,
    sourceExitProgress: detailSourceExitProgress,
    backExitProgress: detailBackExitProgress,
    surfaceProgress: detailSurfaceProgress,
    windowWidth,
    windowHeight,
    darkTheme,
    readerBackDragging,
    sourceReturnTargetPending: sourceReaderReturnTargetPending,
    blockedBackSwipeActive: sourceReaderBlockedBackSwipeActive,
    returningToFeed: detailReturningToFeed,
    committedListReturn: detailCommittedListReturn,
  },
  detailFrame: {
    item: detailItem,
    metricsInitialDelay: detailFrameMetricsInitialDelay,
    metricsSettledDelay: detailFrameMetricsSettledDelay,
  },
})
const sourceContentStyle = readerMotionState.sourceContentStyle
const sourceReaderStyle = readerMotionState.sourceReaderStyle
const detailReaderStyle = readerMotionState.detailReaderStyle
const detailTransitionSurfaceStyle = readerMotionState.detailTransitionSurfaceStyle
const detailContentStyle = readerMotionState.detailContentStyle
const detailProgressStyle = readerMotionState.detailProgressStyle
const detailProgressFillStyle = readerMotionState.detailProgressFillStyle
const detailProgressThumbStyle = readerMotionState.detailProgressThumbStyle
const detailMorphTextStyle = readerMotionState.detailMorphTextStyle
const detailHeaderTitleStyle = readerMotionState.detailHeaderTitleStyle
const detailHeaderCurrentTextStyle = readerMotionState.detailHeaderCurrentTextStyle
const detailHeaderPreviousTextStyle = readerMotionState.detailHeaderPreviousTextStyle
const detailInlineSourceStyle = readerMotionState.detailInlineSourceStyle
const detailMorphSourceLabelStyle = readerMotionState.detailMorphSourceLabelStyle
const sourceTitleRevealVisible = readerMotionState.sourceTitleRevealVisible
const sourceNameMorphStyle = readerMotionState.sourceNameMorphStyle
const sourceTitleLayerStyle = readerMotionState.sourceTitleLayerStyle
const sourceTitleTextStyle = readerMotionState.sourceTitleTextStyle
const sourceTitleRevealStyle = readerMotionState.sourceTitleRevealStyle
const detailPreviewSummary = readerMotionState.detailPreviewSummary
const detailDisplayDate = readerMotionState.detailDisplayDate
const detailSrcdoc = readerMotionState.detailSrcdoc
const readerMorphVisibilityState = useAppReaderMorphVisibilityState({
  readerSource,
  sourceToggleActive,
  sourceTitleRevealVisible,
  sourceTitleRevealStyle,
  detailItem,
  sourceNameMorphVisible,
  sourceNameMorphStyle,
  detailMorphTextVisible,
  detailMorphTextStyle,
  detailMorphSourceLabelStyle,
  detailDisplayDate,
  detailMorphSummaryVisible,
  detailPreviewSummary,
})
const readerDetailHeaderState = useAppReaderDetailHeaderState({
  chromeVisible: detailChromeVisible,
  readerOpen: detailReaderOpen,
  visible: detailHeaderVisible,
  layerStyle: detailHeaderLayerStyle,
  item: detailItem,
  titleStyle: detailHeaderTitleStyle,
  previousTitle: detailHeaderPreviousTitle,
  previousTextStyle: detailHeaderPreviousTextStyle,
  currentTextStyle: detailHeaderCurrentTextStyle,
})

const { resetGestureTracking } = useAppGestureResetAction({
  resetNavigationGesture: navigationGesture.reset,
  resetFeedViewSwipeTracking: appSwipeNavigationState.resetFeedViewSwipeTracking,
  clearFeedViewStartedWithHiddenChrome: appSwipeNavigationState.clearFeedViewStartedWithHiddenChrome,
  resetReaderBackSwipeCandidate: resetReaderBackSwipeCandidateState,
})

const { managementItems, feedTabs } = useAppNavigationConfig()
const appNavigation = useAppNavigationActions({
  router,
  routeRuntime,
  navigationDrawer,
  feedPagerTransition,
  managementItems,
  resetGestureTracking,
  setChromeStableVisible: chromeState.setStableVisible,
  motionDelay,
  motionNormalDuration,
})
const pushRoute = appNavigation.pushRoute
const replaceRoute = appNavigation.replaceRoute
const settleNavigation = appNavigation.settleNavigation
const openNavigation = appNavigation.openNavigation
const closeNavigation = appNavigation.closeNavigation
const handleMenuClick = appNavigation.handleMenuClick
const goHome = appNavigation.goHome
const handleCornerButtonClick = appNavigation.handleCornerButtonClick
const navigateTo = appNavigation.navigateTo

const navigationOpenDistance = 72
const viewSwitchDistance = 62
const topChromeSettleDuration = motionChromeDuration
const appTopChromeActions = useAppTopChromeActions({
  sourceReaderOpen,
  sourceReaderScrollTop,
  isFeedRoute,
  feedScrollTop,
  topChromeSettleDuration,
  resolveDelay: motionDelay,
  setChromeVisible: chromeState.setVisible,
  setChromeCollapsedHidden: chromeState.setCollapsedHidden,
  currentPageScrollTop: pageOutlet.currentScrollTop,
  settlePagePullOffset: pagePullState.settleOffset,
})
const setTopChromeVisible = appTopChromeActions.setTopChromeVisible
const collapseTopChrome = appTopChromeActions.collapseTopChrome
const currentContentScrollTop = appTopChromeActions.currentContentScrollTop
const settlePagePullOffset = appTopChromeActions.settlePagePullOffset

const appInteractionTargetGuards = useAppInteractionTargetGuards()
const isPageTopPullControlTarget = appInteractionTargetGuards.isPageTopPullControlTarget

const appShellEventActions = useAppShellEventActions({
  consumeClick: (event) => clickSuppression.consume(event),
  suppressNextClick: () => clickSuppression.suppressNext(),
  closeNavigation,
  syncViewportSize: () => viewportSize.sync(),
})
const handleClickCapture = appShellEventActions.handleClickCapture
const suppressFollowingClick = appShellEventActions.suppressFollowingClick
const handleKeydown = appShellEventActions.handleKeydown
const handleResize = appShellEventActions.handleResize

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
  viewSwipeChromeRevealDelay,
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
  scheduleDelayedCommit: feedPagerTransition.scheduleDelayedCommit,
  markStartedWithHiddenChrome: feedPagerTransition.markStartedWithHiddenChrome,
  setTopChromeVisible,
  pushRoute,
})
const scheduleSwipeTransitionReset = feedViewSwipeInteractions.scheduleSwipeTransitionReset
const beginViewSwipeTransition = feedViewSwipeInteractions.beginViewSwipeTransition
const syncViewSwipeTransition = feedViewSwipeInteractions.syncViewSwipeTransition

const readerBackSwipeInteractions = useAppReaderBackSwipeInteractions({
  pagePull: pagePullState,
  transition: {
    activeFeedSurface: appSwipeNavigationState.activeFeedSurface,
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
    returnPage: () => goHome(false),
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
    feedTopChromeIsVisiblyOpen,
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
    sourcePullActive,
    feedTopPulling,
    feedChromeSettling,
    detailReaderOpen,
    detailScrollMax,
    feedHeaderHeight,
    isFeedRoute,
    setTopChromeVisible,
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
  sourceReaderStyle,
  readerSource,
  sourceToggleActive,
  detailItem,
  readerMorph: readerMorphVisibilityState,
  detailReaderOpen,
  readerMotionSettling,
  detailReturningToFeed,
  detailReaderStyle,
  sourceNotice,
  topChromePhase,
  topChromeProgress,
  sourceHeaderStyle,
  sourceTitleTextStyle,
  sourceTitleLayerStyle,
  sourceMainLayerStyle,
  sourcePullStatusStyle,
  sourcePullIconStyle,
  sourcePullActive,
  sourcePullRefreshing: () => feedInteraction.pullRefreshing,
  pullStatusText,
  pullStatusMeta,
  sourceToggleLabel,
  sourceToggleDisabled,
  sourceContentStyle,
  sourceReaderRefreshNonce,
  sourceReaderScrollTop,
  feedHeaderHeight,
  morphingItemId,
  morphingHeightLockItemId,
  morphingItemHeight,
  feedItemPreviewProgress,
  sourceReaderVisible,
  detailEntrySettling,
  feedChromeSettling,
  detailTransitionSurfaceStyle,
  detailContentStyle,
  detailLoading,
  detailError,
  detailSrcdoc,
  detailInlineSourceStyle,
  detailProgressVisible,
  detailProgressDragging,
  detailReadingProgress,
  detailProgressStyle,
  detailProgressFillStyle,
  detailProgressThumbStyle,
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
  feedTabsLayerHidden,
  feedTabsLayerStyle,
  viewSwipeTargetVisible,
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
  viewSettling,
  feedTrackStyle,
  feedScrollTop,
  topChromeProgress,
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

const appWindowEventListeners = useAppWindowEventListeners({
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
})
const installWindowEventListeners = appWindowEventListeners.installWindowEventListeners
const uninstallWindowEventListeners = appWindowEventListeners.uninstallWindowEventListeners

useAppRouteSessionWatchers({
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
  resetPageTopPullTracking,
  finishFeedTopPull: feedTopPull.finish,
  resetPagePullMotion: pagePullState.resetMotion,
  resetFeedViewDragOffset: appSwipeNavigationState.resetFeedViewDragOffset,
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

useFeedRefreshCompletionWatcher({
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
})

useAppLifecycle({
  loadReaderSettings,
  loadTheme: () => themeState.load(),
  installVirtualBackGuard: () => virtualBackGuard.installRouterGuard(),
  uninstallVirtualBackGuard: () => virtualBackGuard.uninstallRouterGuard(),
  waitForRouterReady: () => router.isReady(),
  restoreReaderSession: restoreReaderStateOnLoad,
  markReaderSessionReady: () => routeRuntime.markReaderSessionReady(),
  scheduleReaderURLAndHistorySync,
  installWindowEventListeners,
  uninstallWindowEventListeners,
  saveReaderSessionNow,
  clearRuntimeTimers: [
    () => appSwipeNavigationState.clearFeedPagerTimers(),
    () => clearSwipeTransitionTimer(),
    () => navigationDrawer.clearTimer(),
    () => refreshCompletion.clearTimer(),
    () => chromeState.clearTimer(),
    () => readerMotionState.clearSourceContentTimer(),
    clearDetailSourceTransitionRectCapture,
    () => pagePullState.clearTimers(),
    () => clickSuppression.clearTimer(),
    clearSourceNoticeTimer,
    clearReaderDetailFrames,
    clearReaderSessionScrollRestoreTimers,
    clearReaderStackTimers,
    clearBackSwipeStretchAnchorTimer,
    () => readerSession.clearTimer(),
  ],
})
</script>

<template>
  <div class="app-shell" @click.capture="handleClickCapture">
    <AppNavigationLayer
      :navigation-visible="navigationVisible"
      :navigation-settling="navigationSettling"
      :feed-corner-hidden="feedCornerHidden"
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
      @go-home="goHome(true)"
      @menu-click="handleMenuClick"
      @toggle-theme="toggleTheme"
      @open-settings="pushRoute('/settings'); closeNavigation()"
    />

    <AppMainOutlet v-bind="mainOutletProps" v-on="mainOutletListeners" />

    <AppReaderStackOutlet v-bind="readerStackOutletProps" v-on="readerStackOutletListeners" />
  </div>
</template>
