<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { IconBook } from '@arco-design/web-vue/es/icon'
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
import { type ChromePhase, useChromeState } from '@/composables/useChromeState'
import { useReaderSourceSubscription } from '@/composables/useReaderSourceSubscription'
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

type SwipeSurface =
  | 'feed:subscriptions'
  | 'feed:recommendations'
  | 'reader:detail'
  | 'reader:source'
  | 'page:management'
type PageViewExpose = {
  refreshPage?: (options?: { noticeDelayMS?: number; suppressStartNotice?: boolean }) => Promise<void> | void
}

const route = useRoute()
const router = useRouter()
const feedInteraction = useFeedInteractionStore()
const feedContentRef = ref<HTMLElement | null>(null)
const pageContentRef = ref<HTMLElement | null>(null)
const pageViewRef = ref<PageViewExpose | null>(null)
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
})
const feedScrollTop = ref(0)
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
const windowWidth = ref(typeof window === 'undefined' ? 1440 : window.innerWidth)
const windowHeight = ref(typeof window === 'undefined' ? 900 : window.innerHeight)
const navigationDrawer = useNavigationDrawer({ windowWidth, resolveDelay: motionDelay })
const navigationOpen = navigationDrawer.open
const navigationProgress = navigationDrawer.progress
const navigationSettling = navigationDrawer.settling
const navigationWidth = navigationDrawer.width
const navigationVisible = navigationDrawer.visible
const navigationPanelStyle = navigationDrawer.panelStyle
const navigationScrimStyle = navigationDrawer.scrimStyle
const darkTheme = ref(false)
const refreshCompletion = useRefreshCompletionState()
const refreshStartedWithChrome = refreshCompletion.startedWithChrome
const feedRefreshSettling = refreshCompletion.settling
const feedTopPulling = ref(false)
const feedTopPullStartedWithChrome = ref(false)
const pagePullRefresh = usePullRefresh({ threshold: 52 })
const pageRefreshThreshold = pagePullRefresh.threshold
const pagePullOffset = pagePullRefresh.offset
const pagePullDistance = pagePullRefresh.distance
const pagePullSettling = pagePullRefresh.settling
const pagePullRefreshing = pagePullRefresh.refreshing
const pageContentMotion = usePageContentMotion({ pullOffset: pagePullOffset })
const pageSideStretch = pageContentMotion.sideStretch
const pageContentInnerStyle = pageContentMotion.contentStyle
const readerMorphDuration = 360
const readerMorphCleanupBuffer = 96
const readerMorphCleanupDelay = readerMorphDuration + readerMorphCleanupBuffer
const readerRectRetryDelay = 64
const homeExitDoubleBackMs = 1600
let programmaticRouteNavigation = false
let readerSessionInitialized = false
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

const selectedKeys = computed(() => [route.name?.toString() ?? 'subscriptions'])
const pageTitle = computed(() => route.meta.title?.toString() ?? '订阅')
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
  canHandleNavigation: () => readerSessionInitialized && !programmaticRouteNavigation,
  consumeBack: runVirtualBackAnimation,
  onBackConsumed: () => {
    scheduleReaderSessionSave()
    scheduleReaderURLAndHistorySync(true)
  },
})
const readerRouteSync = useReaderRouteSync({
  route,
  router,
  canSync: () => readerSessionInitialized && !readerSession.restoring.value,
  getReaderSource: () => readerSource.value,
  isSourceReaderOpen: () => sourceReaderOpen.value,
  getDetailItemID: () => detailItem.value?.id,
  getDetailSourceKind: () => detailSourceKind.value,
  setProgrammaticRouteNavigation: (active) => {
    programmaticRouteNavigation = active
  },
  syncVirtualHistoryState: virtualBackGuard.syncHistoryState,
})
const isFeedRoute = computed(() => ['subscriptions', 'recommendations'].includes(route.name?.toString() ?? ''))
const cornerButtonLabel = computed(() => '打开导航')
const feedPagerTransition = useFeedPagerTransition({
  getActiveKey: () => route.name,
  getWindowWidth: () => windowWidth.value,
  isFeedRoute: () => isFeedRoute.value,
  isDetailReaderOpen: () => detailReaderOpen.value,
})
const viewDragOffset = feedPagerTransition.dragOffset
const viewSettling = feedPagerTransition.settling
const activeFeedIndex = feedPagerTransition.activeIndex
const feedPullActive = computed(() => isFeedRoute.value && (feedInteraction.pullActive || feedInteraction.pullOffset > 1))
const pagePullActive = computed(() => !isFeedRoute.value && (pagePullRefreshing.value || pagePullOffset.value > 1))
const sourcePullActive = computed(
  () =>
    sourceReaderOpen.value &&
    feedInteraction.pullViewKey.startsWith('source:') &&
    (feedInteraction.pullActive || feedInteraction.pullOffset > 1),
)
const feedOrSourcePullActive = computed(() => feedPullActive.value || sourcePullActive.value)
const pullProgress = computed(() => Math.min(feedInteraction.pullOffset / 76, 1))
const pagePullProgress = pagePullRefresh.distanceProgress
const sourcePullProgress = computed(() => Math.min(feedInteraction.pullOffset / 76, 1))
const feedHeaderHeight = computed(() => (windowWidth.value <= 720 ? 78 : 86))
const feedHeaderProgress = computed(() => {
  if (!isFeedRoute.value) {
    return topChromeProgress.value
  }

  if (feedPullActive.value) {
    return Math.max(topChromeProgress.value, pullProgress.value)
  }

  return topChromeProgress.value
})
const feedContentSpace = computed(() => {
  if (feedTopPullStartedWithChrome.value || refreshStartedWithChrome.value) {
    return feedHeaderHeight.value
  }

  if (feedTopPulling.value) {
    return feedHeaderHeight.value * (feedPullActive.value ? pullProgress.value : topChromeProgress.value)
  }

  if (feedPullActive.value) {
    return feedHeaderHeight.value * Math.max(topChromeProgress.value, pullProgress.value)
  }

  if (feedContentCollapsed.value && topChromeProgress.value <= 0.01) {
    return 0
  }

  return feedHeaderHeight.value
})
const freezeFeedBodyDuringTopRefresh = computed(
  () => feedTopPullStartedWithChrome.value || refreshStartedWithChrome.value,
)
const feedTopChromeIsVisiblyOpen = computed(
  () => !feedContentCollapsed.value || topChromeProgress.value > 0.04,
)
const feedTabsVisible = computed(() => isFeedRoute.value && topChromeProgress.value > 0.04)
const feedChromeHidden = computed(() => isFeedRoute.value && feedHeaderProgress.value <= 0.01 && !feedPullActive.value)
const feedHeaderReturnProgress = computed(() => (isFeedRoute.value ? detailFeedHeaderReturnProgress.value : 0))
const feedTabsLayerHidden = computed(() => {
  if (!isFeedRoute.value || feedPullActive.value) {
    return true
  }
  if (detailReaderOpen.value) {
    return feedHeaderReturnProgress.value <= 0.001
  }
  return !feedTabsVisible.value
})
const feedCornerHidden = computed(
  () =>
    (sourceReaderOpen.value && !detailChromeVisible.value) ||
    (detailChromeVisible.value && !detailHeaderVisible.value) ||
    (!detailChromeVisible.value && isFeedRoute.value && (feedPullActive.value || feedHeaderProgress.value <= 0.05)),
)
const detailHeaderVisible = computed(() => detailChromeVisible.value && topChromeProgress.value > 0.04)
const headerDetailLayoutActive = computed(
  () => detailChromeVisible.value || (detailReaderOpen.value && isFeedRoute.value && feedHeaderReturnProgress.value > 0.001),
)
const pullStatusText = computed(() => feedInteraction.statusText)
const pullStatusMeta = computed(() => feedInteraction.statusMeta)
const pagePullStatusText = computed(() => {
  if (pagePullRefreshing.value) {
    return '抓取中'
  }
  return pagePullProgress.value >= 1 ? '释放刷新' : '下拉刷新'
})
const pagePullStatusMeta = computed(() => {
  if (pagePullRefreshing.value) {
    return pageTitle.value === '订阅管理' ? '正在更新订阅源列表与推荐源目录' : `正在更新${pageTitle.value}`
  }
  return pageTitle.value === '订阅管理' ? '下拉更新订阅管理' : `下拉更新${pageTitle.value}`
})
const pullStatusStyle = computed(() => ({
  ...chromeLayerStyle(feedPullActive.value, pullProgress.value, { shift: -10, scaleStart: 0.96 }),
}))
const pullIconStyle = computed(() => ({
  transform: feedInteraction.pullRefreshing ? 'none' : cssRotate(pullProgress.value * 300),
}))
const pagePullStatusStyle = computed(() => ({
  ...chromeLayerStyle(pagePullActive.value, pagePullProgress.value, { shift: -10, scaleStart: 0.96 }),
}))
const pagePullIconStyle = computed(() => ({
  transform: pagePullRefreshing.value ? 'none' : cssRotate(pagePullProgress.value * 300),
}))
const feedTabsLayerStyle = computed(() => {
  if (detailReaderOpen.value) {
    return chromeLayerStyle(feedHeaderReturnProgress.value > 0.001, feedHeaderReturnProgress.value, {
      shift: 7,
      scaleStart: 0.98,
      disableTransition: readerBackDragging.value,
      pointerEnabled: !detailBlocksGestures(),
    })
  }

  return chromeLayerStyle(!feedPullActive.value, feedHeaderProgress.value)
})
const feedTabsTargetLayerStyle = computed(() =>
  chromeLayerStyle(
    viewSwipeTargetVisible.value && !feedPullActive.value,
    feedHeaderProgress.value * viewSwipeTargetProgress.value,
    {
      shift: 6,
      scaleStart: 0.985,
      pointerEnabled: false,
    },
  ),
)
const sourcePullStatusStyle = computed(() => ({
  ...chromeLayerStyle(sourcePullActive.value, sourcePullProgress.value, { shift: -10, scaleStart: 0.96 }),
}))
const sourcePullIconStyle = computed(() => ({
  transform: feedInteraction.pullRefreshing ? 'none' : cssRotate(sourcePullProgress.value * 300),
}))
const feedTrackStyle = feedPagerTransition.trackStyle
const viewSwipeProgress = feedPagerTransition.swipeProgress
const viewSwipeTargetKey = feedPagerTransition.targetKey
const viewSwipeTargetVisible = feedPagerTransition.targetVisible
const viewSwipeTargetProgress = feedPagerTransition.targetProgress
const mainClass = computed(() => ({
  'app-main--feed': isFeedRoute.value,
  'app-main--page': !isFeedRoute.value,
  'app-main--tabs-hidden': feedChromeHidden.value,
  'app-main--refreshing': feedPullActive.value || pagePullActive.value,
  'app-main--pull-dragging': feedPullActive.value && !feedInteraction.pullRefreshing,
  'app-main--top-refresh-contained': freezeFeedBodyDuringTopRefresh.value,
  'app-main--refresh-settling': feedRefreshSettling.value,
  'app-main--chrome-settling': feedChromeSettling.value,
  'app-main--page-pull-settling': pagePullSettling.value,
  'app-main--view-settling': viewSettling.value,
  'app-main--detail-reader': detailReaderOpen.value && !detailReturningToFeed.value,
  'app-main--detail-chrome': detailChromeVisible.value,
}))
const sourceHeaderSpace = computed(() =>
  feedContentCollapsed.value && topChromeProgress.value <= 0.01 ? 0 : feedHeaderHeight.value,
)
const sourceContentMotion = useSourceContentMotion({
  headerSpace: sourceHeaderSpace,
  headerHeight: feedHeaderHeight,
  isVisible: () => sourceReaderVisible.value,
  resolveDelay: motionDelay,
})
const sourceContentStyle = sourceContentMotion.contentStyle
const sourceReaderStyle = computed(() => {
  const underlayBaseOpacity = darkTheme.value ? 0.74 : 0.54
  const overlayBaseOpacity = darkTheme.value ? 0.48 : 0.34
  const sourceStretch = sourceReaderStretch.value
  return {
    '--feed-header-height': `${feedHeaderHeight.value}px`,
    '--source-header-space': cssPx(sourceHeaderSpace.value),
    zIndex: sourceReaderUnderDetail.value ? 96 : sourceReaderVisible.value ? 110 : 90,
    opacity: !sourceReaderVisible.value
      ? '0'
      : sourceReaderUnderDetail.value
        ? (overlayBaseOpacity + sourceReaderRevealProgress.value * (1 - overlayBaseOpacity)).toFixed(3)
        : '1',
    pointerEvents:
      !sourceReaderVisible.value || detailBlocksGestures() ? ('none' as const) : ('auto' as const),
    '--source-underlay-blur': `${((1 - sourceReaderRevealProgress.value) * (darkTheme.value ? 5 : 8)).toFixed(2)}px`,
    '--source-underlay-opacity': (underlayBaseOpacity + sourceReaderRevealProgress.value * (1 - underlayBaseOpacity)).toFixed(3),
    transform: `${cssTranslate3d(sourceReaderOffset.value, 0)} scaleX(${(1 + Math.abs(sourceStretch)).toFixed(4)})`,
    transformOrigin: stretchTransformOrigin(sourceStretch, sourceStretchAnchor.value),
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-normal) ease, transform var(--motion-normal) var(--ease-standard)',
  }
})
const sourceHeaderStyle = computed(() => ({
  opacity: topChromeProgress.value.toFixed(3),
  pointerEvents: topChromeProgress.value > 0.86 ? ('auto' as const) : ('none' as const),
  transform: cssTranslate3d(0, (topChromeProgress.value - 1) * feedHeaderHeight.value),
  transition:
    feedChromeSettling.value || readerBackDragging.value
      ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) ease'
      : undefined,
}))
const detailHeaderLayerStyle = computed(() => chromeLayerStyle(detailHeaderVisible.value, topChromeProgress.value))
const pageTitleLayerStyle = computed(() => chromeLayerStyle(!pagePullActive.value, feedHeaderProgress.value))
const sourceMainLayerStyle = computed(() => chromeLayerStyle(!sourcePullActive.value, topChromeProgress.value))
const detailReaderStyle = computed(() => ({
  transform: `translate3d(0, 0, 0) scaleX(${(1 + Math.abs(detailReaderStretch.value)).toFixed(4)})`,
  transition: readerBackDragging.value ? 'none' : undefined,
  transformOrigin: stretchTransformOrigin(detailReaderStretch.value, detailStretchAnchor.value),
  pointerEvents: detailCommittedListReturn() ? ('none' as const) : ('auto' as const),
  '--detail-overlay-opacity':
    sourceReaderBlockedBackSwipeActive.value || detailCommittedListReturn() || detailReturningToFeed.value
    ? '0'
    : detailSurfaceProgress.value.toFixed(3),
}))
const detailSourceFallbackTargetRect = computed<RectSnapshot>(() => {
  const side = windowWidth.value <= 720 ? 24 : 46
  const top = feedHeaderHeight.value + 24
  return {
    left: side,
    top,
    width: Math.max(1, windowWidth.value - side * 2),
    height: 146,
  }
})
const detailSurfaceMargin = computed(() => (windowWidth.value <= 720 ? 10 : 14))
const detailExpandedTop = computed(() =>
  (feedHeaderHeight.value + detailSurfaceMargin.value) * topChromeProgress.value,
)
const detailFrameMinHeight = computed(() =>
  Math.max(300, windowHeight.value - detailExpandedTop.value - detailSurfaceMargin.value - 96),
)
const detailTransitionSurfaceStyle = computed(() => {
  const origin = detailOriginRect.value
  const sourceExiting =
    detailRestoringFromSourceReader.value ||
    (detailSourceExitProgress.value >= detailBackExitProgress.value && detailSourceExitProgress.value > 0)
  const collapsedTarget = sourceExiting ? detailSourceItemTargetRect.value ?? detailSourceFallbackTargetRect.value : origin
  const exitProgress = Math.max(detailBackExitProgress.value, detailSourceExitProgress.value)
  const progress = detailSurfaceProgress.value
  const surfaceMargin = detailSurfaceMargin.value
  const expandedLeft = surfaceMargin
  const targetTop = detailExpandedTop.value
  const expandedWidth = Math.max(1, windowWidth.value - surfaceMargin * 2)
  const targetHeight = Math.max(1, windowHeight.value - targetTop - surfaceMargin)
  const draggingToList =
    readerBackDragging.value &&
    (detailBackExitProgress.value > 0 || detailSourceExitProgress.value > 0) &&
    !sourceReaderReturnTargetPending.value
  const committedListReturn = detailCommittedListReturn()
  const suppressSourceReturnPreview = sourceReaderBlockedBackSwipeActive.value

  if (!collapsedTarget) {
    const opacity =
      draggingToList
        ? 1
        : committedListReturn || detailReturningToFeed.value
          ? progress
          : 1 - exitProgress * 0.28
    return {
      width: cssPx(expandedWidth),
      height: cssPx(targetHeight),
      opacity: suppressSourceReturnPreview ? '0' : clamp(opacity).toFixed(3),
      transform: cssTranslate3d(expandedLeft, targetTop + exitProgress * 18),
      transition: readerBackDragging.value ? 'none' : undefined,
      borderRadius: cssPx(16 - exitProgress * 4),
    }
  }

  const width = collapsedTarget.width + (expandedWidth - collapsedTarget.width) * progress
  const height = collapsedTarget.height + (targetHeight - collapsedTarget.height) * progress
  const x = collapsedTarget.left + (expandedLeft - collapsedTarget.left) * progress
  const y = collapsedTarget.top + (targetTop - collapsedTarget.top) * progress
  const radius = 12 + 4 * progress
  const minimumSurfaceOpacity = darkTheme.value ? 0.64 : 0.36
  const opacity =
    draggingToList
      ? 1
      : committedListReturn || detailReturningToFeed.value
        ? progress
        : minimumSurfaceOpacity + progress * (1 - minimumSurfaceOpacity)

  return {
    width: cssPx(width),
    height: cssPx(height),
    opacity: suppressSourceReturnPreview ? '0' : clamp(opacity).toFixed(3),
    transform: cssTranslate3d(x, y),
    borderRadius: cssPx(radius),
    transition: readerBackDragging.value ? 'none' : undefined,
  }
})
const detailContentStyle = computed(() => {
  const progress = detailSurfaceProgress.value
  const sourceExitProgress = detailSourceExitProgress.value
  const committedListReturn = detailCommittedListReturn()
  const opacity = sourceExitProgress > 0 ? 1 : clamp((progress - 0.56) / 0.22)
  const bodyOpacity = sourceExitProgress > 0 ? clamp((0.72 - sourceExitProgress) / 0.32) : 1
  const inlineMetaOpacity = sourceExitProgress > 0 ? clamp((0.24 - sourceExitProgress) / 0.18) : 1
  return {
    opacity: committedListReturn ? '0' : opacity.toFixed(3),
    '--detail-body-opacity': bodyOpacity.toFixed(3),
    '--detail-inline-meta-opacity': inlineMetaOpacity.toFixed(3),
    '--detail-frame-min-height': cssPx(detailFrameMinHeight.value),
    '--detail-frame-content-height': cssPx(Math.max(detailFrameMinHeight.value, detailFrameContentHeight.value)),
    transform:
      sourceExitProgress > 0 ? cssTranslate3d(0, 0) : cssTranslate3d(0, (1 - progress) * 8),
    visibility: !committedListReturn && opacity > 0.01 ? ('visible' as const) : ('hidden' as const),
    transition: readerBackDragging.value || committedListReturn ? 'none' : undefined,
  }
})
const detailProgressStyle = computed(() => {
  const margin = detailSurfaceMargin.value
  const top = Math.max(margin, detailExpandedTop.value + margin)
  return {
    top: cssPx(top),
    right: cssPx(Math.max(6, margin * 0.5)),
    bottom: `${margin}px`,
    opacity: detailProgressVisible.value ? '1' : '0',
    pointerEvents: detailProgressVisible.value ? ('auto' as const) : ('none' as const),
    transition: detailProgressDragging.value || readerBackDragging.value ? 'none' : undefined,
  }
})
const detailProgressFillStyle = computed(() => ({
  height: `${(detailReadingProgress.value * 100).toFixed(2)}%`,
}))
const detailProgressThumbStyle = computed(() => {
  const progress = detailReadingProgress.value
  return {
    top: `${(progress * 100).toFixed(2)}%`,
    transform: `translate3d(0, ${(-progress * 42).toFixed(2)}px, 0)`,
  }
})
const detailMorphTextStyle = computed(() => {
  const progress = detailSurfaceProgress.value
  const committedListReturn = detailCommittedListReturn()
  const summaryOpacity = clamp((0.56 - progress) / 0.18)
  const textOpacity = clamp((0.74 - progress) / 0.18)
  return {
    opacity: committedListReturn ? '0' : '1',
    '--morph-title-size': `${(18 + progress * 10).toFixed(2)}px`,
    '--morph-text-opacity': textOpacity.toFixed(3),
    '--morph-summary-opacity': summaryOpacity.toFixed(3),
    '--morph-source-pointer-events': textOpacity > 0.12 ? 'auto' : 'none',
    transform: cssTranslate3d(0, progress * -4),
    transition: readerBackDragging.value || committedListReturn ? 'none' : undefined,
  }
})
const detailHeaderTitleStyle = computed(() => {
  const sourceListTitleProgress = detailSourceListTitleProgress.value
  const opacity =
    sourceListTitleProgress > 0
      ? sourceListTitleProgress
      : detailHeaderFeedTitleProgress.value * (1 - feedHeaderReturnProgress.value)
  return {
    opacity: opacity.toFixed(3),
    transform: cssTranslate3d(0, (1 - opacity) * 8),
    filter: `blur(${((1 - opacity) * 3.2).toFixed(2)}px)`,
    transition: readerBackDragging.value ? 'none' : undefined,
  }
})
const detailHeaderCurrentTextStyle = computed(() => {
  const progress = detailHeaderTitleSwapping.value ? detailHeaderSwapProgress.value : 1
  return {
    opacity: progress.toFixed(3),
    filter: `blur(${((1 - progress) * 2.8).toFixed(2)}px)`,
    transform: cssTranslate3d(0, (1 - progress) * 6),
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-normal) ease, filter var(--motion-normal) ease, transform var(--motion-normal) var(--ease-emphasized)',
  }
})
const detailHeaderPreviousTextStyle = computed(() => {
  const progress = detailHeaderSwapProgress.value
  return {
    opacity: (1 - progress).toFixed(3),
    filter: `blur(${(progress * 3.2).toFixed(2)}px)`,
    transform: cssTranslate3d(0, progress * -6),
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) ease, filter var(--motion-normal) ease, transform var(--motion-normal) var(--ease-emphasized)',
  }
})
const detailInlineSourceStyle = computed(() => {
  return {
    opacity: sourceNameMorphLabelOpacity.value.toFixed(3),
    filter: `blur(${sourceNameMorphLabelBlur.value.toFixed(2)}px)`,
    transform: 'translate3d(0, 0, 0)',
    transition: readerBackDragging.value ? 'none' : 'opacity var(--motion-short) ease, filter var(--motion-short) ease',
  }
})
const detailMorphSourceLabelStyle = computed(() => {
  return {
    opacity: sourceNameMorphLabelOpacity.value.toFixed(3),
    filter: `blur(${sourceNameMorphLabelBlur.value.toFixed(2)}px)`,
    transition: readerBackDragging.value ? 'none' : 'opacity var(--motion-short) ease, filter var(--motion-short) ease',
  }
})
const sourceTitleRevealVisible = computed(
  () => sourceTitleRevealReady.value && !sourcePullActive.value,
)
const sourceNameMorphStyle = computed(() => {
  const origin = detailSourceNameOriginRect.value
  const target = detailSourceNameTargetRect.value
  const progress = sourceNameMorphProgress.value
  if (!origin || !target) {
    return {
      opacity: '0',
      filter: 'blur(0)',
      transform: 'translate3d(0, 0, 0)',
    }
  }

  const left = origin.left + (target.left - origin.left) * progress
  const top = origin.top + (target.top - origin.top) * progress
  const width = Math.max(origin.width, target.width, origin.width + (target.width - origin.width) * progress) + 18
  const size = 13 + (12 - 13) * progress
  const fadeOut = clamp((progress - 0.62) / 0.28)
  const opacity = clamp(1 - fadeOut)
  const blur = Math.sin(progress * Math.PI) * 1.6 + fadeOut * 2.2

  return {
    left: cssPx(left),
    top: cssPx(top),
    width: cssPx(width),
    opacity: opacity.toFixed(3),
    fontSize: `${size.toFixed(2)}px`,
    filter: `blur(${blur.toFixed(2)}px)`,
    transform: 'translate3d(0, 0, 0)',
    transition: readerBackDragging.value
      ? 'none'
      : 'left var(--motion-reader) var(--ease-standard), top var(--motion-reader) var(--ease-standard), width var(--motion-reader) var(--ease-standard), font-size var(--motion-reader) var(--ease-standard), opacity var(--motion-quick) ease, filter var(--motion-quick) ease',
  }
})
const sourceTitleLayerStyle = computed(() => {
  const revealProgress = sourceTitleRevealVisible.value ? sourceTitleRevealProgress.value : 0
  const opacity = sourceTitleRevealVisible.value ? sourceTitleProgress.value * (1 - revealProgress) : 1

  return {
    opacity: opacity.toFixed(3),
    transform: 'translate3d(0, 0, 0)',
    filter: `blur(${(revealProgress * 2).toFixed(2)}px)`,
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) ease, filter var(--motion-short) ease, transform var(--motion-short) var(--ease-standard)',
  }
})
const sourceTitleTextStyle = computed(() => ({
  display: 'inline-block',
}))
const sourceTitleRevealStyle = computed(() => {
  const progress = sourceTitleRevealProgress.value
  const left = windowWidth.value <= 720 ? 72 : 80
  const right = windowWidth.value <= 720 ? 104 : 120
  const top = (feedHeaderHeight.value - 44) / 2
  return {
    top: cssPx(top),
    left: cssPx(left),
    width: `calc(100vw - ${left + right}px)`,
    opacity: progress.toFixed(3),
    transform: `${cssTranslate3d(0, (1 - progress) * 12)} scale(${(
      0.965 +
      progress * 0.035
    ).toFixed(3)})`,
    filter: `blur(${((1 - progress) * 2.4).toFixed(2)}px)`,
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-slow) ease, transform var(--motion-slow) var(--ease-emphasized), filter var(--motion-slow) ease',
  }
})
const mainStyle = computed(() => {
  const baseStyle = {
    '--feed-header-height': `${feedHeaderHeight.value}px`,
    '--feed-header-space': cssPx(feedContentSpace.value),
    '--detail-underlay-blur': `${(detailSurfaceProgress.value * 7).toFixed(2)}px`,
    '--detail-underlay-opacity': (1 - detailSurfaceProgress.value * 0.08).toFixed(3),
  }

  if (!isFeedRoute.value) {
    return baseStyle
  }

  return baseStyle
})
const headerClass = computed(() => ({
  'app-header--feed-inactive':
    feedHeaderProgress.value <= 0.01 && !feedChromeSettling.value && !feedTopPulling.value,
  'app-header--detail': headerDetailLayoutActive.value,
}))
const headerStyle = computed(() => {
  const progress = feedHeaderProgress.value
  return {
    opacity: progress.toFixed(3),
    pointerEvents: progress > 0.86 ? ('auto' as const) : ('none' as const),
    transform: cssTranslate3d(0, (progress - 1) * feedHeaderHeight.value),
  }
})
const navOpenButtonStyle = computed(() => {
  const progress = feedCornerHidden.value ? 0 : feedHeaderProgress.value
  const settling = feedChromeSettling.value || feedRefreshSettling.value
  return {
    top: cssPx((feedHeaderHeight.value - 44) / 2),
    opacity: progress.toFixed(3),
    pointerEvents: progress > 0.86 && !feedCornerHidden.value ? ('auto' as const) : ('none' as const),
    transform: `${cssTranslate3d(0, (progress - 1) * feedHeaderHeight.value)} scale(${(
      0.92 +
      progress * 0.08
    ).toFixed(3)})`,
    transition: settling
      ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) ease, visibility var(--motion-chrome) ease, border-color var(--motion-fast) ease, background var(--motion-fast) ease'
      : undefined,
    visibility: progress > 0.01 && !feedCornerHidden.value ? ('visible' as const) : ('hidden' as const),
  }
})
const detailHTML = computed(() => detailItem.value?.content_html || detailItem.value?.content_snippet || '')
const detailText = computed(() => detailItem.value?.content_text || detailItem.value?.summary || detailItem.value?.content_snippet || '')
const detailPreviewSummary = computed(
  () =>
    plainPreviewText(
      detailItem.value?.summary,
      detailItem.value?.content_snippet,
      detailItem.value?.content_text,
      detailItem.value?.content_html,
    ) || '暂无摘要。',
)
const detailDisplayDate = computed(() => formatItemDate(detailItem.value?.published_at || detailItem.value?.fetched_at))
const detailFrameBody = computed(() => {
  const source = detailHTML.value || `<p>${escapeHTML(detailText.value || '暂无正文。')}</p>`
  return sanitizeDetailHTML(source)
})
const detailSrcdoc = computed(() => {
  const body = detailFrameBody.value
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>
  :root { color-scheme: light dark; }
  * {
    scrollbar-width: none;
    -ms-overflow-style: none;
  }
  html {
    scrollbar-width: none;
    -ms-overflow-style: none;
    touch-action: pan-y;
  }
  body {
    margin: 0;
    padding: 0;
    background: transparent;
    color: #162033;
    font: 16px/1.72 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    overflow-wrap: anywhere;
    overflow: hidden;
    scrollbar-width: none;
    -ms-overflow-style: none;
    touch-action: pan-y;
  }
  #messagefeed-detail-body {
    display: flow-root;
    min-height: 1px;
    overflow-wrap: anywhere;
  }
  *::-webkit-scrollbar,
  html::-webkit-scrollbar,
  body::-webkit-scrollbar {
    width: 0;
    height: 0;
    display: none;
  }
  img, video, iframe { max-width: 100%; height: auto; }
  pre, code { white-space: pre-wrap; overflow-wrap: anywhere; }
  a { color: #1d4ed8; }
  blockquote { margin: 1em 0; padding-left: 1em; border-left: 3px solid #d1d5db; color: #4b5563; }
  @media (prefers-color-scheme: dark) {
    body { color: #dbe6f3; background: transparent; }
    a { color: #93c5fd; }
    blockquote { border-left-color: #475569; color: #a9b6c6; }
  }
</style>
</head>
<body>
<main id="messagefeed-detail-body">
${body}
</main>
<script>
(() => {
  let startX = 0;
  let startY = 0;
  let tracking = false;
  let intent = null;
  let metricsTicking = false;
  let resizeObserver = null;
  const preferTouchEvents = 'ontouchstart' in window || navigator.maxTouchPoints > 0;
  const post = (phase, touch) => {
    window.parent.postMessage({
      type: 'messagefeed-detail-gesture',
      phase,
      startX,
      startY,
      x: touch.clientX,
      y: touch.clientY,
      dx: touch.clientX - startX,
      dy: touch.clientY - startY,
      source: 'detail-frame'
    }, '*');
  };
  const scrollMetrics = () => {
    const doc = document.documentElement;
    const body = document.body;
    const content = document.getElementById('messagefeed-detail-body');
    const contentRect = content?.getBoundingClientRect();
    const scrollHeight = Math.max(
      doc.scrollHeight || 0,
      body.scrollHeight || 0,
      content?.scrollHeight || 0,
      contentRect ? Math.ceil(contentRect.height) : 0
    );
    const clientHeight = window.innerHeight || doc.clientHeight || body.clientHeight || 0;
    return {
      scrollTop: 0,
      scrollHeight,
      clientHeight
    };
  };
  const postScrollMetrics = () => {
    window.parent.postMessage({
      type: 'messagefeed-detail-scroll',
      ...scrollMetrics()
    }, '*');
  };
  const requestScrollMetrics = () => {
    if (metricsTicking) return;
    metricsTicking = true;
    requestAnimationFrame(() => {
      metricsTicking = false;
      postScrollMetrics();
    });
  };
  window.addEventListener('resize', () => requestAnimationFrame(postScrollMetrics), { passive: true });
  window.addEventListener('message', (event) => {
    if (event.data?.type !== 'messagefeed-detail-scroll-to') return;
    requestAnimationFrame(postScrollMetrics);
  });
  window.addEventListener('load', () => {
    requestAnimationFrame(() => {
      postScrollMetrics();
      if ('ResizeObserver' in window) {
        resizeObserver = new ResizeObserver(() => requestAnimationFrame(postScrollMetrics));
        const content = document.getElementById('messagefeed-detail-body');
        resizeObserver.observe(document.documentElement);
        resizeObserver.observe(document.body);
        if (content) resizeObserver.observe(content);
      }
      window.setTimeout(postScrollMetrics, 180);
      window.setTimeout(postScrollMetrics, 520);
    });
  });
  window.addEventListener('touchstart', (event) => {
    if (!preferTouchEvents) return;
    if (event.touches.length !== 1) return;
    startX = event.touches[0].clientX;
    startY = event.touches[0].clientY;
    tracking = true;
    intent = null;
    post('start', event.touches[0]);
  }, { passive: true, capture: true });
  window.addEventListener('touchmove', (event) => {
    if (!preferTouchEvents) return;
    if (!tracking || event.touches.length !== 1) return;
    const touch = event.touches[0];
    const dx = touch.clientX - startX;
    const dy = touch.clientY - startY;
    const absX = Math.abs(dx);
    const absY = Math.abs(dy);
    if (!intent) {
      if (absX > 3 && absX > absY * 0.52) {
        intent = 'horizontal';
      } else {
        post('move', touch);
        requestScrollMetrics();
        return;
      }
    }
    if (event.cancelable) {
      event.preventDefault();
    }
    post('move', touch);
  }, { passive: false, capture: true });
  window.addEventListener('touchcancel', (event) => {
    if (!preferTouchEvents) return;
    const touch = event.changedTouches[0];
    if (tracking && touch) post('cancel', touch);
    requestScrollMetrics();
    tracking = false;
    intent = null;
  }, { passive: true, capture: true });
  window.addEventListener('touchend', (event) => {
    if (!preferTouchEvents) return;
    const touch = event.changedTouches[0];
    if (!touch) return;
    if (tracking) post('end', touch);
    requestScrollMetrics();
    tracking = false;
    intent = null;
  }, { passive: true, capture: true });
  if (!preferTouchEvents && window.PointerEvent) {
    let pointerTracking = false;
    let pointerIntent = null;
    let pointerId = null;
    window.addEventListener('pointerdown', (event) => {
      if (event.pointerType !== 'touch' || !event.isPrimary) return;
      pointerId = event.pointerId;
      startX = event.clientX;
      startY = event.clientY;
      pointerTracking = true;
      pointerIntent = null;
      post('start', event);
    }, { passive: true, capture: true });
    window.addEventListener('pointermove', (event) => {
      if (!pointerTracking || event.pointerId !== pointerId || event.pointerType !== 'touch') return;
      const dx = event.clientX - startX;
      const dy = event.clientY - startY;
      const absX = Math.abs(dx);
      const absY = Math.abs(dy);
      if (!pointerIntent) {
        if (absX > 3 && absX > absY * 0.52) {
          pointerIntent = 'horizontal';
        } else {
          post('move', event);
          requestScrollMetrics();
          return;
        }
      }
      if (event.cancelable) {
        event.preventDefault();
      }
      post('move', event);
    }, { passive: false, capture: true });
    window.addEventListener('pointercancel', (event) => {
      if (pointerTracking && event.pointerId === pointerId) post('cancel', event);
      requestScrollMetrics();
      pointerTracking = false;
      pointerIntent = null;
      pointerId = null;
    }, { passive: true, capture: true });
    window.addEventListener('pointerup', (event) => {
      if (pointerTracking && event.pointerId === pointerId) post('end', event);
      requestScrollMetrics();
      pointerTracking = false;
      pointerIntent = null;
      pointerId = null;
    }, { passive: true, capture: true });
  }
})();
<\/script>
</body>
</html>`
})

const managementItems = [
  { key: 'sources', label: '订阅管理', path: '/sources', icon: IconBook },
]
const feedTabs = [
  { key: 'subscriptions', label: '订阅', path: '/subscriptions' },
  { key: 'recommendations', label: '推荐', path: '/recommendations' },
]

const navigationOpenDistance = 72
const viewSwitchDistance = 62
const directionLockRatio = 1.25
const navigationDragRatio = 1.1
const viewDirectionLockRatio = 1.35
const topPullDirectionLockRatio = 1.18
const viewDragThreshold = feedPagerTransition.dragThreshold
const viewSwipeChromeRevealDelay = 520
const topChromeSettleDuration = 1000
let touchStartX = 0
let touchStartY = 0
let touchStartNavigationProgress = 0
let activeNavigationPointerId: number | null = null
let activeFeedPointerId: number | null = null
let trackingEdgeSwipeCandidate = false
let trackingNavigationCloseCandidate = false
let trackingViewSwipeCandidate = false
let trackingEdgeSwipe = false
let trackingNavigationClose = false
let trackingViewSwipe = false
let navigationDragStarted = false
let removeSystemBackGuard: (() => void) | null = null
let lastHomeBackAttemptAt = 0
let lastScrollY = typeof window === 'undefined' ? 0 : window.scrollY
let lastFeedScrollTop = 0
let lastPageScrollTop = 0
let lastSourceReaderScrollTop = 0
let lastDetailScrollTop = 0
let topPullStartProgress = 1
let pageTouchStartX = 0
let pageTouchStartY = 0
let pageTopPullDistance = 0
let trackingPageTopPullCandidate = false
let trackingPageTopPull = false

function resetGestureTracking() {
  trackingEdgeSwipeCandidate = false
  trackingNavigationCloseCandidate = false
  trackingViewSwipeCandidate = false
  feedPagerTransition.clearStartedWithHiddenChrome()
  trackingEdgeSwipe = false
  trackingNavigationClose = false
  trackingViewSwipe = false
  navigationDragStarted = false
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

async function pushRoute(path: string) {
  programmaticRouteNavigation = true
  try {
    await router.push(path)
  } finally {
    window.setTimeout(() => {
      programmaticRouteNavigation = false
    }, 0)
  }
}

async function replaceRoute(path: string) {
  programmaticRouteNavigation = true
  try {
    await router.replace(path)
  } finally {
    window.setTimeout(() => {
      programmaticRouteNavigation = false
    }, 0)
  }
}

function scheduleReaderURLAndHistorySync(forcePush = false) {
  readerRouteSync.scheduleSync(forcePush)
}

function handleMenuClick(key: string) {
  const item = managementItems.find((navItem) => navItem.key === key)
  if (item) {
    void pushRoute(item.path)
    closeNavigation()
  }
}

function goHome(closePanel = navigationVisible.value) {
  void pushRoute('/recommendations')
  chromeState.setStableVisible()
  feedPagerTransition.reset()
  if (closePanel) {
    closeNavigation()
  }
}

function handleCornerButtonClick() {
  openNavigation()
}

function navigateTo(path: string) {
  feedPagerTransition.setSettling(true)
  feedPagerTransition.setDragOffset(0)
  void pushRoute(path)
  feedPagerTransition.clearDelayedCommitTimer()
  feedPagerTransition.scheduleSettlingEnd(motionDelay(260))
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(Math.max(value, min), max)
}

function setChromeProgress(progress: number, phase?: ChromePhase) {
  chromeState.setProgress(progress, phase)
}

function setChromeContentCollapsed(collapsed: boolean) {
  chromeState.setContentCollapsed(collapsed)
}

function setChromeSettling(settling: boolean, phase?: ChromePhase) {
  chromeState.setSettling(settling, phase)
}

function motionDelay(duration = readerMorphDuration) {
  return duration === readerMorphDuration ? readerMorphCleanupDelay : duration + readerMorphCleanupBuffer
}

function cssNumber(value: number, digits = 2) {
  return Number.isFinite(value) ? value.toFixed(digits) : '0.00'
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
}

function stretchTransformOrigin(stretch: number, anchor: 'left' | 'right' | null) {
  if (stretch > 0 || anchor === 'left') {
    return 'left center'
  }
  if (stretch < 0 || anchor === 'right') {
    return 'right center'
  }
  return undefined
}

function updateStretchAnchor(
  anchorRef: typeof detailStretchAnchor,
  stretch: number,
) {
  if (stretch > 0) {
    anchorRef.value = 'left'
  } else if (stretch < 0) {
    anchorRef.value = 'right'
  }
}

function clearStretchAnchors(delay = 280) {
  window.setTimeout(() => {
    if (!readerBackDragging.value && detailReaderStretch.value === 0) {
      detailStretchAnchor.value = null
    }
    if (!readerBackDragging.value && sourceReaderStretch.value === 0) {
      sourceStretchAnchor.value = null
    }
    pageContentMotion.clearStretchAnchorIfIdle(readerBackDragging.value)
  }, delay)
}

function cssRotate(degrees: number) {
  return `rotate(${cssNumber(degrees)}deg)`
}

function chromeLayerStyle(
  visible: boolean,
  progress: number,
  options: {
    shift?: number
    scaleStart?: number
    disableTransition?: boolean
    pointerEnabled?: boolean
  } = {},
) {
  const safeProgress = clamp(visible ? progress : 0)
  const shift = options.shift ?? -8
  const scaleStart = options.scaleStart ?? 0.96
  const pointerEnabled = options.pointerEnabled ?? true
  return {
    opacity: safeProgress.toFixed(3),
    pointerEvents: safeProgress > 0.86 && pointerEnabled ? ('auto' as const) : ('none' as const),
    transform: `${cssTranslate3d(0, (1 - safeProgress) * shift)} scale(${(
      scaleStart +
      safeProgress * (1 - scaleStart)
    ).toFixed(3)})`,
    transition: options.disableTransition
      ? 'none'
      : feedChromeSettling.value || feedRefreshSettling.value
        ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) ease, visibility var(--motion-chrome) ease'
        : undefined,
    visibility: safeProgress > 0.01 ? ('visible' as const) : ('hidden' as const),
  }
}

function escapeHTML(value: string) {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;')
}

function plainPreviewText(...values: Array<string | undefined>) {
  const value = values.find((item) => item?.trim())
  if (!value) {
    return ''
  }

  const input = value.trim()
  if (typeof DOMParser === 'undefined') {
    return input.replace(/\s+/g, ' ')
  }

  const documentFragment = new DOMParser().parseFromString(input, 'text/html')
  return (documentFragment.body.textContent || '').replace(/\s+/g, ' ').trim()
}

function sanitizeDetailHTML(value: string) {
  const input = value.trim()
  if (!input || typeof DOMParser === 'undefined') {
    return input
      .replace(/<script[\s\S]*?<\/script>/gi, '')
      .replace(/<style[\s\S]*?<\/style>/gi, '')
      .replace(/<\/?(?:html|head|body)[^>]*>/gi, '')
  }

  const documentFragment = new DOMParser().parseFromString(input, 'text/html')
  documentFragment
    .querySelectorAll('script, style, link, meta, base, title, noscript, object, embed')
    .forEach((element) => element.remove())
  documentFragment.body.querySelectorAll('*').forEach((element) => {
    for (const attribute of Array.from(element.attributes)) {
      const name = attribute.name.toLowerCase()
      const attributeValue = attribute.value.trim().toLowerCase()
      if (
        name.startsWith('on') ||
        ((name === 'href' || name === 'src') && attributeValue.startsWith('javascript:'))
      ) {
        element.removeAttribute(attribute.name)
      }
    }
  })
  return documentFragment.body.innerHTML || input
}

function formatItemDate(value?: string) {
  if (!value) {
    return '时间未知'
  }
  return new Intl.DateTimeFormat('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(value))
}

function snapshotRect(rect?: DOMRect): RectSnapshot | null {
  if (!rect || rect.width <= 0 || rect.height <= 0) {
    return null
  }
  return {
    left: Math.max(0, rect.left),
    top: Math.max(0, rect.top),
    width: Math.max(1, rect.width),
    height: Math.max(1, rect.height),
  }
}

function snapshotElementRect(element: Element | null) {
  return element instanceof HTMLElement ? snapshotRect(element.getBoundingClientRect()) : null
}

function setSourceReaderContentElement(element: HTMLElement | null) {
  sourceReaderContentRef.value = element
}

function setPageContentElement(element: HTMLElement | null) {
  pageContentRef.value = element
}

function setPageViewInstance(view: PageViewExpose | null) {
  pageViewRef.value = view
}

function findSourceFeedItemElement(itemID?: number) {
  if (!itemID || !sourceReaderContentRef.value) {
    return null
  }
  return sourceReaderContentRef.value.querySelector(`[data-feed-item-id="${itemID}"]`)
}

function findFeedItemElement(itemID?: number) {
  if (!itemID || !feedContentRef.value) {
    return null
  }

  const activePane = feedContentRef.value.querySelectorAll('.feed-pane').item(activeFeedIndex.value)
  return (
    activePane?.querySelector(`[data-feed-item-id="${itemID}"]`) ??
    feedContentRef.value.querySelector(`[data-feed-item-id="${itemID}"]`)
  )
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
  detailContentRef.value = element
}

function setDetailInlineSourceElement(element: HTMLElement | null) {
  detailInlineSourceRef.value = element
}

function setDetailFrameElement(element: HTMLIFrameElement | null) {
  detailFrameRef.value = element
}

function restoreMorphingItemContent(unlockDelay = 180) {
  restoreMorphingItemContentWithDelay(unlockDelay)
}

function scheduleHiddenSourceReaderCleanup(delay = 180) {
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

function settleReaderMotion(duration = 260, done?: () => void) {
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
  if (!readerSessionInitialized && !readerSession.restoring.value) {
    return
  }
  readerSession.saveNow()
}

function scheduleReaderSessionSave() {
  if (!readerSessionInitialized && !readerSession.restoring.value) {
    return
  }
  readerSession.scheduleSave()
}

function restoreSavedScrollPositions(snapshot: ReaderSessionSnapshot) {
  const apply = () => {
    if (feedContentRef.value) {
      feedContentRef.value.scrollTop = snapshot.feedScrollTop
    }
    if (sourceReaderContentRef.value) {
      sourceReaderContentRef.value.scrollTop = snapshot.sourceReaderScrollTop
    }
    if (detailContentRef.value) {
      detailContentRef.value.scrollTop = snapshot.detailScrollTop
      syncDetailContainerMetrics()
    }
  }

  nextTick(() => {
    apply()
    window.setTimeout(apply, 120)
    window.setTimeout(apply, 520)
  })
}

function applyReaderSessionSnapshot(snapshot: ReaderSessionSnapshot) {
  feedScrollTop.value = snapshot.feedScrollTop || 0
  setChromeProgress(typeof snapshot.topChromeProgress === 'number' ? snapshot.topChromeProgress : 1)
  setChromeContentCollapsed(Boolean(snapshot.feedContentCollapsed))
  applyReaderStackSessionSnapshot(snapshot, {
    onSourceScrollTop: (scrollTop) => {
      lastSourceReaderScrollTop = scrollTop
    },
    onDetailScrollTop: (scrollTop) => {
      lastDetailScrollTop = scrollTop
    },
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
    lastHomeBackAttemptAt = 0
    closeNavigation()
    return true
  }

  if (detailReaderOpen.value && detailOpenedFromSourceReader.value && !detailCommittedListReturn()) {
    lastHomeBackAttemptAt = 0
    collapseItemReader()
    return true
  }

  if (sourceReaderShouldReturnToDetail()) {
    lastHomeBackAttemptAt = 0
    restoreSourceReaderBackTarget()
    return true
  }

  if (sourceReaderOpen.value && !detailReaderOpen.value) {
    lastHomeBackAttemptAt = 0
    closeSourceReader()
    return true
  }

  if (detailReaderOpen.value) {
    lastHomeBackAttemptAt = 0
    collapseItemReader()
    return true
  }

  if (!isFeedRoute.value && !navigationVisible.value) {
    lastHomeBackAttemptAt = 0
    goHome(false)
    return true
  }

  if (isFeedRoute.value) {
    const now = Date.now()
    if (now - lastHomeBackAttemptAt <= homeExitDoubleBackMs) {
      lastHomeBackAttemptAt = 0
      return false
    }
    lastHomeBackAttemptAt = now
    return true
  }

  return false
}

function restoreSourceReaderBackTarget() {
  const result = restoreSourceReaderBackTargetState({
    onDetailScrollTop: (scrollTop) => {
      lastDetailScrollTop = scrollTop
    },
  })
  if (result.action === 'restore-detail') {
    restoreDetailFromParkedSource()
    return
  }

  closeSourceReader()
}

function restoreParkedDetailSnapshot(snapshot: ParkedDetailSnapshot | null) {
  return restoreReaderStackParkedDetailSnapshot(snapshot, {
    onDetailScrollTop: (scrollTop) => {
      lastDetailScrollTop = scrollTop
    },
  })
}

function restorePreviousParkedDetail() {
  return restoreReaderStackPreviousParkedDetail({
    onDetailScrollTop: (scrollTop) => {
      lastDetailScrollTop = scrollTop
    },
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
    lastSourceReaderScrollTop = 0
  }
  nextTick(() => {
    if (result.resetScroll && sourceReaderContentRef.value) {
      sourceReaderContentRef.value.scrollTop = 0
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
    headerSwapDelay: motionDelay(320),
    detailEntryDelay: motionDelay(readerMorphDuration),
    afterBegin: () => {
      chromeState.setStableVisible()
      feedTopPulling.value = false
      lastDetailScrollTop = 0
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
      if (detailContentRef.value) {
        detailContentRef.value.scrollTop = 0
      }
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
      onDetailScrollTop: (scrollTop) => {
        lastDetailScrollTop = scrollTop
      },
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

function restoreDetailFromParkedSource(duration = 360) {
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

function restoreParkedSourceReader(duration = 260) {
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

function collapseItemReader(duration = 360) {
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

function restoreItemReaderExpansion(duration = 360) {
  restoreItemReaderExpansionWithDelay(motionDelay(duration))
}

function restoreDetailFromSourceSwipe(duration = 360) {
  restoreDetailFromSourceSwipeWithDelay(motionDelay(duration))
}

function completeDetailToSourceReader(duration = 360) {
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

function settleNavigation(open: boolean) {
  navigationDrawer.settle(open)
}

function openNavigation() {
  resetGestureTracking()
  navigationDrawer.openPanel()
}

function closeNavigation() {
  navigationDrawer.closePanel()
}

function isHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > Math.abs(deltaY) * directionLockRatio
}

function isViewHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
}

function isNavigationDrag(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > 8 && Math.abs(deltaX) > Math.abs(deltaY) * navigationDragRatio
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

function scheduleSwipeTransitionReset(duration = 260) {
  swipeTransition.scheduleReset(motionDelay(duration))
}

function beginViewSwipeTransition(offset: number) {
  swipeTransition.begin({
    from: feedPagerTransition.activeSurface.value,
    to: feedPagerTransition.surfaceFromOffset(offset),
    direction: offset < 0 ? 'left' : 'right',
    progress: viewSwipeProgress.value,
  })
}

function syncViewSwipeTransition(offset: number) {
  const targetSurface = feedPagerTransition.surfaceFromOffset(offset)
  swipeTransition.update({
    to: targetSurface,
    direction: offset < 0 ? 'left' : 'right',
    progress: viewSwipeProgress.value,
    isBlocked: targetSurface === null,
  })
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

function isBackHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > viewDragThreshold && Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
}

function showTopChromeForSourceReturn() {
  if (topChromeProgress.value < 0.99 || feedContentCollapsed.value) {
    setTopChromeVisible(true)
  }
}

function settleSourceContentAfterRefresh() {
  sourceContentMotion.settleAfterRefresh(topChromeSettleDuration)
}

function prepareSourceReaderReturnDrag() {
  const ready = prepareSourceReaderReturnDragState({
    onDetailScrollTop: (scrollTop) => {
      lastDetailScrollTop = scrollTop
    },
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
  touchStartX = startX
  touchStartY = startY
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
  trackingEdgeSwipeCandidate = false
  trackingNavigationCloseCandidate = false
  trackingViewSwipeCandidate = false
  return true
}

function updateBackSwipe(deltaX: number, deltaY: number, fromDetailFrame = false, currentX = touchStartX + deltaX) {
  beginBackSwipeIfAllowed(deltaX, deltaY, fromDetailFrame)

  if (!readerBackSwipeTrackingActive.value) {
    return false
  }

  suppressFollowingClick()
  updateReaderBackSwipeDragState(
    deltaX,
    { currentX, startX: touchStartX, width: windowWidth.value },
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

function readerBackSwipeActionHandlers(): Parameters<typeof applyReaderBackSwipeAction>[1] {
  return {
    restoreItemExpansion: restoreItemReaderExpansion,
    restoreDetailFromSourceSwipe: restoreDetailFromSourceSwipe,
    restoreParkedSource: restoreParkedSourceReader,
    completeDetailToSource: completeDetailToSourceReader,
    collapseDetail: collapseItemReader,
    restoreDetailFromParkedSource: restoreDetailFromParkedSource,
    reset: resetBackSwipeOffset,
  }
}

function finishBackSwipe(deltaX: number, _deltaY: number) {
  const result = readerBackSwipeFinishResult(deltaX, viewSwitchDistance, pageSideStretch.value)

  swipeTransition.settle(result.committed, {
    progress: result.progress,
    isBlocked: result.isBlocked,
  })
  scheduleSwipeTransitionReset(360)

  if (result.committed) {
    suppressFollowingClick()
  }
  applyReaderBackSwipeAction(result.action, readerBackSwipeActionHandlers())
}

function cancelBackSwipe() {
  const result = readerBackSwipeCancelResult(pageSideStretch.value)

  swipeTransition.settle(false, {
    progress: result.progress,
    isBlocked: result.isBlocked,
  })
  scheduleSwipeTransitionReset(360)
  applyReaderBackSwipeAction(result.action, readerBackSwipeActionHandlers())
}

function finishViewSwipe(nextPath: string | null) {
  const committed = Boolean(nextPath)
  feedPagerTransition.setSettling(true)
  swipeTransition.settle(committed, { progress: committed ? 1 : 0, isBlocked: false })
  feedPagerTransition.clearDelayedCommitTimer()
  feedPagerTransition.clearSettlingTimer()
  const startedWithHiddenChrome = feedPagerTransition.consumeStartedWithHiddenChrome()
  const shouldRevealChromeFirst = Boolean(nextPath) && startedWithHiddenChrome
  if (shouldRevealChromeFirst) {
    setTopChromeVisible(true)
    feedPagerTransition.scheduleDelayedCommit(viewSwipeChromeRevealDelay, () => {
      if (nextPath) {
        void pushRoute(nextPath)
      }
      feedPagerTransition.setDragOffset(0)
      feedPagerTransition.scheduleSettlingEnd(motionDelay(260))
    })
    scheduleSwipeTransitionReset(viewSwipeChromeRevealDelay + 260)
    return
  }
  if (nextPath) {
    void pushRoute(nextPath)
  }
  feedPagerTransition.setDragOffset(0)
  feedPagerTransition.scheduleSettlingEnd(motionDelay(260))
  scheduleSwipeTransitionReset(260)
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
  touchStartX = touch.clientX
  touchStartY = touch.clientY
  touchStartNavigationProgress = navigationProgress.value

  if (navigationVisible.value) {
    trackingNavigationCloseCandidate = navigationOpen.value
    trackingEdgeSwipeCandidate = false
    trackingViewSwipeCandidate = false
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

  trackingEdgeSwipeCandidate = canStartNavigationOpen(touchStartX)
  trackingNavigationCloseCandidate = navigationOpen.value
  trackingViewSwipeCandidate = canStartViewSwipe(touchStartX)
}

function handleTouchMove(event: TouchEvent) {
  if (
    !trackingEdgeSwipeCandidate &&
    !trackingNavigationCloseCandidate &&
    !trackingViewSwipeCandidate &&
    !readerBackSwipeCandidateActive.value &&
    !trackingEdgeSwipe &&
    !trackingNavigationClose &&
    !trackingViewSwipe &&
    !readerBackSwipeTrackingActive.value
  ) {
    return
  }

  const touch = event.touches[0]
  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY
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

  if (trackingEdgeSwipeCandidate && deltaX > 8 && isNavigationDrag(deltaX, deltaY)) {
    trackingEdgeSwipe = true
    trackingEdgeSwipeCandidate = false
    trackingViewSwipeCandidate = false
    trackingNavigationCloseCandidate = false
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingNavigationCloseCandidate && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    trackingNavigationClose = true
    trackingNavigationCloseCandidate = false
    trackingViewSwipeCandidate = false
    trackingEdgeSwipeCandidate = false
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingEdgeSwipe || trackingNavigationClose || trackingViewSwipe) {
    event.preventDefault()
  }

  if (trackingEdgeSwipe) {
    navigationProgress.value = clamp(deltaX / navigationWidth.value)
    return
  }

  if (trackingNavigationClose) {
    navigationProgress.value = clamp(touchStartNavigationProgress + deltaX / navigationWidth.value)
    return
  }

  if (trackingViewSwipeCandidate && viewHorizontal) {
    if (feedPagerTransition.canStartDrag(deltaX)) {
      trackingViewSwipe = true
      trackingViewSwipeCandidate = false
      trackingEdgeSwipeCandidate = false
      trackingNavigationCloseCandidate = false
      showTopChromeForViewSwipe()
      beginViewSwipeTransition(deltaX)
    } else {
      return
    }
  }

  if (trackingViewSwipe) {
    trackingViewSwipe = true
    feedPagerTransition.setDragDelta(deltaX)
    syncViewSwipeTransition(viewDragOffset.value)
    return
  }
}

function handleWindowPointerDown(event: PointerEvent) {
  if (event.pointerType === 'mouse' || event.isPrimary === false) {
    return
  }

  finishCommittedListReturnForGesture()

  touchStartX = event.clientX
  touchStartY = event.clientY
  touchStartNavigationProgress = navigationProgress.value
  trackingEdgeSwipeCandidate = canStartNavigationOpen(touchStartX)
  trackingNavigationCloseCandidate = navigationOpen.value
  trackingViewSwipeCandidate = false
  activeNavigationPointerId =
    trackingEdgeSwipeCandidate || trackingNavigationCloseCandidate ? event.pointerId : null
}

function handleWindowPointerMove(event: PointerEvent) {
  if (activeNavigationPointerId !== event.pointerId) {
    return
  }

  const deltaX = event.clientX - touchStartX
  const deltaY = event.clientY - touchStartY

  if (trackingEdgeSwipeCandidate && deltaX > 8 && isNavigationDrag(deltaX, deltaY)) {
    trackingEdgeSwipe = true
    trackingEdgeSwipeCandidate = false
    trackingNavigationCloseCandidate = false
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingNavigationCloseCandidate && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    trackingNavigationClose = true
    trackingNavigationCloseCandidate = false
    trackingEdgeSwipeCandidate = false
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingEdgeSwipe || trackingNavigationClose) {
    event.preventDefault()
  }

  if (trackingEdgeSwipe) {
    navigationProgress.value = clamp(deltaX / navigationWidth.value)
  } else if (trackingNavigationClose) {
    navigationProgress.value = clamp(touchStartNavigationProgress + deltaX / navigationWidth.value)
  }
}

function handleWindowPointerUp(event: PointerEvent) {
  if (activeNavigationPointerId !== event.pointerId) {
    return
  }

  const deltaX = event.clientX - touchStartX
  const deltaY = event.clientY - touchStartY
  const horizontal = trackingViewSwipe ? isViewHorizontalSwipe(deltaX, deltaY) : isHorizontalSwipe(deltaX, deltaY)

  if (trackingEdgeSwipe && navigationDragStarted) {
    settleNavigation(horizontal && (deltaX >= navigationOpenDistance || navigationProgress.value >= 0.42))
  }

  if (trackingNavigationClose && navigationDragStarted) {
    settleNavigation(!(horizontal && (deltaX <= -navigationOpenDistance || navigationProgress.value <= 0.58)))
  }

  activeNavigationPointerId = null
  resetGestureTracking()
}

function handleWindowPointerCancel(event: PointerEvent) {
  if (activeNavigationPointerId !== event.pointerId) {
    return
  }

  activeNavigationPointerId = null
  const hadNavigationGesture = trackingEdgeSwipe || trackingNavigationClose
  resetGestureTracking()
  if (hadNavigationGesture) {
    settleNavigation(navigationProgress.value >= 0.42)
  }
}

function handleTouchEnd(event: TouchEvent) {
  if (
    !trackingEdgeSwipeCandidate &&
    !trackingNavigationCloseCandidate &&
    !trackingViewSwipeCandidate &&
    !readerBackSwipeCandidateActive.value &&
    !trackingEdgeSwipe &&
    !trackingNavigationClose &&
    !trackingViewSwipe &&
    !readerBackSwipeTrackingActive.value
  ) {
    return
  }

  const touch = event.changedTouches[0]
  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY
  const horizontal = isHorizontalSwipe(deltaX, deltaY)

  if (readerBackSwipeTrackingActive.value) {
    finishBackSwipe(deltaX, deltaY)
    resetGestureTracking()
    return
  }

  if (!trackingEdgeSwipe && !trackingNavigationClose && !trackingViewSwipe) {
    resetGestureTracking()
    return
  }

  if (trackingEdgeSwipe) {
    if (navigationDragStarted) {
      settleNavigation(horizontal && (deltaX >= navigationOpenDistance || navigationProgress.value >= 0.42))
    }
  }

  if (trackingNavigationClose) {
    if (navigationDragStarted) {
      settleNavigation(!(horizontal && (deltaX <= -navigationOpenDistance || navigationProgress.value <= 0.58)))
    }
  }

  if (trackingViewSwipe) {
    suppressFollowingClick()
    finishViewSwipe(feedPagerTransition.commitPath(deltaX, horizontal, viewSwitchDistance))
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

  touchStartX = event.clientX
  touchStartY = event.clientY
  touchStartNavigationProgress = navigationProgress.value
  trackingViewSwipeCandidate = true
  trackingViewSwipe = false
  trackingEdgeSwipe = false
  trackingNavigationClose = false
  activeFeedPointerId = event.pointerId
  feedPagerTransition.setSettling(false)
}

function handleFeedPointerMove(event: PointerEvent) {
  if (activeFeedPointerId !== event.pointerId || event.pointerType === 'mouse') {
    return
  }

  const deltaX = event.clientX - touchStartX
  const deltaY = event.clientY - touchStartY

  if (trackingViewSwipeCandidate && !trackingViewSwipe) {
    if (!isViewHorizontalSwipe(deltaX, deltaY)) {
      return
    }

    if (feedPagerTransition.isBlockedDragDirection(deltaX)) {
      activeFeedPointerId = null
      trackingViewSwipeCandidate = false
      return
    }

    if (feedPagerTransition.canStartDrag(deltaX)) {
      trackingViewSwipe = true
      suppressFollowingClick()
      trackingViewSwipeCandidate = false
      showTopChromeForViewSwipe()
      beginViewSwipeTransition(deltaX)
      ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
    } else {
      return
    }
  }

  if (!trackingViewSwipe || !isViewHorizontalSwipe(deltaX, deltaY)) {
    return
  }

  if (feedPagerTransition.isBlockedDragDirection(deltaX)) {
    feedPagerTransition.setDragOffset(0)
    return
  }

  feedPagerTransition.setDragDelta(deltaX)
  syncViewSwipeTransition(viewDragOffset.value)
}

function handleFeedPointerUp(event: PointerEvent) {
  if (activeFeedPointerId !== event.pointerId || event.pointerType === 'mouse') {
    return
  }

  const deltaX = event.clientX - touchStartX
  const deltaY = event.clientY - touchStartY
  const horizontal = isViewHorizontalSwipe(deltaX, deltaY)

  const nextPath = trackingViewSwipe ? feedPagerTransition.commitPath(deltaX, horizontal, viewSwitchDistance) : null
  if (nextPath) {
    suppressFollowingClick()
    finishViewSwipe(nextPath)
  } else {
    finishViewSwipe(null)
  }

  trackingViewSwipe = false
  trackingViewSwipeCandidate = false
  activeFeedPointerId = null
}

function handleFeedPointerCancel() {
  trackingViewSwipe = false
  trackingViewSwipeCandidate = false
  activeFeedPointerId = null
  finishViewSwipe(null)
}

function handleTouchCancel() {
  const hadNavigationGesture = trackingEdgeSwipe || trackingNavigationClose
  const hadViewGesture = trackingViewSwipe
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
  activeFeedPointerId = null
}

function toggleTheme() {
  darkTheme.value = !darkTheme.value
  if (!darkTheme.value) {
    document.body.removeAttribute('arco-theme')
    localStorage.setItem('messagefeed-theme', 'light')
    return
  }
  document.body.setAttribute('arco-theme', 'dark')
  localStorage.setItem('messagefeed-theme', 'dark')
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    closeNavigation()
  }
}

function handleResize() {
  windowWidth.value = window.innerWidth
  windowHeight.value = window.innerHeight
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
  lastDetailScrollTop = nextScrollTop
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
    if (detailScrollTop.value > 0 && detailContentRef.value) {
      detailContentRef.value.scrollTop = detailScrollTop.value
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
  sourceTimelinePreloadEnabled.value = localStorage.getItem('messagefeed-source-preload') !== 'false'
}

function handleReaderSettingsChanged(event: Event) {
  const detail = (event as CustomEvent<{ sourceTimelinePreload?: boolean }>).detail
  if (typeof detail?.sourceTimelinePreload === 'boolean') {
    sourceTimelinePreloadEnabled.value = detail.sourceTimelinePreload
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

  return pageContentRef.value?.scrollTop ?? 0
}

function handleFeedTopPullStart(startedWithVisibleChrome = false) {
  if (isFeedRoute.value && feedInteraction.pullRefreshing) {
    return
  }

  feedTopPulling.value = true
  feedTopPullStartedWithChrome.value = startedWithVisibleChrome || feedTopChromeIsVisiblyOpen.value
  setChromeSettling(false, 'refreshing')
  chromeState.clearSettlingTimer()
  topPullStartProgress = topChromeProgress.value
}

function handleFeedTopPullMove(distance: number) {
  if (!feedTopPulling.value || (isFeedRoute.value && feedInteraction.pullRefreshing)) {
    return
  }

  if (!feedTopPullStartedWithChrome.value && feedTopChromeIsVisiblyOpen.value) {
    feedTopPullStartedWithChrome.value = true
  }

  if (!feedTopPullStartedWithChrome.value && currentContentScrollTop() > 0) {
    return
  }

  if (feedTopPullStartedWithChrome.value) {
    setChromeProgress(1, 'refreshing')
    setChromeContentCollapsed(false)
    return
  }

  setChromeProgress(clamp(topPullStartProgress - distance / feedHeaderHeight.value), 'refreshing')
}

function handleFeedTopPullEnd(shouldRefresh = false) {
  if (!feedTopPulling.value) {
    feedTopPullStartedWithChrome.value = false
    return
  }

  const startedWithChrome = feedTopPullStartedWithChrome.value
  feedTopPulling.value = false

  if (shouldRefresh) {
    refreshStartedWithChrome.value = startedWithChrome
    setChromeContentCollapsed(!startedWithChrome)
    if (startedWithChrome) {
      setChromeProgress(1, 'refreshing')
    }
    return
  }

  if (topChromeProgress.value <= 0.04) {
    setChromeContentCollapsed(true)
    setTopChromeVisible(false)
    feedTopPullStartedWithChrome.value = false
    return
  }

  setTopChromeVisible(true)
  feedTopPullStartedWithChrome.value = false
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

function handleScroll() {
  lastScrollY = window.scrollY
}

function handleFeedContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  feedScrollTop.value = current
  updateTopTabsByScroll(current, lastFeedScrollTop)
  lastFeedScrollTop = current
  scheduleReaderSessionSave()
}

function handlePageContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  updateTopTabsByScroll(current, lastPageScrollTop)
  lastPageScrollTop = current
  scheduleReaderSessionSave()
}

function handleSourceReaderScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  updateSourceReaderScrollTopState(current)
  updateTopTabsByScroll(current, lastSourceReaderScrollTop)
  lastSourceReaderScrollTop = current
  scheduleReaderSessionSave()
}

function handleDetailContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  updateDetailScrollMetricsState(current, target.scrollHeight, target.clientHeight)
  updateTopTabsByScroll(current, lastDetailScrollTop)
  lastDetailScrollTop = current
  scheduleReaderSessionSave()
}

function resetPageTopPullTracking() {
  pageTopPullDistance = 0
  pagePullRefresh.setDistance(pagePullRefreshing.value ? pageRefreshThreshold : 0)
  trackingPageTopPullCandidate = false
  trackingPageTopPull = false
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
  pageTouchStartX = touch.clientX
  pageTouchStartY = touch.clientY
  pageTopPullDistance = 0
  trackingPageTopPullCandidate = true
  trackingPageTopPull = false
}

function handlePageTouchMove(event: TouchEvent) {
  if (
    isFeedRoute.value ||
    event.touches.length !== 1 ||
    currentContentScrollTop() > 0 ||
    (!trackingPageTopPullCandidate && !trackingPageTopPull)
  ) {
    return
  }

  const touch = event.touches[0]
  const deltaX = touch.clientX - pageTouchStartX
  const deltaY = touch.clientY - pageTouchStartY

  if (!trackingPageTopPull) {
    if (deltaY <= 0 || Math.abs(deltaX) > Math.abs(deltaY) * topPullDirectionLockRatio) {
      resetPageTopPullTracking()
      return
    }

    if (deltaY < 2 || Math.abs(deltaY) <= Math.abs(deltaX) * topPullDirectionLockRatio) {
      return
    }

    trackingPageTopPull = true
    trackingPageTopPullCandidate = false
    setTopChromeVisible(true)
  }

  if (trackingPageTopPull) {
    event.preventDefault()
    pageTopPullDistance = Math.max(pageTopPullDistance, deltaY)
    pagePullRefresh.setDistance(pageTopPullDistance)
    pagePullRefresh.setSettling(false)
    pagePullRefresh.clearSettleTimer()
    pagePullRefresh.setOffset(pageRubberBandOffset(deltaY))
  }
}

async function refreshCurrentPageFromPull() {
  const refreshPage = pageViewRef.value?.refreshPage
  if (!refreshPage || pagePullRefreshing.value) {
    return
  }

  pagePullRefresh.setRefreshing(true)
  pagePullRefresh.setDistance(pageRefreshThreshold)
  try {
    await refreshPage({ noticeDelayMS: 180, suppressStartNotice: true })
  } finally {
    pagePullRefresh.setRefreshing(false)
    pagePullRefresh.setDistance(0)
    setChromeContentCollapsed(true)
    setTopChromeVisible(false)
  }
}

function handlePageTouchEnd() {
  if (trackingPageTopPull) {
    const shouldRefresh = pageTopPullDistance >= pageRefreshThreshold
    feedTopPulling.value = false
    setTopChromeVisible(true)
    settlePagePullOffset()
    if (shouldRefresh) {
      void refreshCurrentPageFromPull()
    }
  } else if (trackingPageTopPullCandidate) {
    feedTopPulling.value = false
  }
  resetPageTopPullTracking()
}

function handlePageTouchCancel() {
  if (trackingPageTopPull || trackingPageTopPullCandidate) {
    feedTopPulling.value = false
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
    feedTopPulling.value = false
    pagePullRefresh.setOffset(0)
    pagePullRefresh.setDistance(0)
    pagePullRefresh.setSettling(false)
    feedPagerTransition.setDragOffset(0)
    if (isFeedRoute.value) {
      setTopChromeVisible(true)
      nextTick(() => {
        const current = feedContentRef.value?.scrollTop ?? 0
        feedScrollTop.value = current
        lastFeedScrollTop = current
      })
    } else {
      setTopChromeVisible(true)
      nextTick(() => {
        lastPageScrollTop = pageContentRef.value?.scrollTop ?? 0
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
      feedTopPullStartedWithChrome.value = false
      setChromeContentCollapsed(true)
      setTopChromeVisible(false)
    }

    if (!active && !refreshCompletion.wasActive.value) {
      refreshCompletion.resetInactive()
      feedTopPullStartedWithChrome.value = false
    }
  },
)

onMounted(() => {
  loadReaderSettings()
  if (localStorage.getItem('messagefeed-theme') === 'dark') {
    document.body.setAttribute('arco-theme', 'dark')
    darkTheme.value = true
  }
  removeSystemBackGuard = virtualBackGuard.installRouterGuard()
  void router.isReady().then(() => restoreReaderSession()).finally(() => {
    readerSessionInitialized = true
    scheduleReaderURLAndHistorySync()
  })
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', handleResize)
  window.addEventListener('message', handleMessage)
  window.addEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
  window.addEventListener('popstate', virtualBackGuard.handlePopState)
  window.addEventListener('beforeunload', saveReaderSessionNow)
  window.addEventListener('scroll', handleScroll, { passive: true })
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
  removeSystemBackGuard?.()
  removeSystemBackGuard = null
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('message', handleMessage)
  window.removeEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
  window.removeEventListener('popstate', virtualBackGuard.handlePopState)
  window.removeEventListener('beforeunload', saveReaderSessionNow)
  window.removeEventListener('scroll', handleScroll)
  window.removeEventListener('pointerdown', handleWindowPointerDown)
  window.removeEventListener('pointermove', handleWindowPointerMove)
  window.removeEventListener('pointerup', handleWindowPointerUp)
  window.removeEventListener('pointercancel', handleWindowPointerCancel)
  window.removeEventListener('touchstart', handleTouchStart)
  window.removeEventListener('touchmove', handleTouchMove)
  window.removeEventListener('touchend', handleTouchEnd)
  window.removeEventListener('touchcancel', handleTouchCancel)
  feedPagerTransition.clearDelayedCommitTimer()
  feedPagerTransition.clearSettlingTimer()
  swipeTransition.clearResetTimer()
  navigationDrawer.clearTimer()
  refreshCompletion.clearTimer()
  chromeState.clearSettlingTimer()
  sourceContentMotion.clearTimer()
  pagePullRefresh.clearSettleTimer()
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
        ref="feedContentRef"
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
