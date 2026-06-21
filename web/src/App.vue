<script setup lang="ts">
import { nextTick, onMounted, onUnmounted, watch, type ComponentPublicInstance } from 'vue'
import { storeToRefs } from 'pinia'
import { useRoute, useRouter } from 'vue-router'

import { useFeedInteractionStore } from '@/stores/feedInteraction'
import {
  getFeedItem,
  type FeedItem,
} from '@/api/feed'
import AppFeedHeaderContent from '@/components/AppFeedHeaderContent.vue'
import AppNavigationLayer from '@/components/AppNavigationLayer.vue'
import AppPageHeaderContent from '@/components/AppPageHeaderContent.vue'
import AppPageOutlet from '@/components/AppPageOutlet.vue'
import AppReaderStackContent from '@/components/AppReaderStackContent.vue'
import FeedPager from '@/components/FeedPager.vue'
import TopChrome from '@/components/TopChrome.vue'
import { useChromeState } from '@/composables/useChromeState'
import { useReaderSourceSubscription } from '@/composables/useReaderSourceSubscription'
import { useReaderBackSwipeCompletion } from '@/composables/useReaderBackSwipeCompletion'
import { usePullRefresh } from '@/composables/usePullRefresh'
import {
  type FeedSourceKind,
  type ParkedDetailSnapshot,
  type ReaderSessionSnapshot,
  type ReaderSource,
  type RectSnapshot,
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
import { useReaderDetailProgressMotion } from '@/composables/useReaderDetailProgressMotion'
import { useReaderDetailTextMotion } from '@/composables/useReaderDetailTextMotion'
import { useReaderSourceTitleMotion } from '@/composables/useReaderSourceTitleMotion'
import { useReaderDetailTransitionMotion } from '@/composables/useReaderDetailTransitionMotion'
import { useAppShellMotion } from '@/composables/useAppShellMotion'
import { useTopPullState } from '@/composables/useTopPullState'
import { useViewportSize } from '@/composables/useViewportSize'
import { useThemeState } from '@/composables/useThemeState'
import { useFeedScrollState } from '@/composables/useFeedScrollState'
import { usePageOutletState, type PageViewExpose } from '@/composables/usePageOutletState'
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
import { useAppNavigationActions } from '@/composables/useAppNavigationActions'
import { useAppNavigationConfig } from '@/composables/useAppNavigationConfig'
import { usePullActivityState } from '@/composables/usePullActivityState'
import { useFeedChromeLayoutState } from '@/composables/useFeedChromeLayoutState'
import { useFeedChromeVisibilityState } from '@/composables/useFeedChromeVisibilityState'
import { snapshotElementRect, snapshotRect } from '@/utils/domSnapshot'

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
  restoreParkedDetailSnapshot: restoreReaderStackParkedDetailSnapshot,
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
  settleReaderMotionWithDelay,
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
const feedTopPullStartProgress = feedTopPull.startProgress
const pagePullRefresh = usePullRefresh({ threshold: 52 })
const pageRefreshThreshold = pagePullRefresh.threshold
const pagePullOffset = pagePullRefresh.offset
const pagePullDistance = pagePullRefresh.distance
const pagePullSettling = pagePullRefresh.settling
const pagePullRefreshing = pagePullRefresh.refreshing
const pageTopPullDistance = pagePullRefresh.gestureDistance
const trackingPageTopPullCandidate = pagePullRefresh.gestureCandidate
const trackingPageTopPull = pagePullRefresh.gestureTracking
const pageContentMotion = usePageContentMotion({ pullOffset: pagePullOffset })
const pageSideStretch = pageContentMotion.sideStretch
const pageContentInnerStyle = pageContentMotion.contentStyle
const homeExitDoubleBackMs = 1600
const homeBackGuard = useDoubleBackGuard(homeExitDoubleBackMs)
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

const appRouteState = useAppRouteState(route)
const selectedKeys = appRouteState.selectedKeys
const pageTitle = appRouteState.pageTitle
const isFeedRoute = appRouteState.isFeedRoute
const cornerButtonLabel = appRouteState.cornerButtonLabel
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

function resetGestureTracking() {
  navigationGesture.reset()
  feedPagerTransition.resetViewSwipeTracking()
  feedPagerTransition.clearStartedWithHiddenChrome()
  resetReaderBackSwipeCandidateState()
}

function isInteractiveTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select, [role="button"]'))
}

function isPageTopPullControlTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select'))
}

function handleClickCapture(event: MouseEvent) {
  clickSuppression.consume(event)
}

function suppressFollowingClick() {
  clickSuppression.suppressNext()
}

function scheduleReaderURLAndHistorySync(forcePush = false) {
  readerRouteSync.scheduleSync(forcePush)
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(Math.max(value, min), max)
}

function clearStretchAnchors(delay = motionStretchAnchorClearDuration) {
  window.setTimeout(() => {
    clearReaderStretchAnchorsIfIdle()
    pageContentMotion.clearStretchAnchorIfIdle(readerBackDragging.value)
  }, delay)
}

function setSourceReaderContentElement(element: HTMLElement | null) {
  setSourceReaderContentElementState(element)
}

function setFeedContentElement(element: Element | ComponentPublicInstance | null) {
  feedContent.setContentElement(element instanceof HTMLElement ? element : null)
}

function setPageContentElement(element: HTMLElement | null) {
  pageOutlet.setContentElement(element)
}

function setPageViewInstance(view: PageViewExpose | null) {
  pageOutlet.setViewInstance(view)
}

function rememberSourceScrollTop(scrollTop: number) {
  scrollHistory.set('source', scrollTop)
}

function rememberDetailScrollTop(scrollTop: number) {
  scrollHistory.set('detail', scrollTop)
}

function findSourceFeedItemElement(itemID?: number) {
  if (!itemID || !sourceReaderContentRef.value) {
    return null
  }
  return sourceReaderContentRef.value.querySelector(`[data-feed-item-id="${itemID}"]`)
}

function findFeedItemElement(itemID?: number) {
  return feedContent.findItemElement(itemID, activeFeedIndex.value)
}

function refreshDetailFeedOriginRect(lock = false) {
  if (detailFeedOriginLocked.value) {
    return
  }

  const itemRect = snapshotElementRect(findFeedItemElement(detailItem.value?.id))
  if (!itemRect) {
    return
  }

  applyDetailFeedOriginRectState(itemRect, lock)
}

function findSourceFeedItemSourceElement(itemID?: number) {
  const itemElement = findSourceFeedItemElement(itemID)
  return itemElement?.querySelector('.feed-item__source') ?? null
}

function sourceNameTargetFallback(itemRect: RectSnapshot | null) {
  if (itemRect) {
    const left = itemRect.left + 16
    const top = itemRect.top + 16
    return {
      left,
      top,
      width: Math.max(1, Math.min(itemRect.width - 32, 180)),
      height: 18,
    }
  }

  return null
}

function captureDetailSourceTransitionRects(
  retry = 12,
  options: { force?: boolean; lock?: boolean } = {},
) {
  if (detailTransitionRectsLocked.value && !options.force) {
    return
  }

  nextTick(() => {
    requestAnimationFrame(() => {
      if (detailTransitionRectsLocked.value && !options.force) {
        return
      }

      const itemRect = snapshotElementRect(findSourceFeedItemElement(detailItem.value?.id))
      const sourceOriginRect = snapshotElementRect(detailInlineSourceRef.value)
      const sourceTargetRect =
        snapshotElementRect(findSourceFeedItemSourceElement(detailItem.value?.id)) ?? sourceNameTargetFallback(itemRect)

      const result = applyDetailSourceTransitionRectsState({
        itemRect,
        sourceOriginRect,
        sourceTargetRect,
        lock: options.lock,
      })
      if (result.locked) {
        return
      }

      if (retry > 0 && (!itemRect || !sourceOriginRect || !sourceTargetRect)) {
        window.setTimeout(() => captureDetailSourceTransitionRects(retry - 1, options), readerRectRetryDelay)
      }
    })
  })
}

function captureVisibleSourceReturnTarget() {
  const itemRect = snapshotElementRect(findSourceFeedItemElement(detailItem.value?.id))
  const sourceTargetRect =
    snapshotElementRect(findSourceFeedItemSourceElement(detailItem.value?.id)) ?? sourceNameTargetFallback(itemRect)
  const sourceOriginRect = snapshotElementRect(detailInlineSourceRef.value)

  return applyVisibleSourceReturnTargetState(itemRect, sourceOriginRect, sourceTargetRect)
}

function detailFrameViewportOffset() {
  const rect = detailFrameRef.value?.getBoundingClientRect()
  return {
    left: rect?.left ?? 0,
    top: rect?.top ?? 0,
  }
}

function setDetailContentElement(element: HTMLElement | null) {
  setDetailContentElementState(element)
}

function setDetailInlineSourceElement(element: HTMLElement | null) {
  setDetailInlineSourceElementState(element)
}

function setDetailFrameElement(element: HTMLIFrameElement | null) {
  setDetailFrameElementState(element)
}

function restoreMorphingItemContent(unlockDelay = motionQuickDuration) {
  restoreMorphingItemContentWithDelay(unlockDelay)
}

function scheduleHiddenSourceReaderCleanup(delay = motionQuickDuration) {
  scheduleHiddenSourceReaderCleanupWithDelay(delay)
}

function showSourceReaderUnderDetail() {
  if (!detailItem.value?.source_id) {
    return
  }

  const source = {
    id: detailItem.value.source_id,
    name: detailItem.value.source_name || '未知来源',
    kind: detailSourceKind.value,
  }

  if (readerSource.value?.id !== source.id || readerSource.value.kind !== source.kind) {
    openSourceReader(source, { visible: false })
  }

  setTopChromeVisible(true)
  revealSourceReaderUnderDetailState()
  captureDetailSourceTransitionRects(12, { lock: true })
}

function settleReaderMotion(duration = motionNormalDuration, done?: () => void) {
  settleReaderMotionWithDelay(motionDelay(duration), done)
}

function readerSessionSnapshot(): ReaderSessionSnapshot {
  return {
    savedAt: Date.now(),
    routeFullPath: route.fullPath,
    feedScrollTop: feedScrollTop.value,
    topChromeProgress: topChromeProgress.value,
    feedContentCollapsed: feedContentCollapsed.value,
    ...createReaderStackSessionSnapshot(),
  }
}

function saveReaderSessionNow() {
  if (!routeRuntime.canSaveReaderSession(readerSession.restoring.value)) {
    return
  }
  readerSession.saveNow()
}

function scheduleReaderSessionSave() {
  if (!routeRuntime.canSaveReaderSession(readerSession.restoring.value)) {
    return
  }
  readerSession.scheduleSave()
}

function restoreSavedScrollPositions(snapshot: ReaderSessionSnapshot) {
  const apply = () => {
    feedContent.scrollTo(snapshot.feedScrollTop)
    scrollSourceReaderContentElementTo(snapshot.sourceReaderScrollTop)
    if (scrollDetailContentElementTo(snapshot.detailScrollTop)) {
      syncDetailContainerMetrics()
    }
  }

  nextTick(() => {
    apply()
    window.setTimeout(apply, readerScrollRestoreRetryDelay)
    window.setTimeout(apply, readerScrollRestoreSettledDelay)
  })
}

function applyReaderSessionSnapshot(snapshot: ReaderSessionSnapshot) {
  feedScroll.restore(snapshot.feedScrollTop)
  chromeState.restoreSnapshot({
    progress: snapshot.topChromeProgress,
    contentCollapsed: snapshot.feedContentCollapsed,
  })
  applyReaderStackSessionSnapshot(snapshot, {
    onSourceScrollTop: rememberSourceScrollTop,
    onDetailScrollTop: rememberDetailScrollTop,
    onReaderSourceRestored: (source) => {
      void loadSourceReaderSubscription(source)
    },
  })
  restoreSavedScrollPositions(snapshot)
}

async function restoreReaderSession() {
  await readerSession.restore()
}

function hasVirtualBackTarget() {
  return (
    navigationVisible.value ||
    hasParkedDetailSourceState() ||
    sourceReaderOpen.value ||
    detailReaderOpen.value ||
    (!isFeedRoute.value && !navigationVisible.value)
  )
}

function runVirtualBackAnimation() {
  if (navigationVisible.value) {
    homeBackGuard.reset()
    closeNavigation()
    return true
  }

  if (detailReaderOpen.value && detailOpenedFromSourceReader.value && !detailCommittedListReturn()) {
    homeBackGuard.reset()
    collapseItemReader()
    return true
  }

  if (sourceReaderShouldReturnToDetail()) {
    homeBackGuard.reset()
    restoreSourceReaderBackTarget()
    return true
  }

  if (sourceReaderOpen.value && !detailReaderOpen.value) {
    homeBackGuard.reset()
    closeSourceReader()
    return true
  }

  if (detailReaderOpen.value) {
    homeBackGuard.reset()
    collapseItemReader()
    return true
  }

  if (!isFeedRoute.value && !navigationVisible.value) {
    homeBackGuard.reset()
    goHome(false)
    return true
  }

  if (isFeedRoute.value) {
    return homeBackGuard.shouldConsumeBack()
  }

  return false
}

function restoreSourceReaderBackTarget() {
  const result = restoreSourceReaderBackTargetState({
    onDetailScrollTop: rememberDetailScrollTop,
  })
  if (result.action === 'restore-detail') {
    restoreDetailFromParkedSource()
    return
  }

  closeSourceReader()
}

function restoreParkedDetailSnapshot(snapshot: ParkedDetailSnapshot | null) {
  return restoreReaderStackParkedDetailSnapshot(snapshot, {
    onDetailScrollTop: rememberDetailScrollTop,
  })
}

function restorePreviousParkedDetail() {
  return restoreReaderStackPreviousParkedDetail({
    onDetailScrollTop: rememberDetailScrollTop,
  })
}

function finishCommittedListReturnForGesture() {
  if (!detailCommittedListReturn()) {
    return
  }
  if (hasDetailParkedBehindSource()) {
    return
  }

  clearDetailEntryTimer()
  closeItemReader()
}

function openSourceReader(source: ReaderSource, options: { visible?: boolean } = {}) {
  clearHiddenSourceCleanupTimer()
  const nextVisible = options.visible ?? true
  if (nextVisible) {
    setTopChromeVisible(true)
  }

  const result = openSourceReaderState(source, { visible: nextVisible })
  if (!result.sourceChanged) {
    if (result.captureTransition) {
      captureDetailSourceTransitionRects(12, { lock: true })
    }
    if (result.loadSubscription) {
      void loadSourceReaderSubscription(source)
    }
    return
  }

  resetSourceSubscriptionState()
  if (result.resetScroll) {
    rememberSourceScrollTop(0)
  }
  nextTick(() => {
    if (result.resetScroll) {
      scrollSourceReaderContentElementTo(0)
    }
    if (result.captureTransition) {
      captureDetailSourceTransitionRects(12, { lock: true })
    }
  })
  if (result.loadSubscription) {
    void loadSourceReaderSubscription(source)
  }
}

async function openItemReader(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
  const openedFromSourceReader =
    sourceReaderOpen.value && readerSource.value?.id === item.source_id && readerSource.value.kind === sourceKind
  openItemReaderWithTransition(item, sourceKind, {
    openedFromSourceReader,
    originRect: snapshotRect(originRect),
    headerSwapDelay: motionDelay(motionHeaderSwapDuration),
    detailEntryDelay: motionDelay(readerMorphDuration),
    afterBegin: () => {
      chromeState.setStableVisible()
      feedTopPull.finish()
      rememberDetailScrollTop(0)
    },
    afterEntry: () => {
      if (openedFromSourceReader) {
        captureDetailSourceTransitionRects(12, { lock: true })
      }
    },
  })
  if (!openedFromSourceReader && sourceTimelinePreloadEnabled.value && item.source_id) {
    openSourceReader(
      {
        id: item.source_id,
        name: item.source_name || '未知来源',
        kind: sourceKind,
      },
      { visible: false },
    )
  }
  try {
    let loadedItem: FeedItem | undefined
    if (sourceKind === 'subscriptions' && item.id > 0) {
      loadedItem = await getFeedItem(item.id)
    }
    finishOpenItemReaderLoad({ item: loadedItem })
  } catch {
    finishOpenItemReaderLoad({ errorMessage: '无法加载完整条目，已显示当前列表内容。' })
  } finally {
    nextTick(() => {
      scrollDetailContentElementTo(0)
      scheduleReaderSessionSave()
    })
  }
}

function closeSourceReader() {
  if (sourceReaderShouldReturnToDetail()) {
    restoreSourceReaderBackTarget()
    return
  }

  if (hasDetailParkedBehindSource()) {
    restoreDetailFromParkedSource()
    return
  }

  if (
    restorePreviousParkedDetailIfReaderClosed({
      onDetailScrollTop: rememberDetailScrollTop,
    })
  ) {
    restoreDetailFromParkedSource()
    return
  }

  if (sourceReaderOpen.value) {
    closeVisibleSourceReaderState()
    if (isFeedRoute.value && !detailReaderOpen.value) {
      setTopChromeVisible(true)
    }
    scheduleHiddenSourceReaderCleanup(340)
    return
  }

  clearSourceReaderState()
  resetSourceSubscriptionState()
  if (isFeedRoute.value && !detailReaderOpen.value) {
    setTopChromeVisible(true)
  }
}

function restoreDetailFromParkedSource(duration = motionReaderDuration) {
  if (!detailReaderOpen.value) {
    closeSourceReader()
    return
  }

  suppressFollowingClick()
  restoreDetailFromParkedSourceWithDelay(motionDelay(duration), {
    beforeBegin: () => {
      clearMorphingHeightUnlockTimer()
      captureVisibleSourceReturnTarget()
    },
    afterBegin: () => {
      setTopChromeVisible(true)
    },
    afterFinish: () => {
      restoreMorphingItemContent()
      scheduleHiddenSourceReaderCleanup()
    },
  })
}

function restoreParkedSourceReader(duration = motionNormalDuration) {
  if (!restoreParkedSourceReaderWithDelay(motionDelay(duration))) {
    resetBackSwipeOffset()
  }
}

function closeItemReader() {
  const result = closeItemReaderWithTransition()
  if (isFeedRoute.value) {
    setTopChromeVisible(true)
  }
  if (result.shouldScheduleHiddenSourceCleanup) {
    scheduleHiddenSourceReaderCleanup()
  }
}

function collapseItemReader(duration = motionReaderDuration) {
  if (!detailReaderOpen.value) {
    return
  }

  suppressFollowingClick()
  collapseItemReaderWithDelay(motionDelay(duration), {
    afterBegin: (result) => {
      if (result.shouldRefreshFeedOrigin) {
        refreshDetailFeedOriginRect(true)
      }
    },
    afterFinish: (result) => {
      if (result.shouldRestorePreviousParkedDetail && restorePreviousParkedDetail()) {
        scheduleReaderURLAndHistorySync(true)
        return
      }
      closeItemReader()
      scheduleReaderURLAndHistorySync(true)
    },
  })
}

function restoreItemReaderExpansion(duration = motionReaderDuration) {
  restoreItemReaderExpansionWithDelay(motionDelay(duration))
}

function restoreDetailFromSourceSwipe(duration = motionReaderDuration) {
  restoreDetailFromSourceSwipeWithDelay(motionDelay(duration))
}

function completeDetailToSourceReader(duration = motionReaderDuration) {
  completeDetailToSourceReaderWithDelay(motionDelay(duration), {
    afterBegin: () => {
      setTopChromeVisible(true)
      captureDetailSourceTransitionRects(12, { lock: true })
    },
    afterFinish: () => {
      restoreMorphingItemContent()
    },
  })
}

function canStartViewSwipe(_clientX: number) {
  if (!isFeedRoute.value || navigationVisible.value || sourceReaderOpen.value || detailBlocksGestures()) {
    return false
  }

  return true
}

function canStartNavigationOpen(_clientX: number) {
  return (
    route.name === 'subscriptions' &&
    !navigationVisible.value &&
    !sourceReaderOpen.value &&
    !detailBlocksGestures()
  )
}

function scheduleSwipeTransitionReset(duration = motionNormalDuration) {
  swipeTransition.scheduleReset(motionDelay(duration))
}

function beginViewSwipeTransition(offset: number) {
  swipeTransition.begin(feedPagerTransition.swipeTransitionBeginPayload(offset))
}

function syncViewSwipeTransition(offset: number) {
  swipeTransition.update(feedPagerTransition.swipeTransitionUpdatePayload(offset))
}

function beginBackSwipeTransition(deltaX: number) {
  const payload = readerBackSwipeTransitionBeginPayload(deltaX, {
    activeFeedSurface: feedPagerTransition.activeSurface.value,
    pageReturnSurface: 'feed:recommendations',
  })
  swipeTransition.begin(payload)
}

function syncBackSwipeTransition(deltaX: number) {
  const payload = readerBackSwipeTransitionUpdatePayload(deltaX, pageSideStretch.value, {
    activeFeedSurface: feedPagerTransition.activeSurface.value,
    pageReturnSurface: 'feed:recommendations',
  })
  swipeTransition.update(payload)
}

function showTopChromeForSourceReturn() {
  if (topChromeProgress.value < 0.99 || feedContentCollapsed.value) {
    chromeState.beginGestureReturn({ settleDelayMS: motionDelay(topChromeSettleDuration) })
  }
}

function settleSourceContentAfterRefresh() {
  sourceContentMotion.settleAfterRefresh(topChromeSettleDuration)
}

function prepareSourceReaderReturnDrag() {
  const ready = prepareSourceReaderReturnDragState({
    onDetailScrollTop: rememberDetailScrollTop,
  })
  if (!ready) {
    return false
  }

  return captureVisibleSourceReturnTarget()
}

function resetBackSwipeOffset() {
  resetReaderBackSwipeDragState()
  pageContentMotion.resetSideMotion()
  clearStretchAnchors()
}

function prepareDetailSourceReaderPreload() {
  if (!detailItem.value?.source_id || readerSource.value) {
    return
  }

  openSourceReader(
    {
      id: detailItem.value.source_id,
      name: detailItem.value.source_name || '未知来源',
      kind: detailSourceKind.value,
    },
    { visible: false },
  )
}

function beginDetailGestureCandidate(startX: number, startY: number) {
  resetGestureTracking()
  gestureOrigin.begin(startX, startY, navigationProgress.value)
  beginReaderBackSwipeCandidateState('detail')
  if (sourceTimelinePreloadEnabled.value) {
    prepareDetailSourceReaderPreload()
  }
}

function isDetailFrameHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > 3 && Math.abs(deltaX) > Math.abs(deltaY) * 0.52
}

function readerBackSwipeIntentOptions(
  options: { resetSourceExit?: boolean; prepareBlocked?: boolean } = {},
): NonNullable<Parameters<typeof beginReaderBackSwipeDragState>[1]> {
  return {
    ...options,
    beforeSourceReturnIntent: () => {
      prepareSourceReaderReturnDrag()
    },
    afterSourceReturnIntent: () => {
      showTopChromeForSourceReturn()
    },
    beforeDetailBackPrepare: ({ returningToFeed }) => {
      if (!returningToFeed) {
        return
      }
      refreshDetailFeedOriginRect(true)
    },
    afterDetailBackPrepare: ({ revealSourceReader }) => {
      if (!revealSourceReader) {
        return
      }
      captureDetailSourceTransitionRects(12, { lock: true })
    },
    afterDetailSourceIntent: () => {
      showSourceReaderUnderDetail()
    },
  }
}

function beginBackSwipeIfAllowed(deltaX: number, deltaY: number, fromDetailFrame = false) {
  const horizontal = fromDetailFrame ? isDetailFrameHorizontalSwipe(deltaX, deltaY) : isBackHorizontalSwipe(deltaX, deltaY)
  if (!readerBackSwipeCandidateActive.value || !horizontal) {
    return false
  }

  beginReaderBackSwipeDragState(deltaX, readerBackSwipeIntentOptions())
  beginBackSwipeTransition(deltaX)
  navigationGesture.cancelCandidates()
  feedPagerTransition.cancelViewSwipeCandidate()
  return true
}

function updateBackSwipe(deltaX: number, deltaY: number, fromDetailFrame = false, currentX = gestureOrigin.originX() + deltaX) {
  beginBackSwipeIfAllowed(deltaX, deltaY, fromDetailFrame)

  if (!readerBackSwipeTrackingActive.value) {
    return false
  }

  suppressFollowingClick()
  updateReaderBackSwipeDragState(
    deltaX,
    { currentX, startX: gestureOrigin.originX(), width: windowWidth.value },
    {
      intent: readerBackSwipeIntentOptions({ resetSourceExit: true, prepareBlocked: true }),
      visual: {
        resetPageStretch: () => {
          pageContentMotion.setSideStretch(0)
        },
        resetPageOffset: () => {
          pageContentMotion.setSideOffset(0)
        },
        applyPageStretch: (nextStretch: number) => {
          pageContentMotion.setSideStretch(nextStretch)
        },
      },
    },
  )

  syncBackSwipeTransition(deltaX)
  return true
}

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

function finishViewSwipe(nextPath: string | null) {
  const result = feedPagerTransition.finishSwipeResult(nextPath)
  swipeTransition.settle(result.committed, result.settlePayload)
  if (result.shouldRevealChromeFirst) {
    setTopChromeVisible(true)
    feedPagerTransition.scheduleDelayedCommit(viewSwipeChromeRevealDelay, () => {
      if (nextPath) {
        void pushRoute(nextPath)
      }
      feedPagerTransition.settleFinishedSwipe(motionDelay(motionNormalDuration))
    })
    scheduleSwipeTransitionReset(viewSwipeChromeRevealDelay + motionNormalDuration)
    return
  }
  if (nextPath) {
    void pushRoute(nextPath)
  }
  feedPagerTransition.settleFinishedSwipe(motionDelay(motionNormalDuration))
  scheduleSwipeTransitionReset(motionNormalDuration)
}

function showTopChromeForViewSwipe() {
  const shouldRevealChrome = topChromeProgress.value < 0.99 || feedContentCollapsed.value
  if (shouldRevealChrome) {
    feedPagerTransition.markStartedWithHiddenChrome()
    setTopChromeVisible(true)
  }
}

function handleTouchStart(event: TouchEvent) {
  if (event.touches.length !== 1) {
    return
  }

  finishCommittedListReturnForGesture()

  const touch = event.touches[0]
  gestureOrigin.begin(touch.clientX, touch.clientY, navigationProgress.value)

  if (navigationVisible.value) {
    navigationGesture.setCandidates({ closeSwipe: navigationOpen.value })
    feedPagerTransition.resetViewSwipeTracking()
    resetReaderBackSwipeCandidateState()
    return
  }

  if (detailBlocksGestures()) {
    beginDetailGestureCandidate(touch.clientX, touch.clientY)
    return
  }
  if (sourceReaderOpen.value) {
    beginReaderBackSwipeCandidateState('source')
    return
  }
  if (!isFeedRoute.value && !navigationVisible.value) {
    beginReaderBackSwipeCandidateState('page')
  }

  navigationGesture.setCandidates({
    edgeSwipe: canStartNavigationOpen(gestureOrigin.originX()),
    closeSwipe: navigationOpen.value,
  })
  if (canStartViewSwipe(gestureOrigin.originX())) {
    feedPagerTransition.beginViewSwipeCandidate()
  } else {
    feedPagerTransition.resetViewSwipeTracking()
  }
}

function handleTouchMove(event: TouchEvent) {
  if (
    !navigationGesture.hasCandidate() &&
    !viewSwipeCandidateActive.value &&
    !readerBackSwipeCandidateActive.value &&
    !navigationGesture.hasActiveSwipe() &&
    !viewSwipeActive.value &&
    !readerBackSwipeTrackingActive.value
  ) {
    return
  }

  const touch = event.touches[0]
  const { deltaX, deltaY } = gestureOrigin.delta(touch.clientX, touch.clientY)
  const horizontal = isHorizontalSwipe(deltaX, deltaY)
  const viewHorizontal = isViewHorizontalSwipe(deltaX, deltaY)

  if (readerBackSwipeCandidateActive.value || readerBackSwipeTrackingActive.value) {
    const handledBackSwipe = updateBackSwipe(deltaX, deltaY, false, touch.clientX)
    if (!handledBackSwipe) {
      return
    }
    event.preventDefault()
    return
  }

  if (navigationGesture.edgeSwipeCandidate() && deltaX > 8 && isNavigationDrag(deltaX, deltaY)) {
    navigationGesture.beginEdgeSwipe()
    feedPagerTransition.cancelViewSwipeCandidate()
    navigationDrawer.beginDrag()
  }

  if (navigationGesture.closeSwipeCandidate() && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    navigationGesture.beginCloseSwipe()
    feedPagerTransition.cancelViewSwipeCandidate()
    navigationDrawer.beginDrag()
  }

  if (navigationGesture.hasActiveSwipe() || viewSwipeActive.value) {
    event.preventDefault()
  }

  if (navigationGesture.edgeSwipe()) {
    navigationDrawer.updateOpeningDrag(deltaX)
    return
  }

  if (navigationGesture.closeSwipe()) {
    navigationDrawer.updateClosingDrag(gestureOrigin.navigationProgress(), deltaX)
    return
  }

  if (viewSwipeCandidateActive.value && viewHorizontal) {
    const dragStart = feedPagerTransition.tryBeginDrag(deltaX)
    if (dragStart.started) {
      navigationGesture.cancelCandidates()
      showTopChromeForViewSwipe()
      beginViewSwipeTransition(deltaX)
    } else {
      return
    }
  }

  if (viewSwipeActive.value) {
    feedPagerTransition.updateDragDelta(deltaX)
    syncViewSwipeTransition(viewDragOffset.value)
    return
  }
}

function handleWindowPointerDown(event: PointerEvent) {
  if (event.pointerType === 'mouse' || event.isPrimary === false) {
    return
  }

  finishCommittedListReturnForGesture()

  gestureOrigin.begin(event.clientX, event.clientY, navigationProgress.value)
  navigationGesture.setCandidates({
    edgeSwipe: canStartNavigationOpen(gestureOrigin.originX()),
    closeSwipe: navigationOpen.value,
  })
  feedPagerTransition.cancelViewSwipeCandidate()
  gestureOrigin.setActivePointerId(
    navigationGesture.hasCandidate() ? event.pointerId : null,
  )
}

function handleWindowPointerMove(event: PointerEvent) {
  if (!gestureOrigin.isActivePointer(event.pointerId)) {
    return
  }

  const { deltaX, deltaY } = gestureOrigin.delta(event.clientX, event.clientY)

  if (navigationGesture.edgeSwipeCandidate() && deltaX > 8 && isNavigationDrag(deltaX, deltaY)) {
    navigationGesture.beginEdgeSwipe()
    navigationDrawer.beginDrag()
  }

  if (navigationGesture.closeSwipeCandidate() && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    navigationGesture.beginCloseSwipe()
    navigationDrawer.beginDrag()
  }

  if (navigationGesture.hasActiveSwipe()) {
    event.preventDefault()
  }

  if (navigationGesture.edgeSwipe()) {
    navigationDrawer.updateOpeningDrag(deltaX)
  } else if (navigationGesture.closeSwipe()) {
    navigationDrawer.updateClosingDrag(gestureOrigin.navigationProgress(), deltaX)
  }
}

function handleWindowPointerUp(event: PointerEvent) {
  if (!gestureOrigin.isActivePointer(event.pointerId)) {
    return
  }

  const { deltaX, deltaY } = gestureOrigin.delta(event.clientX, event.clientY)
  const horizontal = viewSwipeActive.value ? isViewHorizontalSwipe(deltaX, deltaY) : isHorizontalSwipe(deltaX, deltaY)

  if (navigationGesture.edgeSwipe() && navigationGesture.dragStarted()) {
    settleNavigation(horizontal && (deltaX >= navigationOpenDistance || navigationProgress.value >= 0.42))
  }

  if (navigationGesture.closeSwipe() && navigationGesture.dragStarted()) {
    settleNavigation(!(horizontal && (deltaX <= -navigationOpenDistance || navigationProgress.value <= 0.58)))
  }

  gestureOrigin.clearActivePointer()
  resetGestureTracking()
}

function handleWindowPointerCancel(event: PointerEvent) {
  if (!gestureOrigin.isActivePointer(event.pointerId)) {
    return
  }

  gestureOrigin.clearActivePointer()
  const hadNavigationGesture = navigationGesture.hasActiveSwipe()
  resetGestureTracking()
  if (hadNavigationGesture) {
    settleNavigation(navigationProgress.value >= 0.42)
  }
}

function handleTouchEnd(event: TouchEvent) {
  if (
    !navigationGesture.hasCandidate() &&
    !viewSwipeCandidateActive.value &&
    !readerBackSwipeCandidateActive.value &&
    !navigationGesture.hasActiveSwipe() &&
    !viewSwipeActive.value &&
    !readerBackSwipeTrackingActive.value
  ) {
    return
  }

  const touch = event.changedTouches[0]
  const { deltaX, deltaY } = gestureOrigin.delta(touch.clientX, touch.clientY)
  const horizontal = isHorizontalSwipe(deltaX, deltaY)

  if (readerBackSwipeTrackingActive.value) {
    finishBackSwipe(deltaX, deltaY)
    resetGestureTracking()
    return
  }

  if (!navigationGesture.hasActiveSwipe() && !viewSwipeActive.value) {
    resetGestureTracking()
    return
  }

  if (navigationGesture.edgeSwipe()) {
    if (navigationGesture.dragStarted()) {
      settleNavigation(horizontal && (deltaX >= navigationOpenDistance || navigationProgress.value >= 0.42))
    }
  }

  if (navigationGesture.closeSwipe()) {
    if (navigationGesture.dragStarted()) {
      settleNavigation(!(horizontal && (deltaX <= -navigationOpenDistance || navigationProgress.value <= 0.58)))
    }
  }

  if (viewSwipeActive.value) {
    suppressFollowingClick()
    finishViewSwipe(feedPagerTransition.resolveDragCommitPath(deltaX, horizontal, viewSwitchDistance))
  }

  resetGestureTracking()
}

function handleFeedPointerDown(event: PointerEvent) {
  if (!isFeedRoute.value || navigationVisible.value || event.isPrimary === false || event.pointerType === 'mouse') {
    return
  }

  finishCommittedListReturnForGesture()

  if (!canStartViewSwipe(event.clientX)) {
    return
  }

  gestureOrigin.begin(event.clientX, event.clientY, navigationProgress.value)
  navigationGesture.clearActiveSwipes()
  feedPagerTransition.beginPointerTracking(event.pointerId)
}

function handleFeedPointerMove(event: PointerEvent) {
  if (!feedPagerTransition.isActivePointer(event.pointerId) || event.pointerType === 'mouse') {
    return
  }

  const { deltaX, deltaY } = gestureOrigin.delta(event.clientX, event.clientY)

  if (viewSwipeCandidateActive.value && !viewSwipeActive.value) {
    if (!isViewHorizontalSwipe(deltaX, deltaY)) {
      return
    }

    const dragStart = feedPagerTransition.tryBeginDrag(deltaX)
    if (dragStart.blocked) {
      feedPagerTransition.cancelPointerCandidate()
      return
    }

    if (dragStart.started) {
      suppressFollowingClick()
      showTopChromeForViewSwipe()
      beginViewSwipeTransition(deltaX)
      ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
    } else {
      return
    }
  }

  if (!viewSwipeActive.value || !isViewHorizontalSwipe(deltaX, deltaY)) {
    return
  }

  const dragUpdate = feedPagerTransition.updateDragDelta(deltaX, { resetBlockedDirection: true })
  if (dragUpdate.blocked) {
    return
  }

  syncViewSwipeTransition(viewDragOffset.value)
}

function handleFeedPointerUp(event: PointerEvent) {
  if (!feedPagerTransition.isActivePointer(event.pointerId) || event.pointerType === 'mouse') {
    return
  }

  const { deltaX, deltaY } = gestureOrigin.delta(event.clientX, event.clientY)
  const horizontal = isViewHorizontalSwipe(deltaX, deltaY)

  const nextPath = feedPagerTransition.resolveDragCommitPath(deltaX, horizontal, viewSwitchDistance)
  if (nextPath) {
    suppressFollowingClick()
    finishViewSwipe(nextPath)
  } else {
    finishViewSwipe(null)
  }

  feedPagerTransition.endPointerTracking()
}

function handleFeedPointerCancel() {
  feedPagerTransition.endPointerTracking()
  finishViewSwipe(null)
}

function handleTouchCancel() {
  const hadNavigationGesture = navigationGesture.hasActiveSwipe()
  const hadViewGesture = viewSwipeActive.value
  const hadBackGesture = readerBackSwipeTrackingActive.value
  if (hadBackGesture) {
    cancelBackSwipe()
  }
  resetGestureTracking()
  if (hadNavigationGesture && navigationVisible.value && !navigationOpen.value) {
    settleNavigation(false)
  }
  if (hadViewGesture) {
    finishViewSwipe(null)
  }
  feedPagerTransition.endPointerTracking()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeNavigation()
  }
}

function handleResize() {
  viewportSize.sync()
}

function syncDetailContainerMetrics() {
  const container = detailContentRef.value
  if (!container) {
    return
  }

  updateDetailScrollMetricsState(container.scrollTop, container.scrollHeight, container.clientHeight)
}

function scrollDetailContentTo(top: number) {
  const container = detailContentRef.value
  if (!container) {
    return
  }

  container.scrollTop = Math.max(0, top)
  syncDetailContainerMetrics()
}

function handleDetailProgressChange(progress: number) {
  if (detailScrollMax.value <= 0) {
    return
  }

  const nextScrollTop = detailScrollMax.value * clamp(progress)
  updateDetailScrollTopState(nextScrollTop)
  rememberDetailScrollTop(nextScrollTop)
  scrollDetailContentTo(nextScrollTop)
}

function handleDetailProgressDragStart() {
  suppressFollowingClick()
  setDetailProgressDraggingState(true)
}

function handleDetailProgressDragEnd() {
  setDetailProgressDraggingState(false)
}

function handleDetailFrameLoad() {
  requestAnimationFrame(() => {
    syncDetailContainerMetrics()
    if (detailScrollTop.value > 0) {
      scrollDetailContentElementTo(detailScrollTop.value)
    }
  })
}

function handleMessage(event: MessageEvent) {
  if (detailCommittedListReturn()) {
    return
  }

  if (event.data?.type === 'messagefeed-detail-scroll' && detailReaderOpen.value) {
    const payload = event.data as { scrollTop?: number; scrollHeight?: number; clientHeight?: number }
    const scrollHeight = Number(payload.scrollHeight ?? 0)
    if (Number.isFinite(scrollHeight)) {
      updateDetailFrameContentHeightState(scrollHeight)
    }
    requestAnimationFrame(syncDetailContainerMetrics)
    return
  }

  if (event.data?.type !== 'messagefeed-detail-gesture' || !detailReaderOpen.value) {
    return
  }

  if (navigationVisible.value) {
    return
  }

  const payload = event.data as {
    phase?: 'start' | 'move' | 'end' | 'cancel'
    source?: string
    startX?: number
    startY?: number
    x?: number
    dx?: number
    dy?: number
  }
  const fromDetailFrame = payload.source === 'detail-frame'
  const frameOffset = fromDetailFrame ? detailFrameViewportOffset() : { left: 0, top: 0 }
  const startX = Number(payload.startX ?? 0) + frameOffset.left
  const startY = Number(payload.startY ?? 0) + frameOffset.top
  const deltaX = Number(payload.dx ?? 0)
  const deltaY = Number(payload.dy ?? 0)
  const currentX = Number(payload.x ?? Number(payload.startX ?? 0) + deltaX) + frameOffset.left

  if (payload.phase === 'start') {
    beginDetailGestureCandidate(startX, startY)
    return
  }

  if (payload.phase === 'move') {
    updateBackSwipe(deltaX, deltaY, fromDetailFrame, currentX)
    return
  }

  if (payload.phase === 'end') {
    if (readerBackSwipeTrackingActive.value) {
      finishBackSwipe(deltaX, deltaY)
      resetGestureTracking()
      return
    }
    resetGestureTracking()
    return
  }

  if (payload.phase === 'cancel') {
    if (readerBackSwipeTrackingActive.value) {
      cancelBackSwipe()
    }
    resetGestureTracking()
  }
}

function loadReaderSettings() {
  setSourceTimelinePreloadEnabledState(localStorage.getItem('messagefeed-source-preload') !== 'false')
}

function handleReaderSettingsChanged(event: Event) {
  const detail = (event as CustomEvent<{ sourceTimelinePreload?: boolean }>).detail
  if (typeof detail?.sourceTimelinePreload === 'boolean') {
    setSourceTimelinePreloadEnabledState(detail.sourceTimelinePreload)
  } else {
    loadReaderSettings()
  }
}

function setTopChromeVisible(visible: boolean) {
  chromeState.setVisible(visible, { settleDelayMS: motionDelay(topChromeSettleDuration) })
}

function currentContentScrollTop() {
  if (sourceReaderOpen.value) {
    return sourceReaderScrollTop.value
  }

  if (isFeedRoute.value) {
    return feedScrollTop.value
  }

  return pageOutlet.currentScrollTop()
}

function handleFeedTopPullStart(startedWithVisibleChrome = false) {
  if (isFeedRoute.value && feedInteraction.pullRefreshing) {
    return
  }

  feedTopPull.begin(startedWithVisibleChrome || feedTopChromeIsVisiblyOpen.value, topChromeProgress.value)
  chromeState.beginRefreshing()
}

function handleFeedTopPullMove(distance: number) {
  if (!feedTopPulling.value || (isFeedRoute.value && feedInteraction.pullRefreshing)) {
    return
  }

  if (!feedTopPullStartedWithChrome.value && feedTopChromeIsVisiblyOpen.value) {
    feedTopPull.markStartedWithChrome()
  }

  if (!feedTopPullStartedWithChrome.value && currentContentScrollTop() > 0) {
    return
  }

  if (feedTopPullStartedWithChrome.value) {
    chromeState.setRefreshingProgress(1, { contentCollapsed: false })
    return
  }

  chromeState.setRefreshingProgress(clamp(feedTopPullStartProgress.value - distance / feedHeaderHeight.value))
}

function handleFeedTopPullEnd(shouldRefresh = false) {
  if (!feedTopPulling.value) {
    feedTopPull.resetStartedWithChrome()
    return
  }

  const startedWithChrome = feedTopPullStartedWithChrome.value
  feedTopPull.finish()

  if (shouldRefresh) {
    refreshCompletion.recordStartedWithChrome(startedWithChrome)
    chromeState.commitRefreshing(startedWithChrome)
    return
  }

  if (topChromeProgress.value <= 0.04) {
    chromeState.setCollapsedHidden({ settleDelayMS: motionDelay(topChromeSettleDuration) })
    feedTopPull.resetStartedWithChrome()
    return
  }

  setTopChromeVisible(true)
  feedTopPull.resetStartedWithChrome()
}

function updateTopTabsByScroll(current: number, previous: number) {
  if (current <= 1 && topChromeProgress.value < 0.99 && !feedPullActive.value && !feedTopPulling.value) {
    setTopChromeVisible(true)
    return
  }

  if (feedPullActive.value || sourcePullActive.value || feedTopPulling.value || feedChromeSettling.value) {
    return
  }

  const delta = current - previous
  if (Math.abs(delta) < 3 || current < 0) {
    return
  }

  if (detailReaderOpen.value) {
    const max = detailScrollMax.value
    const bottomStabilityZone = 28
    const nearBottom =
      max > 0 &&
      current >= max - bottomStabilityZone &&
      previous >= max - bottomStabilityZone
    if (nearBottom) {
      return
    }
  }

  const hideThreshold = detailReaderOpen.value ? feedHeaderHeight.value : isFeedRoute.value ? 8 : feedHeaderHeight.value
  if (delta > 0 && current >= hideThreshold && topChromeProgress.value > 0.01) {
    setTopChromeVisible(false)
    return
  }

  if (delta < 0 && topChromeProgress.value < 0.99) {
    setTopChromeVisible(true)
  }
}

function handleFeedContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  const scrollUpdate = scrollHistory.update('feed', current)
  feedScroll.update(current)
  updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
  scheduleReaderSessionSave()
}

function handlePageContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  const scrollUpdate = scrollHistory.update('page', current)
  updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
  scheduleReaderSessionSave()
}

function handleSourceReaderScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  const scrollUpdate = scrollHistory.update('source', current)
  updateSourceReaderScrollTopState(current)
  updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
  scheduleReaderSessionSave()
}

function handleDetailContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  const scrollUpdate = scrollHistory.update('detail', current)
  updateDetailScrollMetricsState(current, target.scrollHeight, target.clientHeight)
  updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
  scheduleReaderSessionSave()
}

function resetPageTopPullTracking() {
  pagePullRefresh.finishGestureTracking()
}

function pageRubberBandOffset(distance: number) {
  if (distance <= 0) {
    return 0
  }
  return Math.min(22, Math.log1p(distance) * 4.8)
}

function settlePagePullOffset() {
  pagePullRefresh.settleOffset(motionDelay(topChromeSettleDuration))
}

function handlePageTouchStart(event: TouchEvent) {
  if (
    isFeedRoute.value ||
    event.touches.length !== 1 ||
    pagePullRefreshing.value ||
    currentContentScrollTop() > 0 ||
    isPageTopPullControlTarget(event.target)
  ) {
    resetPageTopPullTracking()
    return
  }

  const touch = event.touches[0]
  pagePullRefresh.beginGestureCandidate(touch.clientX, touch.clientY)
}

function handlePageTouchMove(event: TouchEvent) {
  if (
    isFeedRoute.value ||
    event.touches.length !== 1 ||
    currentContentScrollTop() > 0 ||
    (!trackingPageTopPullCandidate.value && !trackingPageTopPull.value)
  ) {
    return
  }

  const touch = event.touches[0]
  const { deltaX, deltaY } = pagePullRefresh.gestureDelta(touch.clientX, touch.clientY)

  if (!trackingPageTopPull.value) {
    if (shouldCancelTopPull(deltaX, deltaY)) {
      resetPageTopPullTracking()
      return
    }

    if (shouldWaitForTopPull(deltaX, deltaY)) {
      return
    }

    pagePullRefresh.beginGestureTracking()
    setTopChromeVisible(true)
  }

  if (trackingPageTopPull.value) {
    event.preventDefault()
    pagePullRefresh.updateGestureDistance(deltaY)
    pagePullRefresh.setGestureOffset(pageRubberBandOffset(deltaY))
  }
}

async function refreshCurrentPageFromPull() {
  const refreshPage = pageOutlet.currentRefreshPage()
  if (!refreshPage || pagePullRefreshing.value) {
    return
  }

  pagePullRefresh.beginRefreshing()
  try {
    await refreshPage({ noticeDelayMS: motionQuickDuration, suppressStartNotice: true })
  } finally {
    pagePullRefresh.finishRefreshing()
    chromeState.setCollapsedHidden({ settleDelayMS: motionDelay(topChromeSettleDuration) })
  }
}

function handlePageTouchEnd() {
  if (trackingPageTopPull.value) {
    const shouldRefresh = pageTopPullDistance.value >= pageRefreshThreshold
    feedTopPull.finish()
    setTopChromeVisible(true)
    settlePagePullOffset()
    if (shouldRefresh) {
      void refreshCurrentPageFromPull()
    }
  } else if (trackingPageTopPullCandidate.value) {
    feedTopPull.finish()
  }
  resetPageTopPullTracking()
}

function handlePageTouchCancel() {
  if (trackingPageTopPull.value || trackingPageTopPullCandidate.value) {
    feedTopPull.finish()
    setTopChromeVisible(true)
    settlePagePullOffset()
  }
  resetPageTopPullTracking()
}

watch(
  () => route.name,
  () => {
    resetGestureTracking()
    resetPageTopPullTracking()
    feedTopPull.finish()
    pagePullRefresh.resetMotion()
    feedPagerTransition.resetDragOffset()
    if (isFeedRoute.value) {
      setTopChromeVisible(true)
      nextTick(() => {
        const current = feedContent.currentScrollTop()
        feedScroll.update(current)
        scrollHistory.set('feed', current)
      })
    } else {
      setTopChromeVisible(true)
      nextTick(() => {
        scrollHistory.set('page', pageOutlet.currentScrollTop())
      })
    }
    scheduleReaderSessionSave()
    scheduleReaderURLAndHistorySync()
  },
)

watch(
  () => [
    route.fullPath,
    navigationVisible.value,
    sourceReaderVisible.value,
    readerSource.value?.id ?? 0,
    readerSource.value?.kind ?? '',
    detailItem.value?.id ?? 0,
    detailSourceKind.value,
    detailOpenedFromSourceReader.value,
    detailListReturnCommitted.value,
    sourceReaderReturnMode.value,
    sourceReaderBackDetailItemId.value,
    parkedDetailStackDepth.value,
  ],
  () => {
    scheduleReaderSessionSave()
    scheduleReaderURLAndHistorySync()
  },
)

watch(
  () => [
    detailSourceExitProgress.value,
    topChromeProgress.value,
    feedContentCollapsed.value,
    feedScrollTop.value,
    sourceReaderScrollTop.value,
    detailScrollTop.value,
  ],
  () => {
    scheduleReaderSessionSave()
  },
)

watch(
  () => feedInteraction.pullRefreshing,
  (refreshing) => {
    if (refreshing) {
      refreshCompletion.begin({
        viewKey: feedInteraction.pullViewKey,
        startedWithVisibleChrome: feedTopPullStartedWithChrome.value,
      })
    }
  },
)

watch(
  feedOrSourcePullActive,
  (active) => {
    if (!active && refreshCompletion.wasActive.value) {
      const refreshResult = refreshCompletion.finish(motionDelay(topChromeSettleDuration))
      const shouldSettleSourceContent = refreshResult.wasSource
      if (shouldSettleSourceContent) {
        settleSourceContentAfterRefresh()
      }
      feedTopPull.resetStartedWithChrome()
      chromeState.setCollapsedHidden({ settleDelayMS: motionDelay(topChromeSettleDuration) })
    }

    if (!active && !refreshCompletion.wasActive.value) {
      refreshCompletion.resetInactive()
      feedTopPull.resetStartedWithChrome()
    }
  },
)

onMounted(() => {
  loadReaderSettings()
  themeState.load()
  virtualBackGuard.installRouterGuard()
  void router.isReady().then(() => restoreReaderSession()).finally(() => {
    routeRuntime.markReaderSessionReady()
    scheduleReaderURLAndHistorySync()
  })
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', handleResize)
  window.addEventListener('message', handleMessage)
  window.addEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
  window.addEventListener('popstate', virtualBackGuard.handlePopState)
  window.addEventListener('beforeunload', saveReaderSessionNow)
  window.addEventListener('pointerdown', handleWindowPointerDown, { passive: true })
  window.addEventListener('pointermove', handleWindowPointerMove, { passive: false })
  window.addEventListener('pointerup', handleWindowPointerUp, { passive: true })
  window.addEventListener('pointercancel', handleWindowPointerCancel, { passive: true })
  window.addEventListener('touchstart', handleTouchStart, { passive: true })
  window.addEventListener('touchmove', handleTouchMove, { passive: false })
  window.addEventListener('touchend', handleTouchEnd, { passive: true })
  window.addEventListener('touchcancel', handleTouchCancel, { passive: true })
})

onUnmounted(() => {
  saveReaderSessionNow()
  virtualBackGuard.uninstallRouterGuard()
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('message', handleMessage)
  window.removeEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
  window.removeEventListener('popstate', virtualBackGuard.handlePopState)
  window.removeEventListener('beforeunload', saveReaderSessionNow)
  window.removeEventListener('pointerdown', handleWindowPointerDown)
  window.removeEventListener('pointermove', handleWindowPointerMove)
  window.removeEventListener('pointerup', handleWindowPointerUp)
  window.removeEventListener('pointercancel', handleWindowPointerCancel)
  window.removeEventListener('touchstart', handleTouchStart)
  window.removeEventListener('touchmove', handleTouchMove)
  window.removeEventListener('touchend', handleTouchEnd)
  window.removeEventListener('touchcancel', handleTouchCancel)
  feedPagerTransition.clearTimers()
  swipeTransition.clearTimer()
  navigationDrawer.clearTimer()
  refreshCompletion.clearTimer()
  chromeState.clearTimer()
  sourceContentMotion.clearTimer()
  pagePullRefresh.clearTimers()
  clickSuppression.clearTimer()
  clearSourceNoticeTimer()
  clearReaderStackTimers()
  readerSession.clearTimer()
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

    <main
      class="app-main"
      :class="mainClass"
      :style="mainStyle"
      :data-swipe-phase="swipePhase"
      :data-swipe-direction="swipeDirection || undefined"
      :data-swipe-progress="swipeProgress.toFixed(3)"
      :data-swipe-blocked="swipeIsBlocked ? 'true' : undefined"
    >
      <TopChrome
        variant="app"
        :phase="topChromePhase"
        :progress="feedHeaderProgress"
        :root-class="headerClass"
        :root-style="headerStyle"
      >
        <div class="app-header-slot" :class="{ 'app-header-slot--feed': isFeedRoute || detailChromeVisible }">
          <AppFeedHeaderContent
            v-if="isFeedRoute || detailChromeVisible"
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
            :pull-status-style="pullStatusStyle"
            :pull-refreshing="feedInteraction.pullRefreshing"
            :pull-icon-style="pullIconStyle"
            :pull-status-text="pullStatusText"
            :pull-status-meta="pullStatusMeta"
            @navigate="navigateTo"
          />
          <AppPageHeaderContent
            v-else
            :page-title="pageTitle"
            :page-pull-active="pagePullActive"
            :page-title-layer-style="pageTitleLayerStyle"
            :page-pull-status-style="pagePullStatusStyle"
            :page-pull-refreshing="pagePullRefreshing"
            :page-pull-icon-style="pagePullIconStyle"
            :page-pull-status-text="pagePullStatusText"
            :page-pull-status-meta="pagePullStatusMeta"
          />
        </div>
      </TopChrome>

      <section
        v-if="isFeedRoute"
        :ref="setFeedContentElement"
        class="app-content app-content--feed"
        @scroll.passive="handleFeedContentScroll"
        @pointerdown="handleFeedPointerDown"
        @pointermove="handleFeedPointerMove"
        @pointerup="handleFeedPointerUp"
        @pointercancel="handleFeedPointerCancel"
      >
        <FeedPager
          :active-key="route.name"
          :detail-reader-open="detailReaderOpen"
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
          @top-pull-start="handleFeedTopPullStart"
          @top-pull-move="handleFeedTopPullMove"
          @top-pull-end="handleFeedTopPullEnd"
          @open-item="openItemReader"
        />
      </section>
      <AppPageOutlet
        v-else
        :inner-style="pageContentInnerStyle"
        @content-ref="setPageContentElement"
        @view-ref="setPageViewInstance"
        @content-scroll="handlePageContentScroll"
        @touch-start="handlePageTouchStart"
        @touch-move="handlePageTouchMove"
        @touch-end="handlePageTouchEnd"
        @touch-cancel="handlePageTouchCancel"
        @open-source="openSourceReader"
      />
    </main>

    <AppReaderStackContent
      :source-mounted="sourceReaderMounted && Boolean(readerSource)"
      :source-under-detail="sourceReaderUnderDetail"
      :source-style="sourceReaderStyle"
      :source-title-reveal-mounted="Boolean(readerSource)"
      :source-title-reveal-visible="sourceTitleRevealVisible"
      :source-title-reveal-style="sourceTitleRevealStyle"
      :source-title="readerSource?.name || ''"
      :source-meta="sourceToggleActive ? '已订阅' : '未订阅'"
      :source-name-morph-mounted="Boolean(detailItem)"
      :source-name-morph-visible="sourceNameMorphVisible"
      :source-name-morph-style="sourceNameMorphStyle"
      :source-name-morph-text="detailItem?.source_name || '未知来源'"
      :detail-open="detailReaderOpen"
      :detail-motion-settling="readerMotionSettling"
      :detail-returning-feed="detailReturningToFeed"
      :detail-style="detailReaderStyle"
      :source-notice="sourceNotice"
      :top-chrome-phase="topChromePhase"
      :top-chrome-progress="topChromeProgress"
      :source-header-style="sourceHeaderStyle"
      :source-name="readerSource?.name || ''"
      :source-title-text-style="sourceTitleTextStyle"
      :source-title-layer-style="sourceTitleLayerStyle"
      :source-main-layer-style="sourceMainLayerStyle"
      :source-pull-status-style="sourcePullStatusStyle"
      :source-pull-icon-style="sourcePullIconStyle"
      :source-pull-active="sourcePullActive"
      :source-pull-refreshing="feedInteraction.pullRefreshing"
      :pull-status-text="pullStatusText"
      :pull-status-meta="pullStatusMeta"
      :source-toggle-active="sourceToggleActive"
      :source-toggle-label="sourceToggleLabel"
      :source-toggle-disabled="sourceToggleDisabled"
      :source-content-style="sourceContentStyle"
      :reader-source="readerSource"
      :source-refresh-nonce="sourceReaderRefreshNonce"
      :source-scroll-top="sourceReaderScrollTop"
      :feed-header-height="feedHeaderHeight"
      :morphing-item-id="morphingItemId"
      :morphing-height-lock-item-id="morphingHeightLockItemId"
      :morphing-item-height="morphingItemHeight"
      :feed-item-preview-progress="feedItemPreviewProgress"
      :source-background-refresh="!sourceReaderVisible"
      :detail-entry-settling="detailEntrySettling"
      :detail-chrome-settling="feedChromeSettling"
      :detail-transition-style="detailTransitionSurfaceStyle"
      :detail-item="detailItem"
      :detail-morph-visible="detailMorphTextVisible"
      :detail-morph-text-style="detailMorphTextStyle"
      :detail-morph-source-label-style="detailMorphSourceLabelStyle"
      :detail-display-date="detailDisplayDate"
      :detail-morph-summary-visible="detailMorphSummaryVisible"
      :detail-preview-summary="detailPreviewSummary"
      :detail-content-style="detailContentStyle"
      :detail-loading="detailLoading"
      :detail-error="detailError"
      :detail-srcdoc="detailSrcdoc"
      :detail-inline-source-style="detailInlineSourceStyle"
      :detail-progress-visible="detailProgressVisible"
      :detail-progress-dragging="detailProgressDragging"
      :detail-reading-progress="detailReadingProgress"
      :detail-progress-style="detailProgressStyle"
      :detail-progress-fill-style="detailProgressFillStyle"
      :detail-progress-thumb-style="detailProgressThumbStyle"
      @source-content-ref="setSourceReaderContentElement"
      @source-content-scroll="handleSourceReaderScroll"
      @open-navigation="openNavigation"
      @toggle-source-subscription="toggleSourceReaderSubscription"
      @top-pull-start="handleFeedTopPullStart"
      @top-pull-move="handleFeedTopPullMove"
      @top-pull-end="handleFeedTopPullEnd"
      @open-item="openItemReader"
      @detail-content-ref="setDetailContentElement"
      @detail-content-scroll="handleDetailContentScroll"
      @detail-inline-source-ref="setDetailInlineSourceElement"
      @detail-frame-ref="setDetailFrameElement"
      @detail-frame-load="handleDetailFrameLoad"
      @detail-progress-drag-start="handleDetailProgressDragStart"
      @detail-progress-drag-end="handleDetailProgressDragEnd"
      @detail-progress-change="handleDetailProgressChange"
    />
  </div>
</template>
