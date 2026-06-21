<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { useFeedInteractionStore } from '@/stores/feedInteraction'
import { getFeedItem } from '@/api/feed'
import AppMainOutlet from '@/components/AppMainOutlet.vue'
import AppNavigationLayer from '@/components/AppNavigationLayer.vue'
import AppReaderStackOutlet from '@/components/AppReaderStackOutlet.vue'
import { useChromeState } from '@/composables/useChromeState'
import { useReaderSourceSubscription } from '@/composables/useReaderSourceSubscription'
import { useReaderBackSwipeCompletion } from '@/composables/useReaderBackSwipeCompletion'
import { useReaderBackSwipeDragHandlers } from '@/composables/useReaderBackSwipeDragHandlers'
import { useReaderBackSwipeResetAction } from '@/composables/useReaderBackSwipeResetAction'
import { usePullRefresh } from '@/composables/usePullRefresh'
import {
  type FeedSourceKind,
  type ReaderSessionSnapshot,
  type ReaderSource,
  useReaderSession,
} from '@/composables/useReaderSession'
import { useReaderStackState } from '@/composables/useReaderStackState'
import {
  browserRouteFullPath,
  readerRouteMatchesCurrent,
  useReaderRouteSync,
} from '@/composables/useReaderRouteSync'
import { useNavigationDrawer } from '@/composables/useNavigationDrawer'
import { useSwipeTransition } from '@/composables/useSwipeTransition'
import { useVirtualBackGuard } from '@/composables/useVirtualBackGuard'
import { useFeedPagerTransition } from '@/composables/useFeedPagerTransition'
import { usePageContentMotion } from '@/composables/usePageContentMotion'
import { useClickSuppression } from '@/composables/useClickSuppression'
import { useSourceContentMotion } from '@/composables/useSourceContentMotion'
import { useRefreshCompletionState } from '@/composables/useRefreshCompletionState'
import { useAppChromeLayerState } from '@/composables/useAppChromeLayerState'
import { useReaderSourceSurfaceMotion } from '@/composables/useReaderSourceSurfaceMotion'
import { useReaderDetailSurfaceMotion } from '@/composables/useReaderDetailSurfaceMotion'
import { useReaderDetailContentMotion } from '@/composables/useReaderDetailContentMotion'
import { useReaderDetailSourceTransitionRects } from '@/composables/useReaderDetailSourceTransitionRects'
import { useReaderDetailProgressMotion } from '@/composables/useReaderDetailProgressMotion'
import { useReaderDetailTextMotion } from '@/composables/useReaderDetailTextMotion'
import { useReaderSourceTitleMotion } from '@/composables/useReaderSourceTitleMotion'
import { useReaderDetailTransitionMotion } from '@/composables/useReaderDetailTransitionMotion'
import { useAppShellMotion } from '@/composables/useAppShellMotion'
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
import { useGestureDirection } from '@/composables/useGestureDirection'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { useReaderDetailFrame } from '@/composables/useReaderDetailFrame'
import { useReaderLayoutState } from '@/composables/useReaderLayoutState'
import { useAppRouteState } from '@/composables/useAppRouteState'
import { useAppMainClassState } from '@/composables/useAppMainClassState'
import { usePagePullStatus } from '@/composables/usePagePullStatus'
import { usePagePullGestureHandlers } from '@/composables/usePagePullGestureHandlers'
import { useAppScrollHandlers } from '@/composables/useAppScrollHandlers'
import { useTopChromeScrollBehavior } from '@/composables/useTopChromeScrollBehavior'
import { useFeedTopPullHandlers } from '@/composables/useFeedTopPullHandlers'
import { useFeedRefreshCompletionWatcher } from '@/composables/useFeedRefreshCompletionWatcher'
import { usePagePullRefreshAction } from '@/composables/usePagePullRefreshAction'
import { useFeedViewSwipeController } from '@/composables/useFeedViewSwipeController'
import { useReaderBackSwipeTransitionController } from '@/composables/useReaderBackSwipeTransitionController'
import { useFeedPointerSwipeHandlers } from '@/composables/useFeedPointerSwipeHandlers'
import { useNavigationPointerHandlers } from '@/composables/useNavigationPointerHandlers'
import { useAppTouchGestureHandlers } from '@/composables/useAppTouchGestureHandlers'
import { useReaderDetailProgressHandlers } from '@/composables/useReaderDetailProgressHandlers'
import { useReaderDetailMessageHandler } from '@/composables/useReaderDetailMessageHandler'
import { useReaderSourceOpenAction } from '@/composables/useReaderSourceOpenAction'
import { useReaderSourceRevealAction } from '@/composables/useReaderSourceRevealAction'
import { useReaderSourceCloseAction } from '@/composables/useReaderSourceCloseAction'
import { useReaderItemOpenAction } from '@/composables/useReaderItemOpenAction'
import { useReaderItemCloseAction } from '@/composables/useReaderItemCloseAction'
import { useReaderRestoreActions } from '@/composables/useReaderRestoreActions'
import { useReaderParkedDetailRestoreAction } from '@/composables/useReaderParkedDetailRestoreAction'
import { useAppNavigationActions } from '@/composables/useAppNavigationActions'
import { useAppNavigationConfig } from '@/composables/useAppNavigationConfig'
import { useAppGestureStartGuards } from '@/composables/useAppGestureStartGuards'
import { useAppVirtualBackActions } from '@/composables/useAppVirtualBackActions'
import { useAppReaderSessionSnapshots } from '@/composables/useAppReaderSessionSnapshots'
import { useAppRouteSessionWatchers } from '@/composables/useAppRouteSessionWatchers'
import { useAppTopChromeActions } from '@/composables/useAppTopChromeActions'
import { useAppReaderSessionPersistence } from '@/composables/useAppReaderSessionPersistence'
import { usePullActivityState } from '@/composables/usePullActivityState'
import { useFeedChromeLayoutState } from '@/composables/useFeedChromeLayoutState'
import { useFeedChromeVisibilityState } from '@/composables/useFeedChromeVisibilityState'
import { useAppElementRefs } from '@/composables/useAppElementRefs'
import { useAppGestureResetAction } from '@/composables/useAppGestureResetAction'
import { useAppReaderStackActions } from '@/composables/useAppReaderStackActions'
import { useAppWindowEventListeners } from '@/composables/useAppWindowEventListeners'
import { useReaderSettingsSync } from '@/composables/useReaderSettingsSync'
import { useAppInteractionTargetGuards } from '@/composables/useAppInteractionTargetGuards'
import { useAppLifecycle } from '@/composables/useAppLifecycle'
import { useAppShellEventActions } from '@/composables/useAppShellEventActions'
import { useAppReaderScrollMemoryActions } from '@/composables/useAppReaderScrollMemoryActions'
import { useAppReaderRouteSyncAction } from '@/composables/useAppReaderRouteSyncAction'
import { useAppReaderStackOutletBindings } from '@/composables/useAppReaderStackOutletBindings'

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
} = useReaderStackState()
const {
  sourceToggleLabel,
  sourceToggleActive,
  sourceToggleDisabled,
  clearNoticeTimer: clearSourceNoticeTimer,
  resetSourceSubscriptionState,
  loadSourceReaderSubscription,
  toggleSourceReaderSubscription,
} = useReaderSourceSubscription({
  sourceCatalogEntry,
  sourceSubscription,
  sourceSubscriptionLoading,
  sourceNotice,
  getReaderSource: () => readerSource.value,
  setSourceCatalogEntry: setSourceCatalogEntryState,
  setSourceSubscription: setSourceSubscriptionState,
  setSourceSubscriptionLoading: setSourceSubscriptionLoadingState,
  setSourceNotice: setSourceNoticeState,
})
const feedScroll = useFeedScrollState()
const feedScrollTop = feedScroll.scrollTop
const chromeState = useChromeState()
const topChromeProgress = chromeState.progress
const topChromePhase = chromeState.phase
const feedContentCollapsed = chromeState.contentCollapsed
const feedChromeSettling = chromeState.settling
const swipeTransition = useSwipeTransition<SwipeSurface>()
const swipePhase = swipeTransition.phase
const swipeDirection = swipeTransition.direction
const swipeProgress = swipeTransition.progress
const swipeIsBlocked = swipeTransition.isBlocked
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
const detailFrameMetricsInitialDelay = motionTimings.detailFrameMetricsInitialDelay
const detailFrameMetricsSettledDelay = motionTimings.detailFrameMetricsSettledDelay
const readerScrollRestoreRetryDelay = motionTimings.readerScrollRestoreRetryDelay
const readerScrollRestoreSettledDelay = motionTimings.readerScrollRestoreSettledDelay
const readerMorphDuration = motionTimings.readerMorphDuration
const readerRectRetryDelay = motionTimings.readerRectRetryDelay
const motionDelay = motionTimings.delay
const navigationDrawer = useNavigationDrawer({ windowWidth, resolveDelay: motionDelay })
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
const pagePullRefresh = usePullRefresh({ threshold: 52 })
const pagePullOffset = pagePullRefresh.offset
const pagePullDistance = pagePullRefresh.distance
const pagePullSettling = pagePullRefresh.settling
const pagePullRefreshing = pagePullRefresh.refreshing
const pageContentMotion = usePageContentMotion({ pullOffset: pagePullOffset })
const pageSideStretch = pageContentMotion.sideStretch
const pageContentInnerStyle = pageContentMotion.contentStyle
const homeExitDoubleBackMs = 1600
const homeBackGuard = useDoubleBackGuard(homeExitDoubleBackMs)
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
const readerSession = useReaderSession<ReaderSessionSnapshot>({
  storageKey: 'messagefeed-reader-session-v1',
  maxAgeMS: 24 * 60 * 60 * 1000,
  saveDelayMS: 80,
  createSnapshot: readerSessionSnapshot,
  getCurrentRouteFullPath: () => route.fullPath,
  matchesCurrentRoute: (snapshotRouteFullPath) =>
    readerRouteMatchesCurrent([route.fullPath, browserRouteFullPath()], snapshotRouteFullPath),
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
const readerRouteSync = useReaderRouteSync({
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
appReaderRouteSyncAction.bindReaderRouteSync(readerRouteSync)
const feedPagerTransition = useFeedPagerTransition({
  getActiveKey: () => route.name,
  getWindowWidth: () => windowWidth.value,
  isFeedRoute: () => isFeedRoute.value,
  isDetailReaderOpen: () => detailReaderOpen.value,
})
const gestureDirection = useGestureDirection({ viewDragThreshold: feedPagerTransition.dragThreshold })
const isHorizontalSwipe = gestureDirection.isHorizontalSwipe
const isViewHorizontalSwipe = gestureDirection.isViewHorizontalSwipe
const isNavigationDrag = gestureDirection.isNavigationDrag
const isBackHorizontalSwipe = gestureDirection.isBackHorizontalSwipe
const shouldCancelTopPull = gestureDirection.shouldCancelTopPull
const shouldWaitForTopPull = gestureDirection.shouldWaitForTopPull
const viewDragOffset = feedPagerTransition.dragOffset
const viewSettling = feedPagerTransition.settling
const viewSwipeCandidateActive = feedPagerTransition.viewSwipeCandidateActive
const viewSwipeActive = feedPagerTransition.viewSwipeActive
const activeFeedIndex = feedPagerTransition.activeIndex
const pullActivity = usePullActivityState({
  isFeedRoute,
  pagePullRefreshing,
  pagePullOffset,
  sourceReaderOpen,
  getFeedPullActive: () => feedInteraction.pullActive,
  getFeedPullOffset: () => feedInteraction.pullOffset,
  getFeedPullViewKey: () => feedInteraction.pullViewKey,
})
const feedPullActive = pullActivity.feedActive
const pagePullActive = pullActivity.pageActive
const sourcePullActive = pullActivity.sourceActive
const feedOrSourcePullActive = pullActivity.feedOrSourceActive
const pullProgress = pullActivity.feedProgress
const pagePullProgress = pagePullRefresh.distanceProgress
const sourcePullProgress = pullActivity.sourceProgress
const feedChromeLayout = useFeedChromeLayoutState({
  windowWidth,
  isFeedRoute,
  topChromeProgress,
  feedPullActive,
  pullProgress,
  feedTopPullStartedWithChrome,
  refreshStartedWithChrome,
  feedTopPulling,
  feedContentCollapsed,
  detailFeedHeaderReturnProgress,
})
const feedHeaderHeight = feedChromeLayout.headerHeight
const feedHeaderProgress = feedChromeLayout.headerProgress
const feedContentSpace = feedChromeLayout.contentSpace
const freezeFeedBodyDuringTopRefresh = feedChromeLayout.freezeBodyDuringTopRefresh
const feedTopChromeIsVisiblyOpen = feedChromeLayout.topChromeIsVisiblyOpen
const feedHeaderReturnProgress = feedChromeLayout.headerReturnProgress
const appShellMotion = useAppShellMotion({
  feedHeaderHeight,
  feedContentSpace,
  detailSurfaceProgress,
})
const feedChromeVisibility = useFeedChromeVisibilityState({
  isFeedRoute,
  topChromeProgress,
  feedHeaderProgress,
  feedPullActive,
  detailReaderOpen,
  feedHeaderReturnProgress,
  sourceReaderOpen,
  detailChromeVisible,
})
const feedChromeHidden = feedChromeVisibility.feedChromeHidden
const feedTabsLayerHidden = feedChromeVisibility.feedTabsLayerHidden
const feedCornerHidden = feedChromeVisibility.feedCornerHidden
const detailHeaderVisible = feedChromeVisibility.detailHeaderVisible
const headerDetailLayoutActive = feedChromeVisibility.headerDetailLayoutActive
const { statusText: pullStatusText, statusMeta: pullStatusMeta } = storeToRefs(feedInteraction)
const pagePullStatus = usePagePullStatus({
  refreshing: pagePullRefreshing,
  progress: pagePullProgress,
  pageTitle,
})
const pagePullStatusText = pagePullStatus.text
const pagePullStatusMeta = pagePullStatus.meta
const feedTrackStyle = feedPagerTransition.trackStyle
const viewSwipeTargetKey = feedPagerTransition.targetKey
const viewSwipeTargetVisible = feedPagerTransition.targetVisible
const viewSwipeTargetProgress = feedPagerTransition.targetProgress
const appChromeLayerState = useAppChromeLayerState({
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
})
const pullStatusStyle = appChromeLayerState.pullStatusStyle
const pullIconStyle = appChromeLayerState.pullIconStyle
const pagePullStatusStyle = appChromeLayerState.pagePullStatusStyle
const pagePullIconStyle = appChromeLayerState.pagePullIconStyle
const feedTabsLayerStyle = appChromeLayerState.feedTabsLayerStyle
const feedTabsTargetLayerStyle = appChromeLayerState.feedTabsTargetLayerStyle
const sourcePullStatusStyle = appChromeLayerState.sourcePullStatusStyle
const sourcePullIconStyle = appChromeLayerState.sourcePullIconStyle
const sourceHeaderStyle = appChromeLayerState.sourceHeaderStyle
const detailHeaderLayerStyle = appChromeLayerState.detailHeaderLayerStyle
const pageTitleLayerStyle = appChromeLayerState.pageTitleLayerStyle
const sourceMainLayerStyle = appChromeLayerState.sourceMainLayerStyle
const headerClass = appChromeLayerState.headerClass
const headerStyle = appChromeLayerState.headerStyle
const navOpenButtonStyle = appChromeLayerState.navOpenButtonStyle
const appMainClassState = useAppMainClassState({
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
})
const mainClass = appMainClassState.mainClass
const readerLayoutState = useReaderLayoutState({
  windowWidth,
  windowHeight,
  feedHeaderHeight,
  topChromeProgress,
  feedContentCollapsed,
})
const sourceHeaderSpace = readerLayoutState.sourceHeaderSpace
const detailSourceFallbackTargetRect = readerLayoutState.detailSourceFallbackTargetRect
const detailSurfaceMargin = readerLayoutState.detailSurfaceMargin
const detailExpandedTop = readerLayoutState.detailExpandedTop
const detailFrameMinHeight = readerLayoutState.detailFrameMinHeight
const sourceSurfaceMotion = useReaderSourceSurfaceMotion({
  feedHeaderHeight,
  headerSpace: sourceHeaderSpace,
  darkTheme,
  visible: sourceReaderVisible,
  underDetail: sourceReaderUnderDetail,
  revealProgress: sourceReaderRevealProgress,
  offset: sourceReaderOffset,
  stretch: sourceReaderStretch,
  stretchAnchor: sourceStretchAnchor,
  dragging: readerBackDragging,
  blocksGestures: detailBlocksGestures,
})
const sourceContentMotion = useSourceContentMotion({
  headerSpace: sourceHeaderSpace,
  headerHeight: feedHeaderHeight,
  isVisible: () => sourceReaderVisible.value,
  resolveDelay: motionDelay,
})
const sourceContentStyle = sourceContentMotion.contentStyle
const sourceReaderStyle = sourceSurfaceMotion.surfaceStyle
const detailSurfaceMotion = useReaderDetailSurfaceMotion({
  stretch: detailReaderStretch,
  stretchAnchor: detailStretchAnchor,
  dragging: readerBackDragging,
  blockedBackSwipeActive: sourceReaderBlockedBackSwipeActive,
  returningToFeed: detailReturningToFeed,
  surfaceProgress: detailSurfaceProgress,
  committedListReturn: detailCommittedListReturn,
})
const detailReaderStyle = detailSurfaceMotion.readerStyle
const detailContentMotion = useReaderDetailContentMotion({
  surfaceProgress: detailSurfaceProgress,
  sourceExitProgress: detailSourceExitProgress,
  frameMinHeight: detailFrameMinHeight,
  frameContentHeight: detailFrameContentHeight,
  dragging: readerBackDragging,
  committedListReturn: detailCommittedListReturn,
})
const detailProgressMotion = useReaderDetailProgressMotion({
  surfaceMargin: detailSurfaceMargin,
  expandedTop: detailExpandedTop,
  visible: detailProgressVisible,
  dragging: detailProgressDragging,
  readerBackDragging,
  readingProgress: detailReadingProgress,
})
const detailTextMotion = useReaderDetailTextMotion({
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
})
const sourceTitleMotion = useReaderSourceTitleMotion({
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
})
const detailTransitionMotion = useReaderDetailTransitionMotion({
  originRect: detailOriginRect,
  sourceItemTargetRect: detailSourceItemTargetRect,
  fallbackTargetRect: detailSourceFallbackTargetRect,
  restoringFromSourceReader: detailRestoringFromSourceReader,
  sourceExitProgress: detailSourceExitProgress,
  backExitProgress: detailBackExitProgress,
  surfaceProgress: detailSurfaceProgress,
  surfaceMargin: detailSurfaceMargin,
  expandedTop: detailExpandedTop,
  windowWidth,
  windowHeight,
  darkTheme,
  readerBackDragging,
  sourceReturnTargetPending: sourceReaderReturnTargetPending,
  blockedBackSwipeActive: sourceReaderBlockedBackSwipeActive,
  returningToFeed: detailReturningToFeed,
  committedListReturn: detailCommittedListReturn,
})
const detailTransitionSurfaceStyle = detailTransitionMotion.surfaceStyle
const detailContentStyle = detailContentMotion.contentStyle
const detailProgressStyle = detailProgressMotion.railStyle
const detailProgressFillStyle = detailProgressMotion.fillStyle
const detailProgressThumbStyle = detailProgressMotion.thumbStyle
const detailMorphTextStyle = detailTextMotion.morphTextStyle
const detailHeaderTitleStyle = detailTextMotion.headerTitleStyle
const detailHeaderCurrentTextStyle = detailTextMotion.headerCurrentTextStyle
const detailHeaderPreviousTextStyle = detailTextMotion.headerPreviousTextStyle
const detailInlineSourceStyle = detailTextMotion.inlineSourceStyle
const detailMorphSourceLabelStyle = detailTextMotion.morphSourceLabelStyle
const sourceTitleRevealVisible = sourceTitleMotion.revealVisible
const sourceNameMorphStyle = sourceTitleMotion.nameMorphStyle
const sourceTitleLayerStyle = sourceTitleMotion.titleLayerStyle
const sourceTitleTextStyle = sourceTitleMotion.titleTextStyle
const sourceTitleRevealStyle = sourceTitleMotion.revealStyle
const mainStyle = appShellMotion.style
const readerDetailFrame = useReaderDetailFrame({
  item: detailItem,
  metricsInitialDelay: detailFrameMetricsInitialDelay,
  metricsSettledDelay: detailFrameMetricsSettledDelay,
})
const detailPreviewSummary = readerDetailFrame.previewSummary
const detailDisplayDate = readerDetailFrame.displayDate
const detailSrcdoc = readerDetailFrame.srcdoc

const { resetGestureTracking } = useAppGestureResetAction({
  resetNavigationGesture: navigationGesture.reset,
  resetFeedViewSwipeTracking: feedPagerTransition.resetViewSwipeTracking,
  clearFeedViewStartedWithHiddenChrome: feedPagerTransition.clearStartedWithHiddenChrome,
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
const viewSwipeChromeRevealDelay = 520
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
  settlePagePullOffset: pagePullRefresh.settleOffset,
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

const readerDetailSourceTransitionRects = useReaderDetailSourceTransitionRects({
  sourceReaderContentRef,
  detailInlineSourceRef,
  detailItem,
  detailFeedOriginLocked,
  detailTransitionRectsLocked,
  retryDelay: readerRectRetryDelay,
  findFeedItemElement: (itemID) => feedContent.findItemElement(itemID, activeFeedIndex.value),
  applyDetailFeedOriginRectState,
  applyDetailSourceTransitionRectsState,
  applyVisibleSourceReturnTargetState,
})
const refreshDetailFeedOriginRect = readerDetailSourceTransitionRects.refreshDetailFeedOriginRect
const captureDetailSourceTransitionRects = readerDetailSourceTransitionRects.captureDetailSourceTransitionRects
const captureVisibleSourceReturnTarget = readerDetailSourceTransitionRects.captureVisibleSourceReturnTarget

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

const readerSourceOpenAction = useReaderSourceOpenAction({
  openSourceReaderState,
  clearHiddenSourceCleanupTimer,
  setTopChromeVisible,
  captureDetailSourceTransitionRects,
  loadSourceReaderSubscription,
  resetSourceSubscriptionState,
  rememberSourceScrollTop,
  scrollSourceReaderContentElementTo,
})
const openSourceReader = readerSourceOpenAction.openSourceReader

const readerSourceRevealAction = useReaderSourceRevealAction({
  detailItem,
  detailSourceKind,
  readerSource,
  openSourceReader,
  setTopChromeVisible,
  revealSourceReaderUnderDetailState,
  captureDetailSourceTransitionRects,
})
const showSourceReaderUnderDetail = readerSourceRevealAction.showSourceReaderUnderDetail

const readerItemOpenAction = useReaderItemOpenAction({
  sourceReaderOpen,
  readerSource,
  sourceTimelinePreloadEnabled,
  headerSwapDuration: motionHeaderSwapDuration,
  detailEntryDuration: readerMorphDuration,
  resolveDelay: motionDelay,
  openItemReaderWithTransition,
  openSourceReader,
  loadFeedItem: getFeedItem,
  finishOpenItemReaderLoad,
  setChromeStableVisible: chromeState.setStableVisible,
  finishFeedTopPull: feedTopPull.finish,
  rememberDetailScrollTop,
  captureDetailSourceTransitionRects,
  scrollDetailContentElementTo,
  scheduleReaderSessionSave,
})
const openItemReader = readerItemOpenAction.openItemReader

const readerParkedDetailRestoreAction = useReaderParkedDetailRestoreAction({
  detailReaderOpen,
  readerDuration: motionReaderDuration,
  resolveDelay: motionDelay,
  closeSourceReader: () => closeSourceReader(),
  suppressFollowingClick,
  restoreDetailFromParkedSourceWithDelay,
  clearMorphingHeightUnlockTimer,
  captureVisibleSourceReturnTarget,
  setTopChromeVisible,
  restoreMorphingItemContent,
  scheduleHiddenSourceReaderCleanup,
})
const restoreDetailFromParkedSource = readerParkedDetailRestoreAction.restoreDetailFromParkedSource

const readerSourceCloseAction = useReaderSourceCloseAction({
  sourceReaderOpen,
  detailReaderOpen,
  isFeedRoute,
  sourceReaderShouldReturnToDetail,
  hasDetailParkedBehindSource,
  restorePreviousParkedDetailIfReaderClosed,
  restoreSourceReaderBackTargetState,
  closeVisibleSourceReaderState,
  clearSourceReaderState,
  resetSourceSubscriptionState,
  rememberDetailScrollTop,
  restoreDetailFromParkedSource,
  setTopChromeVisible,
  scheduleHiddenSourceReaderCleanup,
})
const restoreSourceReaderBackTarget = readerSourceCloseAction.restoreSourceReaderBackTarget
const closeSourceReader = readerSourceCloseAction.closeSourceReader

const readerItemCloseAction = useReaderItemCloseAction({
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
})
const finishCommittedListReturnForGesture = readerItemCloseAction.finishCommittedListReturnForGesture
const closeItemReader = readerItemCloseAction.closeItemReader
const collapseItemReader = readerItemCloseAction.collapseItemReader

const readerBackSwipeResetAction = useReaderBackSwipeResetAction({
  readerBackDragging,
  stretchAnchorClearDuration: motionStretchAnchorClearDuration,
  resetReaderBackSwipeDragState,
  resetPageSideMotion: pageContentMotion.resetSideMotion,
  clearReaderStretchAnchorsIfIdle,
  clearPageStretchAnchorIfIdle: pageContentMotion.clearStretchAnchorIfIdle,
})
const resetBackSwipeOffset = readerBackSwipeResetAction.resetBackSwipeOffset

const readerRestoreActions = useReaderRestoreActions({
  normalDuration: motionNormalDuration,
  readerDuration: motionReaderDuration,
  resolveDelay: motionDelay,
  restoreParkedSourceReaderWithDelay,
  restoreItemReaderExpansionWithDelay,
  restoreDetailFromSourceSwipeWithDelay,
  completeDetailToSourceReaderWithDelay,
  resetBackSwipeOffset,
  setTopChromeVisible,
  captureDetailSourceTransitionRects,
  restoreMorphingItemContent,
})
const restoreParkedSourceReader = readerRestoreActions.restoreParkedSourceReader
const restoreItemReaderExpansion = readerRestoreActions.restoreItemReaderExpansion
const restoreDetailFromSourceSwipe = readerRestoreActions.restoreDetailFromSourceSwipe
const completeDetailToSourceReader = readerRestoreActions.completeDetailToSourceReader

const appGestureStartGuards = useAppGestureStartGuards({
  isFeedRoute,
  navigationVisible,
  sourceReaderOpen,
  isSubscriptionsRoute: () => route.name === 'subscriptions',
  detailBlocksGestures,
})
const canStartViewSwipe = appGestureStartGuards.canStartViewSwipe
const canStartNavigationOpen = appGestureStartGuards.canStartNavigationOpen

const feedViewSwipeController = useFeedViewSwipeController({
  topChromeProgress,
  feedContentCollapsed,
  viewSwipeChromeRevealDelay,
  motionNormalDuration,
  resolveDelay: motionDelay,
  beginSwipeTransition: swipeTransition.begin,
  updateSwipeTransition: swipeTransition.update,
  settleSwipeTransition: swipeTransition.settle,
  scheduleSwipeReset: swipeTransition.scheduleReset,
  swipeTransitionBeginPayload: feedPagerTransition.swipeTransitionBeginPayload,
  swipeTransitionUpdatePayload: feedPagerTransition.swipeTransitionUpdatePayload,
  finishSwipeResult: feedPagerTransition.finishSwipeResult,
  settleFinishedSwipe: feedPagerTransition.settleFinishedSwipe,
  scheduleDelayedCommit: feedPagerTransition.scheduleDelayedCommit,
  markStartedWithHiddenChrome: feedPagerTransition.markStartedWithHiddenChrome,
  setTopChromeVisible,
  pushRoute,
})
const scheduleSwipeTransitionReset = feedViewSwipeController.scheduleSwipeTransitionReset
const beginViewSwipeTransition = feedViewSwipeController.beginViewSwipeTransition
const syncViewSwipeTransition = feedViewSwipeController.syncViewSwipeTransition

const readerBackSwipeTransitionController = useReaderBackSwipeTransitionController({
  activeFeedSurface: feedPagerTransition.activeSurface,
  pageReturnSurface: 'feed:recommendations',
  fallbackStretch: pageSideStretch,
  beginSwipeTransition: swipeTransition.begin,
  updateSwipeTransition: swipeTransition.update,
  transitionBeginPayload: readerBackSwipeTransitionBeginPayload,
  transitionUpdatePayload: readerBackSwipeTransitionUpdatePayload,
})
const beginBackSwipeTransition = readerBackSwipeTransitionController.beginBackSwipeTransition
const syncBackSwipeTransition = readerBackSwipeTransitionController.syncBackSwipeTransition

const readerBackSwipeDragHandlers = useReaderBackSwipeDragHandlers({
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
  beginBackSwipeTransition,
  syncBackSwipeTransition,
  cancelNavigationCandidates: navigationGesture.cancelCandidates,
  cancelViewSwipeCandidate: feedPagerTransition.cancelViewSwipeCandidate,
  isBackHorizontalSwipe,
  suppressFollowingClick,
  beginTopChromeGestureReturn: chromeState.beginGestureReturn,
  refreshDetailFeedOriginRect,
  captureDetailSourceTransitionRects,
  showSourceReaderUnderDetail,
  setPageSideStretch: pageContentMotion.setSideStretch,
  setPageSideOffset: pageContentMotion.setSideOffset,
})
const beginDetailGestureCandidate = readerBackSwipeDragHandlers.beginDetailGestureCandidate
const updateBackSwipe = readerBackSwipeDragHandlers.updateBackSwipe

const readerBackSwipeCompletion = useReaderBackSwipeCompletion({
  switchDistance: viewSwitchDistance,
  getFallbackStretch: () => pageSideStretch.value,
  finishResult: readerBackSwipeFinishResult,
  cancelResult: readerBackSwipeCancelResult,
  settleTransition: swipeTransition.settle,
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
  reset: resetBackSwipeOffset,
})
const finishBackSwipe = readerBackSwipeCompletion.finishBackSwipe
const cancelBackSwipe = readerBackSwipeCompletion.cancelBackSwipe

const finishViewSwipe = feedViewSwipeController.finishViewSwipe
const showTopChromeForViewSwipe = feedViewSwipeController.showTopChromeForViewSwipe

const appTouchGestureHandlers = useAppTouchGestureHandlers({
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
})
const handleTouchStart = appTouchGestureHandlers.handleTouchStart
const handleTouchMove = appTouchGestureHandlers.handleTouchMove

const navigationPointerHandlers = useNavigationPointerHandlers({
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
})
const handleWindowPointerDown = navigationPointerHandlers.handleWindowPointerDown
const handleWindowPointerMove = navigationPointerHandlers.handleWindowPointerMove
const handleWindowPointerUp = navigationPointerHandlers.handleWindowPointerUp
const handleWindowPointerCancel = navigationPointerHandlers.handleWindowPointerCancel

const handleTouchEnd = appTouchGestureHandlers.handleTouchEnd

const feedPointerSwipeHandlers = useFeedPointerSwipeHandlers({
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
})
const handleFeedPointerDown = feedPointerSwipeHandlers.handleFeedPointerDown
const handleFeedPointerMove = feedPointerSwipeHandlers.handleFeedPointerMove
const handleFeedPointerUp = feedPointerSwipeHandlers.handleFeedPointerUp
const handleFeedPointerCancel = feedPointerSwipeHandlers.handleFeedPointerCancel

const handleTouchCancel = appTouchGestureHandlers.handleTouchCancel

const readerDetailProgressHandlers = useReaderDetailProgressHandlers({
  detailContentRef,
  detailScrollMax,
  detailScrollTop,
  updateDetailScrollMetrics: updateDetailScrollMetricsState,
  updateDetailScrollTop: updateDetailScrollTopState,
  rememberDetailScrollTop,
  scrollDetailContentElementTo,
  suppressFollowingClick,
  setDetailProgressDragging: setDetailProgressDraggingState,
})
const syncDetailContainerMetrics = readerDetailProgressHandlers.syncDetailContainerMetrics
const handleDetailProgressChange = readerDetailProgressHandlers.handleDetailProgressChange
const handleDetailProgressDragStart = readerDetailProgressHandlers.handleDetailProgressDragStart
const handleDetailProgressDragEnd = readerDetailProgressHandlers.handleDetailProgressDragEnd
const handleDetailFrameLoad = readerDetailProgressHandlers.handleDetailFrameLoad

const readerDetailMessageHandler = useReaderDetailMessageHandler({
  detailReaderOpen,
  navigationVisible,
  readerBackSwipeTrackingActive,
  detailCommittedListReturn,
  updateDetailFrameContentHeight: updateDetailFrameContentHeightState,
  syncDetailContainerMetrics,
  detailFrameViewportOffset,
  beginDetailGestureCandidate,
  updateBackSwipe,
  finishBackSwipe,
  cancelBackSwipe,
  resetGestureTracking,
})
const handleMessage = readerDetailMessageHandler.handleMessage

const readerSettingsSync = useReaderSettingsSync({
  setSourceTimelinePreloadEnabled: setSourceTimelinePreloadEnabledState,
})
const loadReaderSettings = readerSettingsSync.loadReaderSettings
const handleReaderSettingsChanged = readerSettingsSync.handleReaderSettingsChanged

const feedTopPullHandlers = useFeedTopPullHandlers({
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
})
const handleFeedTopPullStart = feedTopPullHandlers.handleFeedTopPullStart
const handleFeedTopPullMove = feedTopPullHandlers.handleFeedTopPullMove
const handleFeedTopPullEnd = feedTopPullHandlers.handleFeedTopPullEnd

const topChromeScrollBehavior = useTopChromeScrollBehavior({
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
})
const updateTopTabsByScroll = topChromeScrollBehavior.updateByScroll

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
  sourceTitleRevealVisible,
  sourceTitleRevealStyle,
  sourceToggleActive,
  detailItem,
  sourceNameMorphVisible,
  sourceNameMorphStyle,
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
  detailMorphTextVisible,
  detailMorphTextStyle,
  detailMorphSourceLabelStyle,
  detailDisplayDate,
  detailMorphSummaryVisible,
  detailPreviewSummary,
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

const pagePullRefreshAction = usePagePullRefreshAction({
  refreshing: pagePullRefreshing,
  noticeDelayMS: motionQuickDuration,
  currentRefreshPage: pageOutlet.currentRefreshPage,
  beginRefreshing: pagePullRefresh.beginRefreshing,
  finishRefreshing: pagePullRefresh.finishRefreshing,
  collapseTopChrome,
})
const refreshCurrentPageFromPull = pagePullRefreshAction.refreshCurrentPageFromPull

const pagePullGestureHandlers = usePagePullGestureHandlers({
  isFeedRoute,
  refreshThreshold: pagePullRefresh.threshold,
  pullRefresh: pagePullRefresh,
  currentContentScrollTop,
  isControlTarget: isPageTopPullControlTarget,
  shouldCancelTopPull,
  shouldWaitForTopPull,
  setTopChromeVisible,
  finishFeedTopPull: () => {
    feedTopPull.finish()
  },
  settlePullOffset: settlePagePullOffset,
  refreshCurrentPageFromPull,
})
const resetPageTopPullTracking = pagePullGestureHandlers.resetPageTopPullTracking
const handlePageTouchStart = pagePullGestureHandlers.handlePageTouchStart
const handlePageTouchMove = pagePullGestureHandlers.handlePageTouchMove
const handlePageTouchEnd = pagePullGestureHandlers.handlePageTouchEnd
const handlePageTouchCancel = pagePullGestureHandlers.handlePageTouchCancel

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
  resetPagePullMotion: pagePullRefresh.resetMotion,
  resetFeedViewDragOffset: feedPagerTransition.resetDragOffset,
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
    sourceContentMotion.settleAfterRefresh(topChromeSettleDuration)
  },
  collapseTopChrome,
})

useAppLifecycle({
  loadReaderSettings,
  loadTheme: () => themeState.load(),
  installVirtualBackGuard: () => virtualBackGuard.installRouterGuard(),
  uninstallVirtualBackGuard: () => virtualBackGuard.uninstallRouterGuard(),
  waitForRouterReady: () => router.isReady(),
  restoreReaderSession,
  markReaderSessionReady: () => routeRuntime.markReaderSessionReady(),
  scheduleReaderURLAndHistorySync,
  installWindowEventListeners,
  uninstallWindowEventListeners,
  saveReaderSessionNow,
  clearRuntimeTimers: [
    () => feedPagerTransition.clearTimers(),
    () => swipeTransition.clearTimer(),
    () => navigationDrawer.clearTimer(),
    () => refreshCompletion.clearTimer(),
    () => chromeState.clearTimer(),
    () => sourceContentMotion.clearTimer(),
    () => pagePullRefresh.clearTimers(),
    () => clickSuppression.clearTimer(),
    clearSourceNoticeTimer,
    clearReaderStackTimers,
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

    <AppMainOutlet
      :main-class="mainClass"
      :main-style="mainStyle"
      :swipe-phase="swipePhase"
      :swipe-direction="swipeDirection"
      :swipe-progress="swipeProgress"
      :swipe-is-blocked="swipeIsBlocked"
      :top-chrome-phase="topChromePhase"
      :feed-header-progress="feedHeaderProgress"
      :header-class="headerClass"
      :header-style="headerStyle"
      :feed-header-active="isFeedRoute || detailChromeVisible"
      :detail-reader-open="detailReaderOpen"
      :detail-header-visible="detailHeaderVisible"
      :detail-header-layer-style="detailHeaderLayerStyle"
      :detail-title="detailItem?.title"
      :detail-header-title-style="detailHeaderTitleStyle"
      :detail-header-previous-title="detailHeaderPreviousTitle"
      :detail-header-previous-text-style="detailHeaderPreviousTextStyle"
      :detail-header-current-text-style="detailHeaderCurrentTextStyle"
      :is-feed-route="isFeedRoute"
      :feed-tabs="feedTabs"
      :active-key="route.name"
      :feed-tabs-layer-hidden="feedTabsLayerHidden"
      :feed-tabs-layer-style="feedTabsLayerStyle"
      :view-swipe-target-visible="viewSwipeTargetVisible"
      :feed-tabs-target-layer-style="feedTabsTargetLayerStyle"
      :view-swipe-target-key="viewSwipeTargetKey"
      :feed-pull-active="feedPullActive"
      :feed-pull-refreshing="feedInteraction.pullRefreshing"
      :pull-status-style="pullStatusStyle"
      :pull-icon-style="pullIconStyle"
      :pull-status-text="pullStatusText"
      :pull-status-meta="pullStatusMeta"
      :page-title="pageTitle"
      :page-pull-active="pagePullActive"
      :page-title-layer-style="pageTitleLayerStyle"
      :page-pull-status-style="pagePullStatusStyle"
      :page-pull-refreshing="pagePullRefreshing"
      :page-pull-icon-style="pagePullIconStyle"
      :page-pull-status-text="pagePullStatusText"
      :page-pull-status-meta="pagePullStatusMeta"
      :source-reader-open="sourceReaderOpen"
      :view-settling="viewSettling"
      :feed-track-style="feedTrackStyle"
      :feed-scroll-top="feedScrollTop"
      :top-chrome-progress="topChromeProgress"
      :feed-header-height="feedHeaderHeight"
      :freeze-body-during-top-refresh="freezeFeedBodyDuringTopRefresh"
      :morphing-item-id="morphingItemId"
      :morphing-height-lock-item-id="morphingHeightLockItemId"
      :morphing-item-height="morphingItemHeight"
      :feed-item-preview-progress="feedItemPreviewProgress"
      :page-content-inner-style="pageContentInnerStyle"
      @navigate="navigateTo"
      @feed-content-ref="setFeedContentElement"
      @feed-content-scroll="handleFeedContentScroll"
      @feed-pointer-down="handleFeedPointerDown"
      @feed-pointer-move="handleFeedPointerMove"
      @feed-pointer-up="handleFeedPointerUp"
      @feed-pointer-cancel="handleFeedPointerCancel"
      @feed-top-pull-start="handleFeedTopPullStart"
      @feed-top-pull-move="handleFeedTopPullMove"
      @feed-top-pull-end="handleFeedTopPullEnd"
      @open-item="openItemReader"
      @page-content-ref="setPageContentElement"
      @page-view-ref="setPageViewInstance"
      @page-content-scroll="handlePageContentScroll"
      @page-touch-start="handlePageTouchStart"
      @page-touch-move="handlePageTouchMove"
      @page-touch-end="handlePageTouchEnd"
      @page-touch-cancel="handlePageTouchCancel"
      @open-source="openSourceReader"
    />

    <AppReaderStackOutlet v-bind="readerStackOutletProps" v-on="readerStackOutletListeners" />
  </div>
</template>
