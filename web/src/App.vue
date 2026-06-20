<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  IconBook,
  IconMenuUnfold,
  IconMoonFill,
  IconSettings,
  IconSunFill,
  IconSync,
} from '@arco-design/web-vue/es/icon'
import { useRoute, useRouter } from 'vue-router'

import { useFeedInteractionStore } from '@/stores/feedInteraction'
import {
  getFeedItem,
  type FeedItem,
} from '@/api/feed'
import ReaderStack from '@/components/ReaderStack.vue'
import TopChrome from '@/components/TopChrome.vue'
import { type ChromePhase, useChromeState } from '@/composables/useChromeState'
import { useReaderSourceSubscription } from '@/composables/useReaderSourceSubscription'
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
import { useSwipeTransition } from '@/composables/useSwipeTransition'
import { useVirtualBackGuard } from '@/composables/useVirtualBackGuard'
import SubscriptionFeedView from '@/views/SubscriptionFeedView.vue'

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
const navigationOpen = ref(false)
const navigationProgress = ref(0)
const navigationSettling = ref(false)
const feedContentRef = ref<HTMLElement | null>(null)
const pageContentRef = ref<HTMLElement | null>(null)
const pageViewRef = ref<PageViewExpose | null>(null)
const {
  sourceReaderContentRef,
  detailContentRef,
  detailFrameRef,
  detailInlineSourceRef,
  sourceTitleTextRef,
  detailProgressTrackRef,
  detailProgressBarRef,
  sourceReaderScrollTop,
  detailReaderTouchOffset,
  detailReaderStretch,
  sourceReaderOffset,
  sourceReaderStretch,
  detailStretchAnchor,
  sourceStretchAnchor,
  readerBackDragging,
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
  parkedDetailStack,
  sourceReaderBackDetail,
  sourceCatalogEntry,
  sourceSubscription,
  sourceSubscriptionLoading,
  sourceNotice,
  sourceTimelinePreloadEnabled,
  detailTransitionRectsLocked,
  detailFeedOriginLocked,
  sourceReturnTargetReady,
  sourceReaderMounted,
  sourceReaderOpen,
  detailReaderOpen,
  detailCommittedListReturn,
  hasDetailParkedBehindSource,
  hasParkedDetailSourceState,
  sourceReaderShouldReturnToDetail,
  createReaderStackSessionSnapshot,
  applyReaderStackSessionSnapshot,
  snapshotCurrentDetail,
  pushParkedDetailSnapshot,
  restoreParkedDetailSnapshot: restoreReaderStackParkedDetailSnapshot,
  restorePreviousParkedDetail: restoreReaderStackPreviousParkedDetail,
  resetDetailTransition,
  clearHiddenSourceReader,
  openSourceReaderState,
  closeVisibleSourceReaderState,
  clearSourceReaderState,
  beginOpenItemReaderState,
  closeItemReaderState,
  beginCollapseItemReaderState,
  detailBlocksGestures,
} = useReaderStackState()
const {
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
const pageSideOffset = ref(0)
const pageSideStretch = ref(0)
const pageStretchAnchor = ref<'left' | 'right' | null>(null)
const viewDragOffset = ref(0)
const viewSettling = ref(false)
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
const windowWidth = ref(typeof window === 'undefined' ? 1440 : window.innerWidth)
const windowHeight = ref(typeof window === 'undefined' ? 900 : window.innerHeight)
const darkTheme = ref(false)
const refreshWasActive = ref(false)
const refreshWasSource = ref(false)
const feedRefreshSettling = ref(false)
const sourceContentSettleOffset = ref(0)
const sourceContentSettling = ref(false)
const feedTopPulling = ref(false)
const feedTopPullStartedWithChrome = ref(false)
const refreshStartedWithChrome = ref(false)
const pagePullOffset = ref(0)
const pagePullDistance = ref(0)
const pagePullSettling = ref(false)
const pagePullRefreshing = ref(false)
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
const activeFeedIndex = computed(() => (route.name === 'recommendations' ? 1 : 0))
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
const pagePullProgress = computed(() => Math.min(pagePullDistance.value / pageRefreshThreshold, 1))
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
const feedHeaderReturnProgress = computed(() => {
  if (!detailReaderOpen.value || !isFeedRoute.value || detailOpenedFromSourceReader.value) {
    return 0
  }
  if (
    sourceReaderVisible.value &&
    !detailReturningToFeed.value &&
    (detailSourceExitProgress.value > 0.001 || detailRestoringFromSourceReader.value || detailListReturnCommitted.value)
  ) {
    return 0
  }
  return clamp(Math.max(detailBackExitProgress.value, detailListReturnCommitted.value ? 1 : 0))
})
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
const detailChromeVisible = computed(
  () =>
    detailReaderOpen.value &&
    !detailParkedBehindSource.value &&
    (!detailReturningToFeed.value || readerBackDragging.value),
)
const detailHeaderVisible = computed(() => detailChromeVisible.value && topChromeProgress.value > 0.04)
const detailParkedBehindSource = computed(
  () =>
    hasDetailParkedBehindSource() && !readerBackDragging.value,
)
const sourceReaderBlockedDamping = computed(
  () => readerBackDragging.value && backSwipeTarget === 'source' && backSwipeIntent === 'blocked',
)
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
const sourceToggleLabel = computed(() => {
  if (sourceSubscriptionLoading.value) {
    return '处理中'
  }
  return sourceSubscription.value?.status === 'active' ? '关闭' : '开启'
})
const sourceToggleActive = computed(() => sourceSubscription.value?.status === 'active')
const navigationWidth = computed(() => {
  if (windowWidth.value <= 420) {
    return Math.round(windowWidth.value * 0.8)
  }
  return Math.round(Math.min(Math.max(304, windowWidth.value * 0.32), Math.min(440, windowWidth.value * 0.8)))
})
const navigationVisible = computed(() => navigationOpen.value || navigationProgress.value > 0 || navigationSettling.value)
const navigationPanelStyle = computed(() => ({
  width: `${navigationWidth.value}px`,
  transform: cssTranslate3d((navigationProgress.value - 1) * (navigationWidth.value + 28), 0),
}))
const navigationScrimStyle = computed(() => ({
  opacity: navigationProgress.value,
  pointerEvents: navigationProgress.value > 0.2 ? ('auto' as const) : ('none' as const),
}))
const feedTrackStyle = computed(() => ({
  transform: `translate3d(calc(${-activeFeedIndex.value * 100}% + ${cssPx(viewDragOffset.value)}), 0, 0)`,
}))
const viewSwipeProgress = computed(() =>
  clamp(Math.abs(viewDragOffset.value) / Math.max(1, Math.min(windowWidth.value, 320))),
)
const viewSwipeTargetKey = computed(() => {
  if (viewDragOffset.value < -viewDragThreshold && activeFeedIndex.value === 0) {
    return 'recommendations'
  }
  if (viewDragOffset.value > viewDragThreshold && activeFeedIndex.value === 1) {
    return 'subscriptions'
  }
  return ''
})
const viewSwipeTargetVisible = computed(
  () => isFeedRoute.value && !detailReaderOpen.value && Boolean(viewSwipeTargetKey.value),
)
const viewSwipeTargetProgress = computed(() =>
  viewSwipeTargetVisible.value ? clamp((viewSwipeProgress.value - 0.26) / 0.48) : 0,
)
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
const sourceReaderUnderDetail = computed(
  () => detailReaderOpen.value && sourceReaderVisible.value,
)
const sourceReaderRevealProgress = computed(() =>
  clamp(Math.max(detailSourceExitProgress.value, detailOpenedFromSourceReader.value ? detailBackExitProgress.value : 0)),
)
const sourceHeaderSpace = computed(() =>
  feedContentCollapsed.value && topChromeProgress.value <= 0.01 ? 0 : feedHeaderHeight.value,
)
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
const sourceContentStyle = computed(() => ({
  paddingTop: cssPx(sourceHeaderSpace.value + 14),
  transform: cssTranslate3d(0, sourceContentSettleOffset.value),
  transition: sourceContentSettling.value
    ? 'padding-top var(--motion-chrome) var(--ease-emphasized), transform var(--motion-chrome) var(--ease-emphasized)'
    : sourceContentSettleOffset.value > 0
      ? 'none'
      : undefined,
}))
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
  '--detail-overlay-opacity': sourceReaderBlockedDamping.value || detailCommittedListReturn() || detailReturningToFeed.value
    ? '0'
    : clamp(
        detailEntryProgress.value * (1 - Math.max(detailBackExitProgress.value, detailSourceExitProgress.value)),
      ).toFixed(3),
}))
const detailSurfaceProgress = computed(() =>
  clamp(detailEntryProgress.value * (1 - Math.max(detailBackExitProgress.value, detailSourceExitProgress.value))),
)
const feedItemPreviewProgress = computed(() => {
  if (
    sourceReaderVisible.value &&
    detailReaderOpen.value &&
    !detailParkedBehindSource.value &&
    (detailSourceExitProgress.value > 0 ||
      detailRestoringFromSourceReader.value ||
      (detailOpenedFromSourceReader.value && detailBackExitProgress.value > 0))
  ) {
    return clamp(Math.max(detailSourceExitProgress.value, detailBackExitProgress.value))
  }

  if (detailParkedBehindSource.value) {
    return 1
  }

  return clamp(Math.max(detailBackExitProgress.value, detailListReturnCommitted.value ? 1 : 0))
})
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
    !(backSwipeTarget === 'source' && !sourceReturnTargetReady.value)
  const committedListReturn = detailCommittedListReturn()
  const suppressSourceReturnPreview = sourceReaderBlockedDamping.value

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
const detailScrollMax = computed(() => Math.max(0, detailScrollHeight.value - detailScrollClientHeight.value))
const detailReadingProgress = computed(() =>
  detailScrollMax.value > 0 ? clamp(detailScrollTop.value / detailScrollMax.value) : 0,
)
const detailProgressVisible = computed(
  () =>
    detailReaderOpen.value &&
    !detailCommittedListReturn() &&
    detailSurfaceProgress.value > 0.86 &&
    detailScrollMax.value > 8,
)
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
  const progress = detailSurfaceProgress.value
  const sourceReturnProgress = sourceNameMorphProgress.value
  const sourceListTitleProgress =
    sourceReaderVisible.value && !detailReturningToFeed.value
      ? clamp((1 - sourceReturnProgress) / 0.52)
      : 0
  const opacity =
    sourceListTitleProgress > 0
      ? sourceListTitleProgress
      : clamp((progress - 0.58) / 0.22) * (1 - feedHeaderReturnProgress.value)
  return {
    opacity: opacity.toFixed(3),
    transform: cssTranslate3d(0, (1 - opacity) * 8),
    filter: `blur(${((1 - opacity) * 3.2).toFixed(2)}px)`,
    transition: readerBackDragging.value ? 'none' : undefined,
  }
})
const detailHeaderTitleSwapping = computed(() =>
  Boolean(detailHeaderPreviousTitle.value) && detailHeaderSwapProgress.value < 0.999,
)
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
      : 'opacity 220ms ease, filter var(--motion-normal) ease, transform var(--motion-normal) var(--ease-emphasized)',
  }
})
const detailInlineSourceStyle = computed(() => {
  const progress = sourceNameMorphProgress.value
  const opacity = sourceNameMorphActive.value ? clamp((0.2 - progress) / 0.2) : 1 - progress
  const blur = sourceNameMorphActive.value ? clamp(progress / 0.2) * 2.2 : progress * 1.8
  return {
    opacity: opacity.toFixed(3),
    filter: `blur(${blur.toFixed(2)}px)`,
    transform: 'translate3d(0, 0, 0)',
    transition: readerBackDragging.value ? 'none' : 'opacity 220ms ease, filter 220ms ease',
  }
})
const detailMorphSourceLabelStyle = computed(() => {
  const progress = sourceNameMorphProgress.value
  const opacity = sourceNameMorphActive.value ? clamp((0.2 - progress) / 0.2) : 1 - progress
  const blur = sourceNameMorphActive.value ? clamp(progress / 0.2) * 2.2 : progress * 1.8
  return {
    opacity: opacity.toFixed(3),
    filter: `blur(${blur.toFixed(2)}px)`,
    transition: readerBackDragging.value ? 'none' : 'opacity 220ms ease, filter 220ms ease',
  }
})
const sourceNameMorphProgress = computed(() =>
  clamp(Math.max(detailSourceExitProgress.value, detailOpenedFromSourceReader.value ? detailBackExitProgress.value : 0)),
)
const sourceNameTransitionActive = computed(
  () =>
    Boolean(detailItem.value) &&
    sourceReaderVisible.value &&
    !sourceReaderBlockedDamping.value &&
    !detailReturningToFeed.value &&
    !detailCommittedListReturn() &&
    (readerBackDragging.value ||
      detailEntrySettling.value ||
      detailRestoringFromSourceReader.value ||
      detailSourceExitProgress.value > 0.001 ||
      (detailOpenedFromSourceReader.value && detailBackExitProgress.value > 0.001)),
)
const sourceTitleProgress = computed(() =>
  detailReaderOpen.value && sourceReaderVisible.value && !detailCommittedListReturn()
    ? sourceNameMorphProgress.value
    : 1,
)
const sourceTitleRevealProgress = computed(() =>
  clamp((sourceTitleProgress.value - 0.64) / 0.24),
)
const sourceTitleRevealVisible = computed(
  () =>
    Boolean(readerSource.value) &&
    sourceNameTransitionActive.value &&
    sourceTitleRevealProgress.value > 0.001 &&
    !detailRestoringFromSourceReader.value &&
    !sourcePullActive.value,
)
const sourceNameMorphActive = computed(
  () =>
    sourceNameTransitionActive.value &&
    sourceNameMorphProgress.value > 0.001 &&
    sourceNameMorphProgress.value < 0.985 &&
    Boolean(detailSourceNameOriginRect.value && detailSourceNameTargetRect.value) &&
    (readerBackDragging.value ||
      detailRestoringFromSourceReader.value ||
      detailSourceExitProgress.value > 0.001),
)
const sourceNameMorphVisible = computed(
  () =>
    sourceNameTransitionActive.value &&
    sourceNameMorphProgress.value > 0.001 &&
    sourceNameMorphProgress.value < 0.995 &&
    Boolean(detailSourceNameOriginRect.value && detailSourceNameTargetRect.value),
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
      : 'left var(--motion-reader) var(--ease-standard), top var(--motion-reader) var(--ease-standard), width var(--motion-reader) var(--ease-standard), font-size var(--motion-reader) var(--ease-standard), opacity 180ms ease, filter 180ms ease',
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
      : 'opacity 220ms ease, filter 220ms ease, transform 220ms var(--ease-standard)',
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
const pageContentInnerStyle = computed(() => ({
  transform: `${cssTranslate3d(pageSideOffset.value, pagePullOffset.value)} scaleX(${(1 + Math.abs(pageSideStretch.value)).toFixed(4)})`,
  transformOrigin: stretchTransformOrigin(pageSideStretch.value, pageStretchAnchor.value),
}))
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
const detailMorphSummaryVisible = computed(() => detailSurfaceProgress.value < 0.54)
const detailMorphTextVisible = computed(() => {
  if (!detailItem.value || detailCommittedListReturn()) {
    return false
  }

  const inTransition =
    detailEntrySettling.value ||
    readerBackDragging.value ||
    detailReturningToFeed.value ||
    detailRestoringFromSourceReader.value ||
    detailBackExitProgress.value > 0.001 ||
    detailSourceExitProgress.value > 0.001
  return inTransition
})
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
const viewDragThreshold = 8
const viewSwipeChromeRevealDelay = 520
const topChromeSettleDuration = 1000
const pageRefreshThreshold = 52
let touchStartX = 0
let touchStartY = 0
let touchStartNavigationProgress = 0
let activeNavigationPointerId: number | null = null
let activeFeedPointerId: number | null = null
let trackingEdgeSwipeCandidate = false
let trackingNavigationCloseCandidate = false
let trackingViewSwipeCandidate = false
let trackingBackSwipeCandidate = false
let trackingEdgeSwipe = false
let trackingNavigationClose = false
let trackingViewSwipe = false
let viewSwipeStartedWithHiddenChrome = false
let trackingBackSwipe = false
let navigationDragStarted = false
let backSwipeTarget: 'detail' | 'source' | 'page' | null = null
let backSwipeIntent: 'back' | 'source' | 'blocked' | null = null
let suppressNextClick = false
let suppressClickTimer = 0
let viewSwipeTimer = 0
let swipeTransitionTimer = 0
let navigationTimer = 0
let readerMotionTimer = 0
let detailEntryTimer = 0
let detailHeaderSwapTimer = 0
let morphingHeightUnlockTimer = 0
let hiddenSourceCleanupTimer = 0
let feedRefreshSettleTimer = 0
let feedChromeSettleTimer = 0
let sourceContentSettleTimer = 0
let pagePullSettleTimer = 0
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
let activeDetailProgressPointerId: number | null = null

function resetGestureTracking() {
  trackingEdgeSwipeCandidate = false
  trackingNavigationCloseCandidate = false
  trackingViewSwipeCandidate = false
  viewSwipeStartedWithHiddenChrome = false
  trackingBackSwipeCandidate = false
  trackingEdgeSwipe = false
  trackingNavigationClose = false
  trackingViewSwipe = false
  trackingBackSwipe = false
  navigationDragStarted = false
  backSwipeTarget = null
  backSwipeIntent = null
  sourceReturnTargetReady.value = false
}

function isInteractiveTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select, [role="button"]'))
}

function isPageTopPullControlTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select'))
}

function handleClickCapture(event: MouseEvent) {
  if (!suppressNextClick) {
    return
  }
  event.preventDefault()
  event.stopPropagation()
  suppressNextClick = false
  window.clearTimeout(suppressClickTimer)
}

function suppressFollowingClick() {
  suppressNextClick = true
  window.clearTimeout(suppressClickTimer)
  suppressClickTimer = window.setTimeout(() => {
    suppressNextClick = false
  }, 420)
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
  setChromeProgress(1, 'visible')
  setChromeContentCollapsed(false)
  viewDragOffset.value = 0
  viewSettling.value = false
  if (closePanel) {
    closeNavigation()
  }
}

function handleCornerButtonClick() {
  openNavigation()
}

function navigateTo(path: string) {
  viewSettling.value = true
  viewDragOffset.value = 0
  void pushRoute(path)
  window.clearTimeout(viewSwipeTimer)
  viewSwipeTimer = window.setTimeout(() => {
    viewSettling.value = false
  }, motionDelay(260))
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
    if (!readerBackDragging.value && pageSideStretch.value === 0) {
      pageStretchAnchor.value = null
    }
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

  detailOriginRect.value = itemRect
  morphingItemHeight.value = itemRect.height
  if (lock) {
    detailFeedOriginLocked.value = true
  }
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

      if (itemRect) {
        detailSourceItemTargetRect.value = itemRect
        morphingItemHeight.value = itemRect.height
      }
      if (sourceOriginRect) {
        detailSourceNameOriginRect.value = sourceOriginRect
      }
      if (sourceTargetRect) {
        detailSourceNameTargetRect.value = sourceTargetRect
      }

      const hasSourceOrigin = Boolean(sourceOriginRect || detailSourceNameOriginRect.value)
      if (options.lock && itemRect && sourceTargetRect && hasSourceOrigin) {
        detailTransitionRectsLocked.value = true
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
  if (!itemRect) {
    sourceReturnTargetReady.value = false
    return false
  }

  const sourceTargetRect =
    snapshotElementRect(findSourceFeedItemSourceElement(detailItem.value?.id)) ?? sourceNameTargetFallback(itemRect)
  const sourceOriginRect = snapshotElementRect(detailInlineSourceRef.value)

  detailSourceItemTargetRect.value = itemRect
  morphingItemHeight.value = itemRect.height
  if (sourceTargetRect) {
    detailSourceNameTargetRect.value = sourceTargetRect
  }
  if (sourceOriginRect) {
    detailSourceNameOriginRect.value = sourceOriginRect
  }
  sourceReturnTargetReady.value = true
  return true
}

function detailFrameViewportOffset() {
  const rect = detailFrameRef.value?.getBoundingClientRect()
  return {
    left: rect?.left ?? 0,
    top: rect?.top ?? 0,
  }
}

function restoreMorphingItemContent(unlockDelay = 180) {
  const lockedItemId = morphingItemId.value ?? morphingHeightLockItemId.value
  morphingItemId.value = null
  morphingHeightLockItemId.value = lockedItemId
  window.clearTimeout(morphingHeightUnlockTimer)
  morphingHeightUnlockTimer = window.setTimeout(() => {
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = null
  }, unlockDelay)
}

function scheduleHiddenSourceReaderCleanup(delay = 180) {
  window.clearTimeout(hiddenSourceCleanupTimer)
  hiddenSourceCleanupTimer = window.setTimeout(() => {
    clearHiddenSourceReader()
  }, delay)
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
  setChromeContentCollapsed(false)
  sourceReaderVisible.value = true
  captureDetailSourceTransitionRects(12, { lock: true })
}

function settleReaderMotion(duration = 260, done?: () => void) {
  readerBackDragging.value = false
  readerMotionSettling.value = true
  window.clearTimeout(readerMotionTimer)
  readerMotionTimer = window.setTimeout(() => {
    readerMotionSettling.value = false
    done?.()
  }, motionDelay(duration))
}

function startDetailEntry(rect?: DOMRect) {
  window.clearTimeout(detailEntryTimer)
  detailOriginRect.value = snapshotRect(rect)

  if (!detailOriginRect.value) {
    detailEntryProgress.value = 1
    detailEntrySettling.value = false
    return
  }

  detailEntryProgress.value = 0
  detailEntrySettling.value = true
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      detailEntryProgress.value = 1
      detailEntryTimer = window.setTimeout(() => {
        detailEntrySettling.value = false
      }, motionDelay(readerMorphDuration))
    })
  })
}

function startDetailHeaderTitleSwap(nextItem: FeedItem) {
  if (!detailItem.value || detailItem.value.id === nextItem.id) {
    detailHeaderPreviousTitle.value = ''
    detailHeaderSwapProgress.value = 1
    window.clearTimeout(detailHeaderSwapTimer)
    return
  }

  detailHeaderPreviousTitle.value = detailItem.value.title
  detailHeaderSwapProgress.value = 0
  window.clearTimeout(detailHeaderSwapTimer)
  requestAnimationFrame(() => {
    detailHeaderSwapProgress.value = 1
  })
  detailHeaderSwapTimer = window.setTimeout(() => {
    detailHeaderPreviousTitle.value = ''
  }, motionDelay(320))
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
  if (detailReaderOpen.value) {
    restoreDetailFromParkedSource()
    return
  }

  if (parkedDetailStack.value.length > 0 && restorePreviousParkedDetail()) {
    restoreDetailFromParkedSource()
    return
  }

  if (sourceReaderBackDetail.value && restoreParkedDetailSnapshot(sourceReaderBackDetail.value)) {
    restoreDetailFromParkedSource()
    return
  }

  sourceReaderReturnMode.value = null
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

  window.clearTimeout(detailEntryTimer)
  closeItemReader()
}

function openSourceReader(source: ReaderSource, options: { visible?: boolean } = {}) {
  window.clearTimeout(hiddenSourceCleanupTimer)
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
  window.clearTimeout(morphingHeightUnlockTimer)
  window.clearTimeout(hiddenSourceCleanupTimer)
  const openedFromSourceReader =
    sourceReaderOpen.value && readerSource.value?.id === item.source_id && readerSource.value.kind === sourceKind
  startDetailHeaderTitleSwap(item)
  beginOpenItemReaderState(item, sourceKind, {
    openedFromSourceReader,
    originRect: snapshotRect(originRect),
  })
  setChromeSettling(false, 'visible')
  feedTopPulling.value = false
  setChromeContentCollapsed(false)
  setChromeProgress(1, 'visible')
  window.clearTimeout(feedChromeSettleTimer)
  activeDetailProgressPointerId = null
  lastDetailScrollTop = 0
  startDetailEntry(originRect)
  morphingItemHeight.value = detailOriginRect.value?.height ?? null
  if (openedFromSourceReader) {
    captureDetailSourceTransitionRects(12, { lock: true })
  }
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
    if (sourceKind === 'subscriptions' && item.id > 0) {
      detailItem.value = await getFeedItem(item.id)
    }
  } catch {
    detailError.value = '无法加载完整条目，已显示当前列表内容。'
  } finally {
    detailLoading.value = false
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

  if (!detailReaderOpen.value && parkedDetailStack.value.length > 0 && restorePreviousParkedDetail()) {
    restoreDetailFromParkedSource()
    return
  }

  if (sourceReaderOpen.value) {
    closeVisibleSourceReaderState()
    if (isFeedRoute.value && !detailReaderOpen.value) {
      setTopChromeVisible(true)
      setChromeContentCollapsed(false)
    }
    scheduleHiddenSourceReaderCleanup(340)
    return
  }

  clearSourceReaderState()
  resetSourceSubscriptionState()
  if (isFeedRoute.value && !detailReaderOpen.value) {
    setTopChromeVisible(true)
    setChromeContentCollapsed(false)
  }
}

function restoreDetailFromParkedSource(duration = 360) {
  if (!detailReaderOpen.value) {
    closeSourceReader()
    return
  }

  suppressFollowingClick()
  window.clearTimeout(detailEntryTimer)
  window.clearTimeout(morphingHeightUnlockTimer)
  captureVisibleSourceReturnTarget()
  if (detailItem.value?.id) {
    morphingItemId.value = detailItem.value.id
    morphingHeightLockItemId.value = detailItem.value.id
    morphingItemHeight.value = detailSourceItemTargetRect.value?.height ?? morphingItemHeight.value
  }

  const startProgress = detailSourceExitProgress.value > 0.001 ? detailSourceExitProgress.value : 1
  readerBackDragging.value = false
  detailEntrySettling.value = true
  setTopChromeVisible(true)
  setChromeContentCollapsed(false)
  detailRestoringFromSourceReader.value = true
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = startProgress
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false

  requestAnimationFrame(() => {
    detailSourceExitProgress.value = 0
  })

  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    detailRestoringFromSourceReader.value = false
    sourceReaderReturnMode.value = null
    sourceReaderBackDetail.value = null
    parkedDetailStack.value = []
    sourceReaderVisible.value = false
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
    restoreMorphingItemContent()
    scheduleHiddenSourceReaderCleanup()
  }, motionDelay(duration))
}

function restoreParkedSourceReader(duration = 260) {
  if (!detailReaderOpen.value || !sourceReaderVisible.value) {
    resetBackSwipeOffset()
    return
  }

  readerBackDragging.value = false
  detailEntrySettling.value = true
  detailRestoringFromSourceReader.value = true
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = Math.max(detailSourceExitProgress.value, 0.001)
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  window.clearTimeout(detailEntryTimer)
  requestAnimationFrame(() => {
    detailSourceExitProgress.value = 1
  })
  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    detailRestoringFromSourceReader.value = false
    detailSourceExitProgress.value = 1
    detailListReturnCommitted.value = true
  }, motionDelay(duration))
}

function closeItemReader() {
  window.clearTimeout(detailHeaderSwapTimer)
  restoreMorphingItemContent()
  const result = closeItemReaderState()
  activeDetailProgressPointerId = null
  if (isFeedRoute.value) {
    setTopChromeVisible(true)
    setChromeContentCollapsed(false)
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
  const result = beginCollapseItemReaderState()
  if (result.shouldRefreshFeedOrigin) {
    refreshDetailFeedOriginRect(true)
  }
  window.clearTimeout(detailEntryTimer)
  detailEntryTimer = window.setTimeout(() => {
    if (result.shouldRestorePreviousParkedDetail && restorePreviousParkedDetail()) {
      scheduleReaderURLAndHistorySync(true)
      return
    }
    closeItemReader()
    scheduleReaderURLAndHistorySync(true)
  }, motionDelay(duration))
}

function restoreItemReaderExpansion(duration = 360) {
  const shouldHideSourceAfterRestore =
    detailOpenedFromSourceReader.value && sourceReaderVisible.value
  readerBackDragging.value = false
  detailEntrySettling.value = true
  detailReaderTouchOffset.value = 0
  detailReaderStretch.value = 0
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = 0
  detailRestoringFromSourceReader.value = false
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  detailFeedOriginLocked.value = false
  window.clearTimeout(detailEntryTimer)
  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    if (shouldHideSourceAfterRestore) {
      sourceReaderVisible.value = false
    }
  }, motionDelay(duration))
}

function restoreDetailFromSourceSwipe(duration = 360) {
  readerBackDragging.value = false
  detailEntrySettling.value = true
  detailSourceExitProgress.value = 0
  detailRestoringFromSourceReader.value = false
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  window.clearTimeout(detailEntryTimer)
  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    sourceReaderVisible.value = false
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
  }, motionDelay(duration))
}

function completeDetailToSourceReader(duration = 360) {
  if (!readerSource.value && detailItem.value?.source_id) {
    readerSource.value = {
      id: detailItem.value.source_id,
      name: detailItem.value.source_name || '未知来源',
      kind: detailSourceKind.value,
    }
  }
  const startProgress = detailSourceExitProgress.value > 0.001 ? detailSourceExitProgress.value : 0
  if (!sourceReaderBackDetail.value) {
    sourceReaderBackDetail.value = snapshotCurrentDetail()
  }
  sourceReaderReturnMode.value = 'detail'
  setTopChromeVisible(true)
  setChromeContentCollapsed(false)
  sourceReaderVisible.value = true
  captureDetailSourceTransitionRects(12, { lock: true })
  readerBackDragging.value = false
  detailEntrySettling.value = true
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = startProgress
  detailRestoringFromSourceReader.value = false
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  window.clearTimeout(detailEntryTimer)
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      detailSourceExitProgress.value = 1
    })
  })
  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    detailListReturnCommitted.value = true
    detailSourceExitProgress.value = 1
    restoreMorphingItemContent()
  }, motionDelay(duration))
}

function settleNavigation(open: boolean) {
  window.clearTimeout(navigationTimer)
  navigationSettling.value = true
  navigationOpen.value = open
  navigationProgress.value = open ? 1 : 0
  navigationTimer = window.setTimeout(() => {
    navigationSettling.value = false
    if (!open) {
      navigationProgress.value = 0
    }
  }, motionDelay(220))
}

function openNavigation() {
  resetGestureTracking()
  window.clearTimeout(navigationTimer)
  navigationOpen.value = true
  navigationSettling.value = true
  navigationProgress.value = 0
  requestAnimationFrame(() => {
    navigationProgress.value = 1
  })
  navigationTimer = window.setTimeout(() => {
    navigationSettling.value = false
  }, motionDelay(220))
}

function closeNavigation() {
  if (!navigationVisible.value) {
    navigationOpen.value = false
    navigationProgress.value = 0
    navigationSettling.value = false
    return
  }
  settleNavigation(false)
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

function activeFeedSurface(): SwipeSurface {
  return activeFeedIndex.value === 0 ? 'feed:subscriptions' : 'feed:recommendations'
}

function feedSurfaceFromPath(path: string | null): SwipeSurface | null {
  if (path === '/subscriptions') {
    return 'feed:subscriptions'
  }
  if (path === '/recommendations') {
    return 'feed:recommendations'
  }
  return null
}

function feedSurfaceFromSwipeOffset(offset: number): SwipeSurface | null {
  if (offset < -viewDragThreshold && activeFeedIndex.value === 0) {
    return 'feed:recommendations'
  }
  if (offset > viewDragThreshold && activeFeedIndex.value === 1) {
    return 'feed:subscriptions'
  }
  return null
}

function readerSurfaceForTarget(target: typeof backSwipeTarget): SwipeSurface {
  if (target === 'source') {
    return 'reader:source'
  }
  if (target === 'page') {
    return 'page:management'
  }
  return 'reader:detail'
}

function readerSwipeTargetSurface(
  target: typeof backSwipeTarget,
  intent: typeof backSwipeIntent,
): SwipeSurface | null {
  if (intent === 'blocked' || !target) {
    return null
  }
  if (intent === 'source') {
    return 'reader:source'
  }
  if (target === 'source') {
    return 'reader:detail'
  }
  if (target === 'page') {
    return 'feed:recommendations'
  }
  return detailOpenedFromSourceReader.value ? 'reader:source' : activeFeedSurface()
}

function scheduleSwipeTransitionReset(duration = 260) {
  window.clearTimeout(swipeTransitionTimer)
  swipeTransitionTimer = window.setTimeout(() => {
    swipeTransition.reset()
  }, motionDelay(duration))
}

function beginViewSwipeTransition(offset: number) {
  swipeTransition.begin({
    from: activeFeedSurface(),
    to: feedSurfaceFromSwipeOffset(offset),
    direction: offset < 0 ? 'left' : 'right',
    progress: viewSwipeProgress.value,
  })
}

function syncViewSwipeTransition(offset: number) {
  swipeTransition.update({
    to: feedSurfaceFromSwipeOffset(offset),
    direction: offset < 0 ? 'left' : 'right',
    progress: viewSwipeProgress.value,
    isBlocked: feedSurfaceFromSwipeOffset(offset) === null,
  })
}

function beginBackSwipeTransition(deltaX: number) {
  const target = backSwipeTarget
  const intent = backSwipeIntent
  swipeTransition.begin({
    from: readerSurfaceForTarget(target),
    to: readerSwipeTargetSurface(target, intent),
    direction: deltaX < 0 ? 'left' : 'right',
    isBlocked: intent === 'blocked',
  })
}

function backSwipeTransitionProgress() {
  if (backSwipeIntent === 'source' && backSwipeTarget === 'detail') {
    return detailSourceExitProgress.value
  }
  if (backSwipeIntent === 'back' && backSwipeTarget === 'source') {
    return 1 - detailSourceExitProgress.value
  }
  if (backSwipeIntent === 'back' && backSwipeTarget === 'detail') {
    return detailBackExitProgress.value
  }
  return clamp(Math.abs(detailReaderStretch.value || sourceReaderStretch.value || pageSideStretch.value) / 0.07)
}

function syncBackSwipeTransition(deltaX: number) {
  const target = backSwipeTarget
  const intent = backSwipeIntent
  swipeTransition.update({
    to: readerSwipeTargetSurface(target, intent),
    direction: deltaX < 0 ? 'left' : 'right',
    progress: backSwipeTransitionProgress(),
    isBlocked: intent === 'blocked',
  })
}

function isBackHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > viewDragThreshold && Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
}

function canCommitRightBackSwipe() {
  return backSwipeTarget === 'detail'
}

function canReturnSourceReaderToDetail() {
  return sourceReaderShouldReturnToDetail() || hasParkedDetailSourceState() || detailRestoringFromSourceReader.value
}

function showTopChromeForSourceReturn() {
  if (topChromeProgress.value < 0.99 || feedContentCollapsed.value) {
    setTopChromeVisible(true)
  }
  setChromeContentCollapsed(false)
}

function settleSourceContentAfterRefresh() {
  if (!sourceReaderVisible.value) {
    sourceContentSettleOffset.value = 0
    sourceContentSettling.value = false
    return
  }

  window.clearTimeout(sourceContentSettleTimer)
  sourceContentSettling.value = false
  sourceContentSettleOffset.value = feedHeaderHeight.value
  requestAnimationFrame(() => {
    requestAnimationFrame(() => {
      sourceContentSettling.value = true
      sourceContentSettleOffset.value = 0
    })
  })
  sourceContentSettleTimer = window.setTimeout(() => {
    sourceContentSettling.value = false
  }, motionDelay(topChromeSettleDuration))
}

function prepareSourceReaderReturnDrag() {
  if (detailReaderOpen.value) {
    return captureVisibleSourceReturnTarget()
  }

  const parkedSnapshot = parkedDetailStack.value[parkedDetailStack.value.length - 1] ?? null
  const snapshot = sourceReaderBackDetail.value ?? parkedSnapshot
  if (!snapshot) {
    return false
  }

  if (!restoreParkedDetailSnapshot(snapshot)) {
    return false
  }
  return captureVisibleSourceReturnTarget()
}

function blockedSwipeStretch(deltaX: number, currentX = touchStartX + deltaX) {
  const width = Math.max(1, windowWidth.value)
  const edgeStopZone = Math.min(54, width * 0.12)
  const availableDistance =
    deltaX < 0
      ? Math.max(1, touchStartX - edgeStopZone)
      : Math.max(1, width - touchStartX - edgeStopZone)
  const travelledToEdge =
    deltaX < 0
      ? Math.max(0, touchStartX - currentX)
      : Math.max(0, currentX - touchStartX)
  const edgeProgress = clamp(travelledToEdge / availableDistance)
  const distanceProgress = Math.log1p(edgeProgress * 14) / Math.log1p(14)
  const stretch = 0.07 * distanceProgress
  return deltaX < 0 ? -stretch : stretch
}

function backSwipeVisualOffset(deltaX: number) {
  const limit = Math.round(windowWidth.value * 0.72)
  return Math.max(-limit, Math.min(limit, deltaX))
}

function resetBackSwipeOffset() {
  const keepDetailParkedBehindSource = hasParkedDetailSourceState()
  detailReaderTouchOffset.value = 0
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = keepDetailParkedBehindSource ? 1 : 0
  detailRestoringFromSourceReader.value = false
  detailReturningToFeed.value = false
  detailReaderStretch.value = 0
  sourceReaderStretch.value = 0
  sourceReaderOffset.value = 0
  pageSideOffset.value = 0
  pageSideStretch.value = 0
  readerBackDragging.value = false
  sourceReturnTargetReady.value = false
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
  trackingBackSwipeCandidate = true
  trackingBackSwipe = false
  backSwipeTarget = 'detail'
  if (sourceTimelinePreloadEnabled.value) {
    prepareDetailSourceReaderPreload()
  }
}

function isDetailFrameHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > 3 && Math.abs(deltaX) > Math.abs(deltaY) * 0.52
}

function beginBackSwipeIfAllowed(deltaX: number, deltaY: number, fromDetailFrame = false) {
  const horizontal = fromDetailFrame ? isDetailFrameHorizontalSwipe(deltaX, deltaY) : isBackHorizontalSwipe(deltaX, deltaY)
  if (!trackingBackSwipeCandidate || !horizontal) {
    return false
  }

  trackingBackSwipe = true
  detailEntrySettling.value = false
  sourceReturnTargetReady.value = false
  if (backSwipeTarget === 'source' && deltaX < 0 && canReturnSourceReaderToDetail()) {
    prepareSourceReaderReturnDrag()
    backSwipeIntent = 'back'
    showTopChromeForSourceReturn()
    detailRestoringFromSourceReader.value = true
    detailReturningToFeed.value = false
  } else if (deltaX > 0) {
    backSwipeIntent = canCommitRightBackSwipe() ? 'back' : 'blocked'
    if (backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value) {
      refreshDetailFeedOriginRect(true)
    }
    detailReturningToFeed.value = backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value
    if (backSwipeTarget === 'detail' && detailOpenedFromSourceReader.value && readerSource.value) {
      sourceReaderVisible.value = true
      captureDetailSourceTransitionRects(12, { lock: true })
    }
  } else if (backSwipeTarget === 'detail' && detailItem.value?.source_id && !detailOpenedFromSourceReader.value) {
    backSwipeIntent = 'source'
    showSourceReaderUnderDetail()
  } else {
    backSwipeIntent = 'blocked'
  }
  readerBackDragging.value = true
  beginBackSwipeTransition(deltaX)
  trackingBackSwipeCandidate = false
  trackingEdgeSwipeCandidate = false
  trackingNavigationCloseCandidate = false
  trackingViewSwipeCandidate = false
  return true
}

function updateBackSwipe(deltaX: number, deltaY: number, fromDetailFrame = false, currentX = touchStartX + deltaX) {
  beginBackSwipeIfAllowed(deltaX, deltaY, fromDetailFrame)

  if (!trackingBackSwipe) {
    return false
  }

  suppressFollowingClick()
  if (backSwipeTarget === 'source' && deltaX < 0 && canReturnSourceReaderToDetail()) {
    prepareSourceReaderReturnDrag()
    backSwipeIntent = 'back'
    showTopChromeForSourceReturn()
    detailRestoringFromSourceReader.value = true
    detailReturningToFeed.value = false
  } else if (deltaX > 0) {
    backSwipeIntent = canCommitRightBackSwipe() ? 'back' : 'blocked'
    if (backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value) {
      refreshDetailFeedOriginRect(true)
    }
    detailSourceExitProgress.value = 0
    detailReturningToFeed.value = backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value
    if (backSwipeTarget === 'detail' && detailOpenedFromSourceReader.value && readerSource.value) {
      sourceReaderVisible.value = true
      captureDetailSourceTransitionRects(12, { lock: true })
    }
  } else if (
    backSwipeTarget === 'detail' &&
    detailItem.value?.source_id &&
    !detailOpenedFromSourceReader.value
  ) {
    backSwipeIntent = 'source'
    showSourceReaderUnderDetail()
  } else {
    backSwipeIntent = 'blocked'
    detailReturningToFeed.value = false
  }
  const intent = backSwipeIntent
  const offset = backSwipeVisualOffset(deltaX)
  const stretch = blockedSwipeStretch(deltaX, currentX)

  detailReaderStretch.value = 0
  sourceReaderStretch.value = 0
  pageSideStretch.value = 0

  if (intent === 'back' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = clamp(Math.max(0, offset) / Math.max(220, windowWidth.value * 0.52))
  } else if (intent === 'back' && backSwipeTarget === 'source') {
    if (offset < 0 && canReturnSourceReaderToDetail()) {
      const returnProgress = clamp(Math.max(0, -offset) / Math.max(220, windowWidth.value * 0.52))
      detailRestoringFromSourceReader.value = true
      detailReaderTouchOffset.value = 0
      detailBackExitProgress.value = 0
      sourceReaderOffset.value = 0
      detailSourceExitProgress.value = 1 - returnProgress
    } else {
      detailSourceExitProgress.value = hasParkedDetailSourceState() ? 1 : 0
      sourceReaderStretch.value = stretch
      updateStretchAnchor(sourceStretchAnchor, stretch)
      sourceReaderOffset.value = 0
    }
  } else if (intent === 'back' && backSwipeTarget === 'page') {
    pageSideOffset.value = 0
    pageSideStretch.value = stretch
    updateStretchAnchor(pageStretchAnchor, stretch)
  } else if (intent === 'source' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = clamp(Math.max(0, -offset) / Math.max(220, windowWidth.value * 0.52))
  } else if (intent === 'blocked' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailReaderStretch.value = stretch
    updateStretchAnchor(detailStretchAnchor, stretch)
  } else if (intent === 'blocked' && backSwipeTarget === 'source') {
    detailSourceExitProgress.value = hasParkedDetailSourceState() ? 1 : 0
    sourceReaderStretch.value = stretch
    updateStretchAnchor(sourceStretchAnchor, stretch)
  } else if (intent === 'blocked' && backSwipeTarget === 'page') {
    pageSideOffset.value = 0
    pageSideStretch.value = stretch
    updateStretchAnchor(pageStretchAnchor, stretch)
  }

  syncBackSwipeTransition(deltaX)
  return true
}

function finishBackSwipe(deltaX: number, _deltaY: number) {
  const target = backSwipeTarget
  const intent = backSwipeIntent
  const shouldCommit =
    intent === 'back' && target === 'detail'
      ? deltaX > 0 && (detailBackExitProgress.value >= 0.42 || deltaX >= viewSwitchDistance)
      : intent === 'back' && target === 'source'
        ? deltaX < 0 && (detailSourceExitProgress.value <= 0.58 || Math.abs(deltaX) >= viewSwitchDistance)
      : intent === 'source' && target === 'detail'
        ? deltaX < 0 && (detailSourceExitProgress.value >= 0.42 || Math.abs(deltaX) >= viewSwitchDistance)
        : intent === 'back'
          ? deltaX > 0 && Math.abs(deltaX) >= viewSwitchDistance
          : false

  swipeTransition.settle(shouldCommit, {
    progress: shouldCommit ? 1 : backSwipeTransitionProgress(),
    isBlocked: intent === 'blocked',
  })
  scheduleSwipeTransitionReset(360)

  if (!shouldCommit) {
    if (intent === 'back' && target === 'detail') {
      restoreItemReaderExpansion()
      return
    }
    if (intent === 'source' && target === 'detail') {
      restoreDetailFromSourceSwipe()
      return
    }
    if (intent === 'back' && target === 'source' && canReturnSourceReaderToDetail()) {
      restoreParkedSourceReader()
      return
    }
    resetBackSwipeOffset()
    return
  }

  suppressFollowingClick()
  if (intent === 'source' && target === 'detail') {
    completeDetailToSourceReader()
    return
  }
  if (intent === 'back' && target === 'detail') {
    collapseItemReader()
    return
  }
  if (intent === 'back' && target === 'source') {
    if (canReturnSourceReaderToDetail()) {
      restoreDetailFromParkedSource()
      return
    }

    resetBackSwipeOffset()
    return
  }
  if (intent === 'back' && target === 'page') {
    resetBackSwipeOffset()
    return
  }

  resetBackSwipeOffset()
}

function finishViewSwipe(nextPath: string | null) {
  const committed = Boolean(nextPath)
  viewSettling.value = true
  swipeTransition.settle(committed, { progress: committed ? 1 : 0, isBlocked: false })
  window.clearTimeout(viewSwipeTimer)
  const shouldRevealChromeFirst = Boolean(nextPath) && viewSwipeStartedWithHiddenChrome
  viewSwipeStartedWithHiddenChrome = false
  if (shouldRevealChromeFirst) {
    setTopChromeVisible(true)
    viewSwipeTimer = window.setTimeout(() => {
      if (nextPath) {
        void pushRoute(nextPath)
      }
      viewDragOffset.value = 0
      viewSwipeTimer = window.setTimeout(() => {
        viewSettling.value = false
      }, motionDelay(260))
    }, viewSwipeChromeRevealDelay)
    scheduleSwipeTransitionReset(viewSwipeChromeRevealDelay + 260)
    return
  }
  if (nextPath) {
    void pushRoute(nextPath)
  }
  viewDragOffset.value = 0
  viewSwipeTimer = window.setTimeout(() => {
    viewSettling.value = false
  }, motionDelay(260))
  scheduleSwipeTransitionReset(260)
}

function showTopChromeForViewSwipe() {
  const shouldRevealChrome = topChromeProgress.value < 0.99 || feedContentCollapsed.value
  if (shouldRevealChrome) {
    viewSwipeStartedWithHiddenChrome = true
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
    trackingBackSwipeCandidate = false
    return
  }

  if (detailBlocksGestures()) {
    beginDetailGestureCandidate(touch.clientX, touch.clientY)
    return
  }
  if (sourceReaderOpen.value) {
    trackingBackSwipeCandidate = true
    backSwipeTarget = 'source'
    return
  }
  if (!isFeedRoute.value && !navigationVisible.value) {
    trackingBackSwipeCandidate = true
    backSwipeTarget = 'page'
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
    !trackingBackSwipeCandidate &&
    !trackingEdgeSwipe &&
    !trackingNavigationClose &&
    !trackingViewSwipe &&
    !trackingBackSwipe
  ) {
    return
  }

  const touch = event.touches[0]
  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY
  const horizontal = isHorizontalSwipe(deltaX, deltaY)
  const viewHorizontal = isViewHorizontalSwipe(deltaX, deltaY)

  if (trackingBackSwipeCandidate || trackingBackSwipe) {
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
    if (
      (activeFeedIndex.value === 0 && deltaX < -viewDragThreshold) ||
      (activeFeedIndex.value === 1 && deltaX > viewDragThreshold)
    ) {
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
    if (activeFeedIndex.value === 0) {
      viewDragOffset.value = Math.min(0, Math.max(deltaX, -windowWidth.value))
    } else {
      viewDragOffset.value = Math.max(0, Math.min(deltaX, windowWidth.value))
    }
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
    !trackingBackSwipeCandidate &&
    !trackingEdgeSwipe &&
    !trackingNavigationClose &&
    !trackingViewSwipe &&
    !trackingBackSwipe
  ) {
    return
  }

  const touch = event.changedTouches[0]
  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY
  const horizontal = isHorizontalSwipe(deltaX, deltaY)

  if (trackingBackSwipe) {
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
    if (activeFeedIndex.value === 0 && horizontal && deltaX <= -viewSwitchDistance) {
      finishViewSwipe('/recommendations')
    } else if (activeFeedIndex.value === 1 && horizontal && deltaX >= viewSwitchDistance) {
      finishViewSwipe('/subscriptions')
    } else {
      finishViewSwipe(null)
    }
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
  viewSettling.value = false
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

    if ((activeFeedIndex.value === 0 && deltaX > 0) || (activeFeedIndex.value === 1 && deltaX < 0)) {
      activeFeedPointerId = null
      trackingViewSwipeCandidate = false
      return
    }

    if (
      (activeFeedIndex.value === 0 && deltaX < -viewDragThreshold) ||
      (activeFeedIndex.value === 1 && deltaX > viewDragThreshold)
    ) {
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

  if ((activeFeedIndex.value === 0 && deltaX > 0) || (activeFeedIndex.value === 1 && deltaX < 0)) {
    viewDragOffset.value = 0
    return
  }

  if (activeFeedIndex.value === 0) {
    viewDragOffset.value = Math.min(0, Math.max(deltaX, -windowWidth.value))
  } else {
    viewDragOffset.value = Math.max(0, Math.min(deltaX, windowWidth.value))
  }
  syncViewSwipeTransition(viewDragOffset.value)
}

function handleFeedPointerUp(event: PointerEvent) {
  if (activeFeedPointerId !== event.pointerId || event.pointerType === 'mouse') {
    return
  }

  const deltaX = event.clientX - touchStartX
  const deltaY = event.clientY - touchStartY
  const horizontal = isViewHorizontalSwipe(deltaX, deltaY)

  if (trackingViewSwipe && activeFeedIndex.value === 0 && horizontal && deltaX <= -viewSwitchDistance) {
    suppressFollowingClick()
    finishViewSwipe('/recommendations')
  } else if (trackingViewSwipe && activeFeedIndex.value === 1 && horizontal && deltaX >= viewSwitchDistance) {
    suppressFollowingClick()
    finishViewSwipe('/subscriptions')
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
  const hadBackGesture = trackingBackSwipe
  const canceledBackIntent = backSwipeIntent
  const canceledBackTarget = backSwipeTarget
  resetGestureTracking()
  if (hadNavigationGesture && navigationVisible.value && !navigationOpen.value) {
    settleNavigation(false)
  }
  if (hadViewGesture) {
    finishViewSwipe(null)
  }
  if (hadBackGesture) {
    if (canceledBackIntent === 'back' && canceledBackTarget === 'detail') {
      restoreItemReaderExpansion()
    } else if (canceledBackIntent === 'source' && canceledBackTarget === 'detail') {
      restoreDetailFromSourceSwipe()
    } else if (
      canceledBackIntent === 'back' &&
      canceledBackTarget === 'source' &&
      (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value)
    ) {
      restoreParkedSourceReader()
    } else {
      resetBackSwipeOffset()
    }
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

  detailScrollTop.value = container.scrollTop
  detailScrollHeight.value = Math.max(0, container.scrollHeight)
  detailScrollClientHeight.value = Math.max(0, container.clientHeight)
}

function scrollDetailContentTo(top: number) {
  const container = detailContentRef.value
  if (!container) {
    return
  }

  container.scrollTop = Math.max(0, top)
  syncDetailContainerMetrics()
}

function updateDetailProgressFromPointer(clientY: number) {
  const track = detailProgressBarRef.value ?? detailProgressTrackRef.value
  if (!track || detailScrollMax.value <= 0) {
    return
  }

  const rect = track.getBoundingClientRect()
  const progress = clamp((clientY - rect.top) / Math.max(1, rect.height))
  const nextScrollTop = detailScrollMax.value * progress
  detailScrollTop.value = nextScrollTop
  lastDetailScrollTop = nextScrollTop
  scrollDetailContentTo(nextScrollTop)
}

function handleDetailProgressPointerDown(event: PointerEvent) {
  if (!detailProgressVisible.value || event.pointerType === 'mouse' && event.button !== 0) {
    return
  }

  event.preventDefault()
  event.stopPropagation()
  suppressFollowingClick()
  detailProgressDragging.value = true
  activeDetailProgressPointerId = event.pointerId
  ;(event.currentTarget as HTMLElement | null)?.setPointerCapture?.(event.pointerId)
  updateDetailProgressFromPointer(event.clientY)
}

function handleDetailProgressPointerMove(event: PointerEvent) {
  if (!detailProgressDragging.value || activeDetailProgressPointerId !== event.pointerId) {
    return
  }

  event.preventDefault()
  event.stopPropagation()
  updateDetailProgressFromPointer(event.clientY)
}

function finishDetailProgressDrag(event?: PointerEvent) {
  if (event && activeDetailProgressPointerId !== event.pointerId) {
    return
  }

  if (event) {
    ;(event.currentTarget as HTMLElement | null)?.releasePointerCapture?.(event.pointerId)
  }
  activeDetailProgressPointerId = null
  detailProgressDragging.value = false
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
      detailFrameContentHeight.value = Math.max(0, scrollHeight)
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
    if (trackingBackSwipe) {
      finishBackSwipe(deltaX, deltaY)
      resetGestureTracking()
      return
    }
    resetGestureTracking()
    return
  }

  if (payload.phase === 'cancel') {
    if (trackingBackSwipe) {
      const canceledBackIntent = backSwipeIntent
      const canceledBackTarget = backSwipeTarget
      resetGestureTracking()
      if (canceledBackIntent === 'back' && canceledBackTarget === 'detail') {
        restoreItemReaderExpansion()
      } else if (canceledBackIntent === 'source' && canceledBackTarget === 'detail') {
        restoreDetailFromSourceSwipe()
      } else if (
        canceledBackIntent === 'back' &&
        canceledBackTarget === 'source' &&
        (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value)
      ) {
        restoreParkedSourceReader()
      } else {
        resetBackSwipeOffset()
      }
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
  const nextProgress = visible ? 1 : 0
  if (topChromeProgress.value === nextProgress) {
    if (visible && feedContentCollapsed.value) {
      setChromeContentCollapsed(false)
    }
    if (!visible && feedContentCollapsed.value) {
      setChromeSettling(true, 'hiding')
      window.clearTimeout(feedChromeSettleTimer)
      feedChromeSettleTimer = window.setTimeout(() => {
        setChromeSettling(false)
      }, motionDelay(topChromeSettleDuration))
      return
    }
    setChromeProgress(nextProgress, visible ? 'visible' : 'hidden')
    return
  }

  setChromeSettling(true, visible ? 'revealing' : 'hiding')
  window.clearTimeout(feedChromeSettleTimer)
  if (visible) {
    setChromeContentCollapsed(false)
  }
  setChromeProgress(nextProgress, visible ? 'revealing' : 'hiding')
  feedChromeSettleTimer = window.setTimeout(() => {
    setChromeSettling(false)
  }, motionDelay(topChromeSettleDuration))
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
  window.clearTimeout(feedChromeSettleTimer)
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
  sourceReaderScrollTop.value = current
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
  detailScrollTop.value = current
  detailScrollHeight.value = Math.max(0, target.scrollHeight)
  detailScrollClientHeight.value = Math.max(0, target.clientHeight)
  updateTopTabsByScroll(current, lastDetailScrollTop)
  lastDetailScrollTop = current
  scheduleReaderSessionSave()
}

function resetPageTopPullTracking() {
  pageTopPullDistance = 0
  pagePullDistance.value = pagePullRefreshing.value ? pageRefreshThreshold : 0
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
  window.clearTimeout(pagePullSettleTimer)
  pagePullSettling.value = true
  pagePullOffset.value = 0
  pagePullSettleTimer = window.setTimeout(() => {
    pagePullSettling.value = false
  }, motionDelay(topChromeSettleDuration))
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
    pagePullDistance.value = pageTopPullDistance
    pagePullSettling.value = false
    window.clearTimeout(pagePullSettleTimer)
    pagePullOffset.value = pageRubberBandOffset(deltaY)
  }
}

async function refreshCurrentPageFromPull() {
  const refreshPage = pageViewRef.value?.refreshPage
  if (!refreshPage || pagePullRefreshing.value) {
    return
  }

  pagePullRefreshing.value = true
  pagePullDistance.value = pageRefreshThreshold
  try {
    await refreshPage({ noticeDelayMS: 180, suppressStartNotice: true })
  } finally {
    pagePullRefreshing.value = false
    pagePullDistance.value = 0
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
    pagePullOffset.value = 0
    pagePullDistance.value = 0
    pagePullSettling.value = false
    viewDragOffset.value = 0
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
    sourceReaderBackDetail.value?.item.id ?? 0,
    parkedDetailStack.value.length,
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
      refreshWasActive.value = true
      refreshWasSource.value = feedInteraction.pullViewKey.startsWith('source:')
      if (feedTopPullStartedWithChrome.value) {
        refreshStartedWithChrome.value = true
      }
    }
  },
)

watch(
  feedOrSourcePullActive,
  (active) => {
    if (!active && refreshWasActive.value) {
      const shouldSettleSourceContent = refreshWasSource.value
      if (shouldSettleSourceContent) {
        settleSourceContentAfterRefresh()
      }
      refreshStartedWithChrome.value = false
      feedTopPullStartedWithChrome.value = false
      setChromeContentCollapsed(true)
      setTopChromeVisible(false)
      refreshWasActive.value = false
      refreshWasSource.value = false
      feedRefreshSettling.value = true
      window.clearTimeout(feedRefreshSettleTimer)
      feedRefreshSettleTimer = window.setTimeout(() => {
        feedRefreshSettling.value = false
      }, motionDelay(topChromeSettleDuration))
    }

    if (!active && !refreshWasActive.value) {
      refreshStartedWithChrome.value = false
      feedTopPullStartedWithChrome.value = false
      refreshWasSource.value = false
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
  window.clearTimeout(viewSwipeTimer)
  window.clearTimeout(swipeTransitionTimer)
  window.clearTimeout(navigationTimer)
  window.clearTimeout(feedRefreshSettleTimer)
  window.clearTimeout(feedChromeSettleTimer)
  window.clearTimeout(sourceContentSettleTimer)
  window.clearTimeout(pagePullSettleTimer)
  window.clearTimeout(suppressClickTimer)
  clearSourceNoticeTimer()
  window.clearTimeout(readerMotionTimer)
  window.clearTimeout(detailEntryTimer)
  window.clearTimeout(detailHeaderSwapTimer)
  window.clearTimeout(morphingHeightUnlockTimer)
  window.clearTimeout(hiddenSourceCleanupTimer)
  readerSession.clearTimer()
})
</script>

<template>
  <div class="app-shell" @click.capture="handleClickCapture">
    <button
      v-if="!navigationVisible"
      class="nav-open-button"
      :class="{ 'nav-open-button--hidden': feedCornerHidden, 'nav-open-button--detail': detailChromeVisible }"
      :style="navOpenButtonStyle"
      type="button"
      :aria-label="cornerButtonLabel"
      @pointerdown.stop
      @touchstart.stop
      @click="handleCornerButtonClick"
    >
      <IconMenuUnfold />
    </button>

    <button
      v-show="navigationVisible"
      class="nav-scrim"
      type="button"
      aria-label="关闭导航"
      :style="navigationScrimStyle"
      @click="closeNavigation"
    />

    <aside
      v-show="navigationVisible"
      class="nav-panel"
      :class="{ 'nav-panel--settling': navigationSettling }"
      :style="navigationPanelStyle"
      aria-label="主导航"
    >
      <div class="nav-panel-glow" />

      <div class="brand">
        <div class="brand-mark">MF</div>
        <button
          class="brand-home-button"
          type="button"
          aria-label="返回主页"
          @pointerdown.stop
          @touchstart.stop
          @click="goHome(true)"
        >
          <div class="brand-title">messageFeed</div>
          <div class="brand-subtitle">信息阅读</div>
        </button>
      </div>

      <nav class="app-menu" aria-label="管理导航">
        <button
          v-for="item in managementItems"
          :key="item.key"
          class="nav-item"
          :class="{ 'nav-item--active': selectedKeys.includes(item.key) }"
          type="button"
          @pointerdown.stop
          @touchstart.stop
          @click="handleMenuClick(item.key)"
        >
          <component :is="item.icon" />
          <span>{{ item.label }}</span>
        </button>
      </nav>

      <div class="nav-panel-actions">
        <button
          class="theme-icon-button"
          type="button"
          aria-label="切换主题"
          @pointerdown.stop
          @touchstart.stop
          @click="toggleTheme"
        >
          <component :is="darkTheme ? IconSunFill : IconMoonFill" />
        </button>

        <button
          class="settings-icon-button"
          :class="{ 'settings-icon-button--active': route.name === 'settings' }"
          type="button"
          aria-label="设置"
          @pointerdown.stop
          @touchstart.stop
          @click="pushRoute('/settings'); closeNavigation()"
        >
          <IconSettings />
        </button>
      </div>
    </aside>

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
          <div v-if="isFeedRoute || detailChromeVisible" class="app-header-feed-stack">
            <div
              v-if="detailReaderOpen"
              class="feed-header-layer feed-header-layer--detail"
              :class="{ 'feed-header-layer--hidden': !detailHeaderVisible }"
              :style="detailHeaderLayerStyle"
            >
              <div v-if="detailItem" class="detail-header-title" :style="detailHeaderTitleStyle">
                <span
                  v-if="detailHeaderPreviousTitle"
                  class="detail-header-title__text detail-header-title__text--previous"
                  :style="detailHeaderPreviousTextStyle"
                  aria-hidden="true"
                >
                  {{ detailHeaderPreviousTitle }}
                </span>
                <span class="detail-header-title__text" :style="detailHeaderCurrentTextStyle">
                  {{ detailItem.title }}
                </span>
              </div>
            </div>
            <div
              v-if="isFeedRoute"
              class="feed-header-layer feed-header-layer--tabs"
              :class="{ 'feed-header-layer--hidden': feedTabsLayerHidden }"
              :style="feedTabsLayerStyle"
            >
              <div class="feed-tabs" role="tablist" aria-label="阅读视图">
                <button
                  v-for="tab in feedTabs"
                  :key="tab.key"
                  class="feed-tab"
                  :class="{ 'feed-tab--active': route.name === tab.key }"
                  type="button"
                  role="tab"
                  :aria-selected="route.name === tab.key"
                  @pointerdown.stop
                  @touchstart.stop
                  @click="navigateTo(tab.path)"
                >
                  {{ tab.label }}
                </button>
              </div>
            </div>
            <div
              v-if="isFeedRoute"
              class="feed-header-layer feed-header-layer--tabs feed-header-layer--view-target"
              :class="{ 'feed-header-layer--hidden': !viewSwipeTargetVisible }"
              :style="feedTabsTargetLayerStyle"
              aria-hidden="true"
            >
              <div class="feed-tabs" role="presentation">
                <button
                  v-for="tab in feedTabs"
                  :key="`target-${tab.key}`"
                  class="feed-tab"
                  :class="{ 'feed-tab--active': viewSwipeTargetKey === tab.key }"
                  type="button"
                  tabindex="-1"
                >
                  {{ tab.label }}
                </button>
              </div>
            </div>
            <div
              v-if="isFeedRoute"
              class="feed-header-layer feed-header-layer--refresh"
              :class="{ 'feed-header-layer--hidden': detailReaderOpen || !feedPullActive }"
              :style="pullStatusStyle"
              aria-live="polite"
            >
              <span
                class="feed-refresh-header__icon"
                :class="{ 'feed-refresh-header__icon--refreshing': feedInteraction.pullRefreshing }"
                :style="pullIconStyle"
              >
                <IconSync />
              </span>
              <div class="feed-refresh-header__copy">
                <div class="feed-refresh-header__title">{{ pullStatusText }}</div>
                <div class="feed-refresh-header__meta">{{ pullStatusMeta }}</div>
              </div>
            </div>
          </div>
          <div v-else class="app-header-page-stack">
            <div
              class="feed-header-layer feed-header-layer--tabs"
              :class="{ 'feed-header-layer--hidden': pagePullActive }"
              :style="pageTitleLayerStyle"
            >
              <h1>{{ pageTitle }}</h1>
            </div>
            <div
              class="feed-header-layer feed-header-layer--refresh"
              :class="{ 'feed-header-layer--hidden': !pagePullActive }"
              :style="pagePullStatusStyle"
              aria-live="polite"
            >
              <span
                class="feed-refresh-header__icon"
                :class="{ 'feed-refresh-header__icon--refreshing': pagePullRefreshing }"
                :style="pagePullIconStyle"
              >
                <IconSync />
              </span>
              <div class="feed-refresh-header__copy">
                <div class="feed-refresh-header__title">{{ pagePullStatusText }}</div>
                <div class="feed-refresh-header__meta">{{ pagePullStatusMeta }}</div>
              </div>
            </div>
          </div>
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
        <div class="feed-stage">
          <div
            class="feed-track"
            :class="{ 'feed-track--settling': viewSettling }"
            :style="feedTrackStyle"
          >
            <section class="feed-pane">
              <SubscriptionFeedView
                mode="subscriptions"
                :active="route.name === 'subscriptions' && !detailReaderOpen && !sourceReaderOpen"
                :scroll-top="feedScrollTop"
                :top-chrome-progress="topChromeProgress"
                :header-height="feedHeaderHeight"
                :freeze-body-during-top-refresh="freezeFeedBodyDuringTopRefresh"
                :morphing-item-id="morphingItemId"
                :morphing-height-lock-item-id="morphingHeightLockItemId"
                :morphing-item-height="morphingItemHeight"
                :morphing-preview-progress="feedItemPreviewProgress"
                @top-pull-start="handleFeedTopPullStart"
                @top-pull-move="handleFeedTopPullMove"
                @top-pull-end="handleFeedTopPullEnd"
                @open-item="openItemReader"
              />
            </section>
            <section class="feed-pane">
              <SubscriptionFeedView
                mode="recommendations"
                :active="route.name === 'recommendations' && !detailReaderOpen && !sourceReaderOpen"
                :scroll-top="feedScrollTop"
                :top-chrome-progress="topChromeProgress"
                :header-height="feedHeaderHeight"
                :freeze-body-during-top-refresh="freezeFeedBodyDuringTopRefresh"
                :morphing-item-id="morphingItemId"
                :morphing-height-lock-item-id="morphingHeightLockItemId"
                :morphing-item-height="morphingItemHeight"
                :morphing-preview-progress="feedItemPreviewProgress"
                @top-pull-start="handleFeedTopPullStart"
                @top-pull-move="handleFeedTopPullMove"
                @top-pull-end="handleFeedTopPullEnd"
                @open-item="openItemReader"
              />
            </section>
          </div>
        </div>
      </section>
      <section
        v-else
        ref="pageContentRef"
        class="app-content app-content--page"
        @scroll.passive="handlePageContentScroll"
        @touchstart.passive="handlePageTouchStart"
        @touchmove="handlePageTouchMove"
        @touchend.passive="handlePageTouchEnd"
        @touchcancel.passive="handlePageTouchCancel"
      >
        <div class="page-content-inner" :style="pageContentInnerStyle">
          <router-view v-slot="{ Component }">
            <component :is="Component" ref="pageViewRef" @open-source="openSourceReader" />
          </router-view>
        </div>
      </section>
    </main>

    <ReaderStack
      :source-mounted="sourceReaderMounted && Boolean(readerSource)"
      :source-under-detail="sourceReaderUnderDetail"
      :source-style="sourceReaderStyle"
      :source-title-reveal-mounted="Boolean(readerSource)"
      :source-title-reveal-visible="sourceTitleRevealVisible"
      :source-title-reveal-style="sourceTitleRevealStyle"
      :source-name-morph-mounted="Boolean(detailItem)"
      :source-name-morph-visible="sourceNameMorphVisible"
      :source-name-morph-style="sourceNameMorphStyle"
      :detail-open="detailReaderOpen"
      :detail-class="{
        'reader-overlay--motion-settling': readerMotionSettling,
        'reader-overlay--returning-feed': detailReturningToFeed,
      }"
      :detail-style="detailReaderStyle"
    >
      <template #source>
        <div
          v-if="sourceNotice"
          class="sources-toast reader-toast"
          :class="`sources-toast--${sourceNotice.type}`"
          role="status"
          aria-live="polite"
        >
          {{ sourceNotice.message }}
        </div>
        <TopChrome
          variant="source"
          :phase="topChromePhase"
          :progress="topChromeProgress"
          :root-style="sourceHeaderStyle"
        >
          <button class="reader-back-button" type="button" aria-label="打开导航" @click="openNavigation">
            <IconMenuUnfold />
          </button>
          <div class="reader-overlay__source-stack">
            <div
              class="reader-source-layer"
              :class="{ 'reader-source-layer--hidden': sourcePullActive }"
              :style="sourceMainLayerStyle"
            >
              <div class="reader-overlay__title" :style="sourceTitleLayerStyle">
                <span ref="sourceTitleTextRef" :style="sourceTitleTextStyle">{{ readerSource?.name }}</span>
                <small>{{ sourceToggleActive ? '已订阅' : '未订阅' }}</small>
              </div>
              <button
                class="sources-button reader-source-toggle"
                :class="{ 'sources-button--active': sourceToggleActive }"
                type="button"
                :disabled="sourceSubscriptionLoading"
                @pointerdown.stop
                @touchstart.stop
                @click="toggleSourceReaderSubscription"
              >
                {{ sourceToggleLabel }}
              </button>
            </div>
            <div
              class="reader-source-layer reader-source-layer--refresh"
              :class="{ 'reader-source-layer--hidden': !sourcePullActive }"
              :style="sourcePullStatusStyle"
              aria-live="polite"
            >
              <span
                class="feed-refresh-header__icon"
                :class="{ 'feed-refresh-header__icon--refreshing': feedInteraction.pullRefreshing }"
                :style="sourcePullIconStyle"
              >
                <IconSync />
              </span>
              <div class="feed-refresh-header__copy">
                <div class="feed-refresh-header__title">{{ pullStatusText }}</div>
                <div class="feed-refresh-header__meta">{{ pullStatusMeta }}</div>
              </div>
            </div>
          </div>
        </TopChrome>
        <div
          ref="sourceReaderContentRef"
          class="reader-overlay__content reader-overlay__content--source"
          :style="sourceContentStyle"
          @scroll.passive="handleSourceReaderScroll"
        >
          <SubscriptionFeedView
            v-if="readerSource"
            :key="`${readerSource.kind}:${readerSource.id}:${sourceReaderRefreshNonce}`"
            mode="source"
            :source-kind="readerSource.kind"
            :source-id="readerSource.id"
            :active="true"
            :scroll-top="sourceReaderScrollTop"
            :top-chrome-progress="topChromeProgress"
            :header-height="feedHeaderHeight"
            :freeze-body-during-top-refresh="true"
            :morphing-item-id="morphingItemId"
            :morphing-height-lock-item-id="morphingHeightLockItemId"
            :morphing-item-height="morphingItemHeight"
            :morphing-preview-progress="feedItemPreviewProgress"
            :background-refresh="!sourceReaderVisible"
            @top-pull-start="handleFeedTopPullStart"
            @top-pull-move="handleFeedTopPullMove"
            @top-pull-end="handleFeedTopPullEnd"
            @open-item="openItemReader"
          />
        </div>
      </template>

      <template #source-title-reveal>
        <span>{{ readerSource?.name }}</span>
        <small>{{ sourceToggleActive ? '已订阅' : '未订阅' }}</small>
      </template>

      <template #source-name-morph>
        {{ detailItem?.source_name || '未知来源' }}
      </template>

      <template #detail>
        <div
          class="reader-transition-surface"
          :class="{
            'reader-transition-surface--entry-settling': detailEntrySettling,
            'reader-transition-surface--chrome-settling': feedChromeSettling,
          }"
          :style="detailTransitionSurfaceStyle"
        >
          <article v-if="detailItem && detailMorphTextVisible" class="reader-morph-text" :style="detailMorphTextStyle">
            <div class="reader-morph-text__meta">
              <span class="reader-morph-text__source-label" :style="detailMorphSourceLabelStyle">
                {{ detailItem.source_name || '未知来源' }}
              </span>
              <span>{{ detailDisplayDate }}</span>
              <span v-if="detailItem.author">{{ detailItem.author }}</span>
            </div>
            <h2>{{ detailItem.title }}</h2>
            <p v-if="detailMorphSummaryVisible">{{ detailPreviewSummary }}</p>
          </article>
          <div
            ref="detailContentRef"
            class="reader-overlay__content reader-detail"
            :style="detailContentStyle"
            @scroll.passive="handleDetailContentScroll"
          >
            <a-alert v-if="detailError" type="warning" show-icon :content="detailError" />
            <section v-if="detailItem" class="reader-detail__surface">
              <div class="reader-detail__inline-meta">
                <span
                  ref="detailInlineSourceRef"
                  class="reader-detail__inline-source"
                  :style="detailInlineSourceStyle"
                >
                  {{ detailItem.source_name || '未知来源' }}
                </span>
                <span>{{ detailDisplayDate }}</span>
              </div>
              <iframe
                ref="detailFrameRef"
                class="reader-detail__frame"
                title="条目正文"
                sandbox="allow-scripts allow-popups allow-popups-to-escape-sandbox"
                :srcdoc="detailSrcdoc"
                @load="handleDetailFrameLoad"
              />
              <div class="reader-detail__actions">
                <a :href="detailItem.url" target="_blank" rel="noreferrer">阅读原文</a>
              </div>
            </section>
            <section v-else class="empty-surface">
              <div class="empty-surface__mark">读</div>
              <h2>{{ detailLoading ? '正在加载条目' : '暂无条目内容' }}</h2>
              <p>请稍候。</p>
            </section>
          </div>
        </div>
        <div
          ref="detailProgressTrackRef"
          class="reader-detail-progress"
          :class="{ 'reader-detail-progress--dragging': detailProgressDragging }"
          role="scrollbar"
          aria-label="正文阅读进度"
          aria-orientation="vertical"
          :aria-valuenow="Math.round(detailReadingProgress * 100)"
          aria-valuemin="0"
          aria-valuemax="100"
          :style="detailProgressStyle"
          @pointerdown="handleDetailProgressPointerDown"
          @pointermove="handleDetailProgressPointerMove"
          @pointerup="finishDetailProgressDrag"
          @pointercancel="finishDetailProgressDrag"
          @touchstart.stop.prevent
        >
          <div ref="detailProgressBarRef" class="reader-detail-progress__track">
            <div class="reader-detail-progress__fill" :style="detailProgressFillStyle" />
            <div class="reader-detail-progress__thumb" :style="detailProgressThumbStyle" />
          </div>
        </div>
      </template>
    </ReaderStack>
  </div>
</template>
