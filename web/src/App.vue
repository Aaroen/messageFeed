<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import {
  IconArrowLeft,
  IconBook,
  IconHome,
  IconMenuUnfold,
  IconMoonFill,
  IconRefresh,
  IconSettings,
  IconSunFill,
} from '@arco-design/web-vue/es/icon'
import { useRoute, useRouter } from 'vue-router'

import { useFeedInteractionStore } from '@/stores/feedInteraction'
import {
  fetchSource,
  getFeedItem,
  importCatalogSources,
  listSourceCatalog,
  listSources,
  updateSourceStatus,
  type FeedItem,
  type Source,
  type SourceCatalogEntry,
} from '@/api/feed'
import { formatAPIError } from '@/api/client'
import SubscriptionFeedView from '@/views/SubscriptionFeedView.vue'

type FeedSourceKind = 'subscriptions' | 'recommendations'
type ReaderSource = {
  id: number
  name: string
  kind: FeedSourceKind
}
type RectSnapshot = {
  left: number
  top: number
  width: number
  height: number
}
type ParkedDetailSnapshot = {
  item: FeedItem
  sourceKind: FeedSourceKind
  originRect: RectSnapshot | null
  sourceItemTargetRect: RectSnapshot | null
  sourceNameOriginRect: RectSnapshot | null
  sourceNameTargetRect: RectSnapshot | null
  morphingItemHeight: number | null
  scrollTop: number
}

const route = useRoute()
const router = useRouter()
const feedInteraction = useFeedInteractionStore()
const navigationOpen = ref(false)
const navigationProgress = ref(0)
const navigationSettling = ref(false)
const feedContentRef = ref<HTMLElement | null>(null)
const pageContentRef = ref<HTMLElement | null>(null)
const sourceReaderContentRef = ref<HTMLElement | null>(null)
const detailContentRef = ref<HTMLElement | null>(null)
const detailFrameRef = ref<HTMLIFrameElement | null>(null)
const detailInlineSourceRef = ref<HTMLElement | null>(null)
const sourceTitleTextRef = ref<HTMLElement | null>(null)
const detailProgressTrackRef = ref<HTMLElement | null>(null)
const feedScrollTop = ref(0)
const sourceReaderScrollTop = ref(0)
const detailReaderTouchOffset = ref(0)
const detailReaderStretch = ref(0)
const pageSideOffset = ref(0)
const pageSideStretch = ref(0)
const readerBackDragging = ref(false)
const readerMotionSettling = ref(false)
const viewDragOffset = ref(0)
const viewSettling = ref(false)
const topChromeProgress = ref(1)
const windowWidth = ref(typeof window === 'undefined' ? 1440 : window.innerWidth)
const windowHeight = ref(typeof window === 'undefined' ? 900 : window.innerHeight)
const darkTheme = ref(false)
const refreshWasActive = ref(false)
const feedRefreshSettling = ref(false)
const feedChromeSettling = ref(false)
const feedContentCollapsed = ref(false)
const feedTopPulling = ref(false)
const feedTopPullStartedWithChrome = ref(false)
const refreshStartedWithChrome = ref(false)
const pagePullOffset = ref(0)
const pagePullSettling = ref(false)
const readerSource = ref<ReaderSource | null>(null)
const sourceReaderVisible = ref(false)
const detailItem = ref<FeedItem | null>(null)
const detailLoading = ref(false)
const detailError = ref('')
const detailSourceKind = ref<FeedSourceKind>('subscriptions')
const detailOriginRect = ref<RectSnapshot | null>(null)
const detailSourceItemTargetRect = ref<RectSnapshot | null>(null)
const detailSourceNameOriginRect = ref<RectSnapshot | null>(null)
const detailSourceNameTargetRect = ref<RectSnapshot | null>(null)
const morphingItemId = ref<number | null>(null)
const morphingHeightLockItemId = ref<number | null>(null)
const morphingItemHeight = ref<number | null>(null)
const detailOpenedFromSourceReader = ref(false)
const detailEntryProgress = ref(1)
const detailEntrySettling = ref(false)
const detailBackExitProgress = ref(0)
const detailSourceExitProgress = ref(0)
const detailReturningToFeed = ref(false)
const detailListReturnCommitted = ref(false)
const detailRestoringFromSourceReader = ref(false)
const detailScrollTop = ref(0)
const detailScrollHeight = ref(0)
const detailScrollClientHeight = ref(0)
const detailProgressDragging = ref(false)
const parkedDetailStack = ref<ParkedDetailSnapshot[]>([])
const sourceCatalogEntry = ref<SourceCatalogEntry | null>(null)
const sourceSubscription = ref<Source | null>(null)
const sourceSubscriptionLoading = ref(false)
const sourceNotice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
const sourceTimelinePreloadEnabled = ref(true)

const selectedKeys = computed(() => [route.name?.toString() ?? 'subscriptions'])
const pageTitle = computed(() => route.meta.title?.toString() ?? '订阅')
const sourceReaderMounted = computed(() => readerSource.value !== null)
const sourceReaderOpen = computed(() => readerSource.value !== null && sourceReaderVisible.value)
const detailReaderOpen = computed(() => detailItem.value !== null || detailLoading.value || detailError.value !== '')
const isFeedRoute = computed(() => ['subscriptions', 'recommendations'].includes(route.name?.toString() ?? ''))
const showHomeShortcut = computed(() => !isFeedRoute.value)
const cornerButtonLabel = computed(() => {
  if (detailChromeVisible.value) {
    return '返回'
  }
  return showHomeShortcut.value ? '返回主页' : '打开导航'
})
const activeFeedIndex = computed(() => (route.name === 'recommendations' ? 1 : 0))
const feedPullActive = computed(() => isFeedRoute.value && (feedInteraction.pullActive || feedInteraction.pullOffset > 1))
const sourcePullActive = computed(
  () =>
    sourceReaderOpen.value &&
    feedInteraction.pullViewKey.startsWith('source:') &&
    (feedInteraction.pullActive || feedInteraction.pullOffset > 1),
)
const pullProgress = computed(() => Math.min(feedInteraction.pullOffset / 76, 1))
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
const headerDetailLayoutActive = computed(
  () => detailChromeVisible.value || (detailReaderOpen.value && isFeedRoute.value && feedHeaderReturnProgress.value > 0.001),
)
const pullStatusText = computed(() => feedInteraction.statusText)
const pullStatusMeta = computed(() => feedInteraction.statusMeta)
const pullStatusStyle = computed(() => ({
  opacity: feedPullActive.value ? '1' : '0',
  transform: `translate3d(0, ${Math.round((1 - pullProgress.value) * -10)}px, 0) scale(${(0.96 + pullProgress.value * 0.04).toFixed(3)})`,
}))
const pullIconStyle = computed(() => ({
  transform: feedInteraction.pullRefreshing ? 'none' : `rotate(${Math.round(pullProgress.value * 300)}deg)`,
}))
const feedTabsLayerStyle = computed(() => {
  if (!detailReaderOpen.value) {
    return undefined
  }

  const progress = feedHeaderReturnProgress.value
  return {
    opacity: progress.toFixed(3),
    pointerEvents: progress > 0.96 && !detailBlocksGestures() ? ('auto' as const) : ('none' as const),
    transform: `translate3d(0, ${Math.round((1 - progress) * 7)}px, 0) scale(${(0.98 + progress * 0.02).toFixed(3)})`,
    transition: readerBackDragging.value ? 'none' : undefined,
  }
})
const sourcePullStatusStyle = computed(() => ({
  opacity: sourcePullActive.value ? '1' : '0',
  transform: `translate3d(0, ${Math.round((1 - sourcePullProgress.value) * -10)}px, 0) scale(${(0.96 + sourcePullProgress.value * 0.04).toFixed(3)})`,
}))
const sourcePullIconStyle = computed(() => ({
  transform: feedInteraction.pullRefreshing ? 'none' : `rotate(${Math.round(sourcePullProgress.value * 300)}deg)`,
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
  transform: `translate3d(${Math.round((navigationProgress.value - 1) * (navigationWidth.value + 28))}px, 0, 0)`,
}))
const navigationScrimStyle = computed(() => ({
  opacity: navigationProgress.value,
  pointerEvents: navigationProgress.value > 0.2 ? ('auto' as const) : ('none' as const),
}))
const feedTrackStyle = computed(() => ({
  transform: `translate3d(calc(${-activeFeedIndex.value * 100}% + ${Math.round(viewDragOffset.value)}px), 0, 0)`,
}))
const mainClass = computed(() => ({
  'app-main--feed': isFeedRoute.value,
  'app-main--page': !isFeedRoute.value,
  'app-main--tabs-hidden': feedChromeHidden.value,
  'app-main--refreshing': feedPullActive.value,
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
const sourceReaderStyle = computed(() => {
  const underlayBaseOpacity = darkTheme.value ? 0.74 : 0.54
  const overlayBaseOpacity = darkTheme.value ? 0.48 : 0.34
  return {
    '--feed-header-height': `${feedHeaderHeight.value}px`,
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
    transform: 'translate3d(0, 0, 0)',
    transition: readerBackDragging.value ? 'none' : 'opacity 320ms ease',
  }
})
const detailReaderStyle = computed(() => ({
  transform: `translate3d(0, 0, 0) scaleX(${(1 + detailReaderStretch.value).toFixed(4)})`,
  transition: readerBackDragging.value ? 'none' : undefined,
  transformOrigin: detailReaderStretch.value > 0 ? 'left center' : undefined,
  pointerEvents: detailCommittedListReturn() ? ('none' as const) : ('auto' as const),
  '--detail-overlay-opacity': detailCommittedListReturn() || detailReturningToFeed.value
    ? '0'
    : clamp(
        detailEntryProgress.value * (1 - Math.max(detailBackExitProgress.value, detailSourceExitProgress.value)),
      ).toFixed(3),
}))
const detailSurfaceProgress = computed(() =>
  clamp(detailEntryProgress.value * (1 - Math.max(detailBackExitProgress.value, detailSourceExitProgress.value))),
)
const feedItemPreviewProgress = computed(() => {
  const movingToSourceList =
    sourceReaderVisible.value &&
    detailReaderOpen.value &&
    (detailSourceExitProgress.value > 0 ||
      detailRestoringFromSourceReader.value ||
      (detailOpenedFromSourceReader.value && detailBackExitProgress.value > 0))

  if (movingToSourceList && !detailParkedBehindSource.value) {
    return 0
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
  Math.round((feedHeaderHeight.value + detailSurfaceMargin.value) * topChromeProgress.value),
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
    readerBackDragging.value && (detailBackExitProgress.value > 0 || detailSourceExitProgress.value > 0)
  const committedListReturn = detailCommittedListReturn()

  if (!collapsedTarget) {
    const opacity =
      draggingToList
        ? 1
        : committedListReturn || detailReturningToFeed.value
          ? progress
          : 1 - exitProgress * 0.28
    return {
      width: `${expandedWidth}px`,
      height: `${targetHeight}px`,
      opacity: clamp(opacity).toFixed(3),
      transform: `translate3d(${Math.round(expandedLeft)}px, ${targetTop + Math.round(exitProgress * 18)}px, 0)`,
      transition: readerBackDragging.value ? 'none' : undefined,
      borderRadius: `${Math.round(16 - exitProgress * 4)}px`,
    }
  }

  const width = collapsedTarget.width + (expandedWidth - collapsedTarget.width) * progress
  const height = collapsedTarget.height + (targetHeight - collapsedTarget.height) * progress
  const x = collapsedTarget.left + (expandedLeft - collapsedTarget.left) * progress
  const y = collapsedTarget.top + (targetTop - collapsedTarget.top) * progress
  const radius = Math.round(12 + 4 * progress)
  const minimumSurfaceOpacity = darkTheme.value ? 0.64 : 0.36
  const opacity =
    draggingToList
      ? 1
      : committedListReturn || detailReturningToFeed.value
        ? progress
        : minimumSurfaceOpacity + progress * (1 - minimumSurfaceOpacity)

  return {
    width: `${Math.round(width)}px`,
    height: `${Math.round(height)}px`,
    opacity: clamp(opacity).toFixed(3),
    transform: `translate3d(${Math.round(x)}px, ${Math.round(y)}px, 0)`,
    borderRadius: `${radius}px`,
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
    '--detail-frame-min-height': `${Math.round(detailFrameMinHeight.value)}px`,
    transform:
      sourceExitProgress > 0 ? 'translate3d(0, 0, 0)' : `translate3d(0, ${Math.round((1 - progress) * 8)}px, 0)`,
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
    top: `${Math.round(top)}px`,
    right: `${Math.max(6, Math.round(margin * 0.5))}px`,
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
    transform: `translate3d(0, ${Math.round(progress * -4)}px, 0)`,
    transition: readerBackDragging.value || committedListReturn ? 'none' : undefined,
  }
})
const detailHeaderTitleStyle = computed(() => {
  const progress = detailSurfaceProgress.value
  const opacity = clamp((progress - 0.58) / 0.22) * (1 - feedHeaderReturnProgress.value)
  return {
    opacity: opacity.toFixed(3),
    transform: `translate3d(0, ${Math.round((1 - opacity) * 8)}px, 0)`,
    transition: readerBackDragging.value ? 'none' : undefined,
  }
})
const detailInlineSourceStyle = computed(() => ({
  opacity: (1 - sourceNameMorphProgress.value).toFixed(3),
  transform: 'translate3d(0, 0, 0)',
  transition: readerBackDragging.value ? 'none' : 'opacity 220ms ease',
}))
const detailMorphSourceLabelStyle = computed(() => ({
  opacity: (1 - sourceNameMorphProgress.value).toFixed(3),
  transition: readerBackDragging.value ? 'none' : 'opacity 220ms ease',
}))
const sourceNameMorphProgress = computed(() =>
  clamp(Math.max(detailSourceExitProgress.value, detailOpenedFromSourceReader.value ? detailBackExitProgress.value : 0)),
)
const sourceTitleProgress = computed(() =>
  detailReaderOpen.value && sourceReaderVisible.value ? sourceNameMorphProgress.value : 1,
)
const sourceTitleRevealVisible = computed(
  () =>
    Boolean(readerSource.value) &&
    detailReaderOpen.value &&
    sourceReaderVisible.value &&
    !sourcePullActive.value,
)
const sourceNameMorphVisible = computed(
  () => false,
)
const sourceNameMorphStyle = computed(() => {
  const origin = detailSourceNameOriginRect.value
  const target = detailSourceNameTargetRect.value
  const progress = sourceNameMorphProgress.value
  if (!origin || !target) {
    return {
      opacity: '0',
      transform: 'translate3d(0, 0, 0)',
    }
  }

  const left = origin.left + (target.left - origin.left) * progress
  const top = origin.top + (target.top - origin.top) * progress
  const width = origin.width + (target.width - origin.width) * progress
  const size = 13 + (18 - 13) * progress

  return {
    left: `${Math.round(left)}px`,
    top: `${Math.round(top)}px`,
    width: `${Math.round(width)}px`,
    opacity: '1',
    fontSize: `${size.toFixed(2)}px`,
    transform: 'translate3d(0, 0, 0)',
    transition: readerBackDragging.value
      ? 'none'
      : 'left 360ms cubic-bezier(0.2, 0.8, 0.2, 1), top 360ms cubic-bezier(0.2, 0.8, 0.2, 1), width 360ms cubic-bezier(0.2, 0.8, 0.2, 1), font-size 360ms cubic-bezier(0.2, 0.8, 0.2, 1), opacity 180ms ease',
  }
})
const sourceTitleLayerStyle = computed(() => {
  if (sourceTitleRevealVisible.value) {
    return {
      opacity: '0',
      transform: 'translate3d(0, 0, 0)',
      filter: 'none',
      transition: readerBackDragging.value ? 'none' : 'opacity 120ms ease',
    }
  }

  return {
    opacity: sourceTitleProgress.value.toFixed(3),
    transform: 'translate3d(0, 0, 0)',
    filter: 'none',
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity 220ms ease, transform 220ms cubic-bezier(0.2, 0.8, 0.2, 1)',
  }
})
const sourceTitleTextStyle = computed(() => ({
  display: 'inline-block',
}))
const sourceTitleRevealStyle = computed(() => {
  const progress = clamp(sourceTitleProgress.value / 0.55)
  const left = windowWidth.value <= 720 ? 72 : 80
  const right = windowWidth.value <= 720 ? 104 : 120
  const top = Math.round((feedHeaderHeight.value - 44) / 2)
  return {
    top: `${top}px`,
    left: `${left}px`,
    width: `calc(100vw - ${left + right}px)`,
    opacity: progress.toFixed(3),
    transform: `translate3d(0, ${Math.round((1 - progress) * 12)}px, 0) scale(${(
      0.965 +
      progress * 0.035
    ).toFixed(3)})`,
    filter: `blur(${((1 - progress) * 2.4).toFixed(2)}px)`,
    transition: readerBackDragging.value
      ? 'none'
      : 'opacity 420ms ease, transform 420ms cubic-bezier(0.16, 1, 0.3, 1), filter 420ms ease',
  }
})
const mainStyle = computed(() => {
  const baseStyle = {
    '--feed-header-height': `${feedHeaderHeight.value}px`,
    '--feed-header-space': `${Math.round(feedContentSpace.value)}px`,
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
    transform: `translate3d(0, ${Math.round((progress - 1) * feedHeaderHeight.value)}px, 0)`,
  }
})
const navOpenButtonStyle = computed(() => {
  const progress = feedCornerHidden.value ? 0 : feedHeaderProgress.value
  const settling = feedChromeSettling.value || feedRefreshSettling.value
  return {
    top: `${Math.round((feedHeaderHeight.value - 44) / 2)}px`,
    opacity: progress.toFixed(3),
    pointerEvents: progress > 0.86 && !feedCornerHidden.value ? ('auto' as const) : ('none' as const),
    transform: `translate3d(0, ${Math.round((progress - 1) * feedHeaderHeight.value)}px, 0) scale(${(
      0.92 +
      progress * 0.08
    ).toFixed(3)})`,
    transition: settling
      ? 'transform 800ms cubic-bezier(0.16, 1, 0.3, 1), opacity 800ms ease, visibility 800ms ease, border-color 160ms ease, background 160ms ease'
      : undefined,
    visibility: progress > 0.01 && !feedCornerHidden.value ? ('visible' as const) : ('hidden' as const),
  }
})
const pageContentInnerStyle = computed(() => ({
  transform: `translate3d(${Math.round(pageSideOffset.value)}px, ${Math.round(pagePullOffset.value)}px, 0) scaleX(${(1 + pageSideStretch.value).toFixed(4)})`,
  transformOrigin: pageSideStretch.value > 0 ? 'left center' : undefined,
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
const detailSrcdoc = computed(() => {
  const body = detailHTML.value || `<p>${escapeHTML(detailText.value || '暂无正文。')}</p>`
  return `<!doctype html>
<html>
<head>
<meta charset="utf-8" />
<meta name="viewport" content="width=device-width, initial-scale=1" />
<base target="_blank" />
<style>
  :root { color-scheme: light dark; }
  body {
    margin: 0;
    padding: 0;
    color: #162033;
    font: 16px/1.72 -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
    overflow-wrap: anywhere;
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
${body}
<script>
(() => {
  let startX = 0;
  let startY = 0;
  let tracking = false;
  let notifiedStart = false;
  let intent = null;
  let scrollTicking = false;
  let resizeObserver = null;
  const post = (phase, touch) => {
    window.parent.postMessage({
      type: 'messagefeed-detail-gesture',
      phase,
      startX,
      startY,
      x: touch.clientX,
      y: touch.clientY,
      dx: touch.clientX - startX,
      dy: touch.clientY - startY
    }, '*');
  };
  const ensureStart = (touch) => {
    if (notifiedStart) return;
    notifiedStart = true;
    post('start', touch);
  };
  const currentScrollTop = () => {
    const doc = document.documentElement;
    const body = document.body;
    return window.scrollY || doc.scrollTop || body.scrollTop || 0;
  };
  const scrollMetrics = () => {
    const doc = document.documentElement;
    const body = document.body;
    const scrollHeight = Math.max(doc.scrollHeight || 0, body.scrollHeight || 0);
    const clientHeight = window.innerHeight || doc.clientHeight || body.clientHeight || 0;
    return {
      scrollTop: currentScrollTop(),
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
  window.addEventListener('scroll', () => {
    if (scrollTicking) return;
    scrollTicking = true;
    requestAnimationFrame(() => {
      scrollTicking = false;
      postScrollMetrics();
    });
  }, { passive: true });
  window.addEventListener('resize', () => requestAnimationFrame(postScrollMetrics), { passive: true });
  window.addEventListener('message', (event) => {
    if (event.data?.type !== 'messagefeed-detail-scroll-to') return;
    const top = Number(event.data.top || 0);
    if (!Number.isFinite(top)) return;
    window.scrollTo({ top: Math.max(0, top), behavior: 'auto' });
    requestAnimationFrame(postScrollMetrics);
  });
  window.addEventListener('load', () => {
    requestAnimationFrame(() => {
      postScrollMetrics();
      if ('ResizeObserver' in window) {
        resizeObserver = new ResizeObserver(() => requestAnimationFrame(postScrollMetrics));
        resizeObserver.observe(document.documentElement);
        resizeObserver.observe(document.body);
      }
      window.setTimeout(postScrollMetrics, 180);
      window.setTimeout(postScrollMetrics, 520);
    });
  });
  window.addEventListener('touchstart', (event) => {
    if (event.touches.length !== 1) return;
    startX = event.touches[0].clientX;
    startY = event.touches[0].clientY;
    tracking = true;
    notifiedStart = false;
    intent = null;
  }, { passive: true });
  window.addEventListener('touchmove', (event) => {
    if (!tracking || event.touches.length !== 1) return;
    const touch = event.touches[0];
    const dx = touch.clientX - startX;
    const dy = touch.clientY - startY;
    if (!intent) {
      const absX = Math.abs(dx);
      const absY = Math.abs(dy);
      if (absX > 16 && absX > absY * 1.55) {
        intent = 'horizontal';
      } else {
        return;
      }
    }
    ensureStart(touch);
    post('move', touch);
  }, { passive: true });
  window.addEventListener('touchcancel', (event) => {
    const touch = event.changedTouches[0];
    if (tracking && touch && notifiedStart) post('cancel', touch);
    tracking = false;
    notifiedStart = false;
    intent = null;
  }, { passive: true });
  window.addEventListener('touchend', (event) => {
    const touch = event.changedTouches[0];
    if (!touch) return;
    if (tracking && notifiedStart) post('end', touch);
    tracking = false;
    notifiedStart = false;
    intent = null;
  }, { passive: true });
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
const viewSwitchDistance = 86
const directionLockRatio = 1.25
const navigationDragRatio = 1.1
const viewDirectionLockRatio = 1.9
const topPullDirectionLockRatio = 1.18
const viewDragThreshold = 16

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
let trackingBackSwipe = false
let navigationDragStarted = false
let backSwipeTarget: 'detail' | 'source' | 'page' | null = null
let backSwipeIntent: 'back' | 'source' | 'blocked' | null = null
let suppressNextClick = false
let suppressClickTimer = 0
let viewSwipeTimer = 0
let navigationTimer = 0
let sourceNoticeTimer = 0
let readerMotionTimer = 0
let detailEntryTimer = 0
let morphingHeightUnlockTimer = 0
let hiddenSourceCleanupTimer = 0
let feedRefreshSettleTimer = 0
let feedChromeSettleTimer = 0
let pagePullSettleTimer = 0
let lastScrollY = typeof window === 'undefined' ? 0 : window.scrollY
let lastFeedScrollTop = 0
let lastPageScrollTop = 0
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
  trackingBackSwipeCandidate = false
  trackingEdgeSwipe = false
  trackingNavigationClose = false
  trackingViewSwipe = false
  trackingBackSwipe = false
  navigationDragStarted = false
  backSwipeTarget = null
  backSwipeIntent = null
}

function isInteractiveTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select, [role="button"]'))
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

function handleMenuClick(key: string) {
  const item = managementItems.find((navItem) => navItem.key === key)
  if (item) {
    router.push(item.path)
    closeNavigation()
  }
}

function goHome(closePanel = navigationVisible.value) {
  router.push('/recommendations')
  topChromeProgress.value = 1
  feedContentCollapsed.value = false
  viewDragOffset.value = 0
  viewSettling.value = false
  if (closePanel) {
    closeNavigation()
  }
}

function handleCornerButtonClick() {
  if (detailChromeVisible.value) {
    collapseItemReader()
    return
  }

  if (showHomeShortcut.value) {
    goHome(false)
    return
  }

  openNavigation()
}

function navigateTo(path: string) {
  viewSettling.value = true
  viewDragOffset.value = 0
  router.push(path)
  window.clearTimeout(viewSwipeTimer)
  viewSwipeTimer = window.setTimeout(() => {
    viewSettling.value = false
  }, 260)
}

function clamp(value: number, min = 0, max = 1) {
  return Math.min(Math.max(value, min), max)
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

function showSourceNotice(type: 'success' | 'warning', message: string) {
  sourceNotice.value = { type, message }
  window.clearTimeout(sourceNoticeTimer)
  sourceNoticeTimer = window.setTimeout(() => {
    sourceNotice.value = null
  }, 2600)
}

async function fetchNow(source: Source) {
  try {
    await fetchSource(source.id)
  } catch (err) {
    showSourceNotice('warning', formatAPIError(err))
  }
}

async function loadSourceReaderSubscription(source: ReaderSource) {
  sourceSubscriptionLoading.value = true
  try {
    const [sources, catalogResult] = await Promise.all([
      listSources(),
      listSourceCatalog({ limit: 200, offset: 0 }),
    ])
    const directSource = sources.find((item) => item.id === source.id)
    const catalogEntry =
      catalogResult.entries.find((entry) => entry.id === source.id) ??
      catalogResult.entries.find((entry) => entry.source_id === source.id) ??
      catalogResult.entries.find((entry) => entry.name === source.name)
    const catalogSource = catalogEntry?.source_id
      ? sources.find((item) => item.id === catalogEntry.source_id)
      : undefined

    sourceCatalogEntry.value = catalogEntry ?? null
    sourceSubscription.value = directSource ?? catalogSource ?? null
  } catch (err) {
    showSourceNotice('warning', formatAPIError(err))
  } finally {
    sourceSubscriptionLoading.value = false
  }
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

function captureDetailSourceTransitionRects(retry = 6) {
  nextTick(() => {
    requestAnimationFrame(() => {
      const itemRect = snapshotElementRect(findSourceFeedItemElement(detailItem.value?.id))
      const sourceOriginRect = snapshotElementRect(detailInlineSourceRef.value)
      const sourceTargetRect = snapshotElementRect(sourceTitleTextRef.value)

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

      if (retry > 0 && (!itemRect || !sourceOriginRect || !sourceTargetRect)) {
        window.setTimeout(() => captureDetailSourceTransitionRects(retry - 1), 80)
      }
    })
  })
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

function clearHiddenSourceReader() {
  if (sourceReaderVisible.value) {
    return
  }
  readerSource.value = null
  sourceCatalogEntry.value = null
  sourceSubscription.value = null
  sourceNotice.value = null
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

  sourceReaderVisible.value = true
  captureDetailSourceTransitionRects()
}

function settleReaderMotion(duration = 260, done?: () => void) {
  readerBackDragging.value = false
  readerMotionSettling.value = true
  window.clearTimeout(readerMotionTimer)
  readerMotionTimer = window.setTimeout(() => {
    readerMotionSettling.value = false
    done?.()
  }, duration)
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
      }, 360)
    })
  })
}

function resetDetailTransition() {
  detailEntryProgress.value = 1
  detailEntrySettling.value = false
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = 0
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  detailRestoringFromSourceReader.value = false
  detailSourceItemTargetRect.value = null
  detailSourceNameOriginRect.value = null
  detailSourceNameTargetRect.value = null
}

function detailCommittedListReturn() {
  return detailReaderOpen.value && detailListReturnCommitted.value && !readerBackDragging.value
}

function hasDetailParkedBehindSource() {
  return (
    detailReaderOpen.value &&
    sourceReaderVisible.value &&
    detailListReturnCommitted.value &&
    !detailReturningToFeed.value &&
    detailSourceExitProgress.value >= 0.99
  )
}

function hasParkedDetailSourceState() {
  return (
    detailReaderOpen.value &&
    sourceReaderVisible.value &&
    detailListReturnCommitted.value &&
    !detailReturningToFeed.value
  )
}

function snapshotParkedDetail(): ParkedDetailSnapshot | null {
  if (!detailItem.value || !hasParkedDetailSourceState()) {
    return null
  }

  return {
    item: { ...detailItem.value },
    sourceKind: detailSourceKind.value,
    originRect: detailOriginRect.value ? { ...detailOriginRect.value } : null,
    sourceItemTargetRect: detailSourceItemTargetRect.value ? { ...detailSourceItemTargetRect.value } : null,
    sourceNameOriginRect: detailSourceNameOriginRect.value ? { ...detailSourceNameOriginRect.value } : null,
    sourceNameTargetRect: detailSourceNameTargetRect.value ? { ...detailSourceNameTargetRect.value } : null,
    morphingItemHeight: morphingItemHeight.value,
    scrollTop: detailScrollTop.value,
  }
}

function pushParkedDetailSnapshot() {
  const snapshot = snapshotParkedDetail()
  if (!snapshot) {
    return
  }

  parkedDetailStack.value.push(snapshot)
}

function restorePreviousParkedDetail() {
  const snapshot = parkedDetailStack.value.pop()
  if (!snapshot) {
    return false
  }

  detailItem.value = snapshot.item
  detailError.value = ''
  detailLoading.value = false
  detailSourceKind.value = snapshot.sourceKind
  detailOpenedFromSourceReader.value = false
  detailOriginRect.value = snapshot.originRect
  detailSourceItemTargetRect.value = snapshot.sourceItemTargetRect
  detailSourceNameOriginRect.value = snapshot.sourceNameOriginRect
  detailSourceNameTargetRect.value = snapshot.sourceNameTargetRect
  detailScrollTop.value = snapshot.scrollTop
  lastDetailScrollTop = snapshot.scrollTop
  morphingItemId.value = null
  morphingHeightLockItemId.value = null
  morphingItemHeight.value = snapshot.morphingItemHeight
  detailEntryProgress.value = 1
  detailEntrySettling.value = false
  detailReaderTouchOffset.value = 0
  detailReaderStretch.value = 0
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = 1
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = true
  detailRestoringFromSourceReader.value = false
  sourceReaderVisible.value = true
  return true
}

function detailBlocksGestures() {
  return detailReaderOpen.value && !detailCommittedListReturn()
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

async function toggleSourceReaderSubscription() {
  if (!readerSource.value || sourceSubscriptionLoading.value) {
    return
  }

  sourceSubscriptionLoading.value = true
  try {
    if (sourceSubscription.value) {
      const nextStatus = sourceSubscription.value.status === 'active' ? 'inactive' : 'active'
      const updated = await updateSourceStatus(sourceSubscription.value.id, nextStatus)
      sourceSubscription.value = updated
      if (updated.status === 'active') {
        await fetchNow(updated)
      }
      showSourceNotice('success', `${updated.name} 已${updated.status === 'active' ? '开启' : '关闭'}`)
      await loadSourceReaderSubscription(readerSource.value)
      return
    }

    if (!sourceCatalogEntry.value) {
      showSourceNotice('warning', '该来源不在官方目录中，暂不支持直接开启')
      return
    }

    const result = await importCatalogSources([sourceCatalogEntry.value.id])
    const imported = result.sources[0]
    if (imported) {
      sourceSubscription.value = imported
      await fetchNow(imported)
    }
    showSourceNotice('success', `${sourceCatalogEntry.value.name} 已开启`)
    await loadSourceReaderSubscription(readerSource.value)
  } catch (err) {
    showSourceNotice('warning', formatAPIError(err))
  } finally {
    sourceSubscriptionLoading.value = false
  }
}

function openSourceReader(source: ReaderSource, options: { visible?: boolean } = {}) {
  window.clearTimeout(hiddenSourceCleanupTimer)
  const nextVisible = options.visible ?? true

  if (readerSource.value?.id === source.id && readerSource.value.kind === source.kind) {
    sourceReaderVisible.value = nextVisible
    if (nextVisible && detailReaderOpen.value) {
      captureDetailSourceTransitionRects()
    }
    return
  }

  readerSource.value = source
  sourceReaderVisible.value = nextVisible
  sourceCatalogEntry.value = null
  sourceSubscription.value = null
  sourceNotice.value = null
  sourceReaderScrollTop.value = 0
  nextTick(() => {
    if (sourceReaderContentRef.value) {
      sourceReaderContentRef.value.scrollTop = 0
    }
    if (nextVisible && detailReaderOpen.value) {
      captureDetailSourceTransitionRects()
    }
  })
  void loadSourceReaderSubscription(source)
}

async function openItemReader(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
  window.clearTimeout(morphingHeightUnlockTimer)
  window.clearTimeout(hiddenSourceCleanupTimer)
  const openedFromSourceReader =
    sourceReaderOpen.value && readerSource.value?.id === item.source_id && readerSource.value.kind === sourceKind
  if (openedFromSourceReader && hasParkedDetailSourceState()) {
    pushParkedDetailSnapshot()
  } else if (!openedFromSourceReader) {
    parkedDetailStack.value = []
  }
  detailError.value = ''
  detailLoading.value = true
  detailItem.value = item
  detailSourceKind.value = sourceKind
  detailOpenedFromSourceReader.value = openedFromSourceReader
  morphingItemId.value = item.id
  morphingHeightLockItemId.value = item.id
  feedChromeSettling.value = false
  feedTopPulling.value = false
  feedContentCollapsed.value = false
  topChromeProgress.value = 1
  window.clearTimeout(feedChromeSettleTimer)
  detailReaderTouchOffset.value = 0
  detailReaderStretch.value = 0
  detailBackExitProgress.value = 0
  detailSourceExitProgress.value = 0
  detailReturningToFeed.value = false
  detailListReturnCommitted.value = false
  detailSourceItemTargetRect.value = openedFromSourceReader ? snapshotRect(originRect) : null
  detailSourceNameOriginRect.value = null
  detailSourceNameTargetRect.value = null
  detailScrollTop.value = 0
  detailScrollHeight.value = 0
  detailScrollClientHeight.value = 0
  detailProgressDragging.value = false
  activeDetailProgressPointerId = null
  lastDetailScrollTop = 0
  sourceReaderVisible.value = openedFromSourceReader
  startDetailEntry(originRect)
  morphingItemHeight.value = detailOriginRect.value?.height ?? null
  if (openedFromSourceReader) {
    captureDetailSourceTransitionRects()
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
    })
  }
}

function closeSourceReader() {
  if (hasDetailParkedBehindSource()) {
    restoreDetailFromParkedSource()
    return
  }

  if (!detailReaderOpen.value && parkedDetailStack.value.length > 0 && restorePreviousParkedDetail()) {
    restoreDetailFromParkedSource()
    return
  }

  if (sourceReaderOpen.value) {
    readerBackDragging.value = false
    sourceReaderVisible.value = false
    parkedDetailStack.value = []
    scheduleHiddenSourceReaderCleanup(340)
    return
  }

  readerSource.value = null
  sourceReaderVisible.value = false
  sourceCatalogEntry.value = null
  sourceSubscription.value = null
  sourceNotice.value = null
  parkedDetailStack.value = []
}

function restoreDetailFromParkedSource(duration = 360) {
  if (!detailReaderOpen.value) {
    closeSourceReader()
    return
  }

  suppressFollowingClick()
  window.clearTimeout(detailEntryTimer)
  window.clearTimeout(morphingHeightUnlockTimer)
  const itemRect = snapshotElementRect(findSourceFeedItemElement(detailItem.value?.id))
  if (itemRect) {
    detailSourceItemTargetRect.value = itemRect
  }
  if (detailItem.value?.id) {
    morphingItemId.value = detailItem.value.id
    morphingHeightLockItemId.value = detailItem.value.id
    morphingItemHeight.value = detailSourceItemTargetRect.value?.height ?? morphingItemHeight.value
  }

  const startProgress = detailSourceExitProgress.value > 0.001 ? detailSourceExitProgress.value : 1
  readerBackDragging.value = false
  detailEntrySettling.value = true
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
    parkedDetailStack.value = []
    sourceReaderVisible.value = false
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    restoreMorphingItemContent()
    scheduleHiddenSourceReaderCleanup()
  }, duration)
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
  }, duration)
}

function closeItemReader() {
  detailItem.value = null
  detailError.value = ''
  detailLoading.value = false
  restoreMorphingItemContent()
  detailOpenedFromSourceReader.value = false
  detailRestoringFromSourceReader.value = false
  detailScrollTop.value = 0
  detailScrollHeight.value = 0
  detailScrollClientHeight.value = 0
  detailProgressDragging.value = false
  activeDetailProgressPointerId = null
  detailReaderTouchOffset.value = 0
  detailReaderStretch.value = 0
  resetDetailTransition()
  if (!sourceReaderVisible.value) {
    parkedDetailStack.value = []
    scheduleHiddenSourceReaderCleanup()
  }
}

function collapseItemReader(duration = 360) {
  if (!detailReaderOpen.value) {
    return
  }

  suppressFollowingClick()
  const shouldRestorePreviousParkedDetail = detailOpenedFromSourceReader.value && parkedDetailStack.value.length > 0
  if (detailOpenedFromSourceReader.value && readerSource.value) {
    sourceReaderVisible.value = true
  }
  readerBackDragging.value = false
  detailEntrySettling.value = true
  detailReaderTouchOffset.value = 0
  detailReaderStretch.value = 0
  detailBackExitProgress.value = 1
  detailSourceExitProgress.value = 0
  detailRestoringFromSourceReader.value = false
  detailReturningToFeed.value = !detailOpenedFromSourceReader.value
  detailListReturnCommitted.value = true
  window.clearTimeout(detailEntryTimer)
  detailEntryTimer = window.setTimeout(() => {
    if (shouldRestorePreviousParkedDetail && restorePreviousParkedDetail()) {
      return
    }
    closeItemReader()
  }, duration)
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
  window.clearTimeout(detailEntryTimer)
  detailEntryTimer = window.setTimeout(() => {
    detailEntrySettling.value = false
    if (shouldHideSourceAfterRestore) {
      sourceReaderVisible.value = false
    }
  }, duration)
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
  }, duration)
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
  sourceReaderVisible.value = true
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
  }, duration)
}

function noopTopPullStart(_startedWithVisibleChrome: boolean) {}

function noopTopPullMove(_distance: number) {}

function noopTopPullEnd(_shouldRefresh: boolean) {}

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
  }, 240)
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
  }, 240)
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

function isLeftHalf(clientX: number) {
  return clientX <= windowWidth.value / 2
}

function isRightHalf(clientX: number) {
  return clientX > windowWidth.value / 2
}

function canStartViewSwipe(clientX: number) {
  if (!isFeedRoute.value || navigationVisible.value || sourceReaderOpen.value || detailBlocksGestures()) {
    return false
  }

  if (activeFeedIndex.value === 0) {
    return isRightHalf(clientX)
  }

  return isLeftHalf(clientX)
}

function canStartNavigationOpen(clientX: number) {
  return (
    route.name === 'subscriptions' &&
    !navigationVisible.value &&
    !sourceReaderOpen.value &&
    !detailBlocksGestures() &&
    isLeftHalf(clientX)
  )
}

function isBackHorizontalSwipe(deltaX: number, deltaY: number) {
  return Math.abs(deltaX) > viewDragThreshold && Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
}

function blockedSwipeStretch(deltaX: number) {
  return Math.min(Math.log1p(Math.abs(deltaX)) / 72, 0.045)
}

function backSwipeVisualOffset(deltaX: number) {
  const limit = Math.round(windowWidth.value * 0.72)
  return Math.max(-limit, Math.min(limit, deltaX))
}

function resetBackSwipeOffset() {
  detailReaderTouchOffset.value = 0
  detailSourceExitProgress.value = 0
  detailRestoringFromSourceReader.value = false
  detailReaderStretch.value = 0
  pageSideOffset.value = 0
  pageSideStretch.value = 0
  readerBackDragging.value = false
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

function beginBackSwipeIfAllowed(deltaX: number, deltaY: number) {
  if (!trackingBackSwipeCandidate || !isBackHorizontalSwipe(deltaX, deltaY)) {
    return false
  }

  if (deltaX > 0 && !isLeftHalf(touchStartX)) {
    trackingBackSwipeCandidate = false
    backSwipeIntent = 'blocked'
    return false
  }

  if (deltaX < 0 && !isRightHalf(touchStartX)) {
    trackingBackSwipeCandidate = false
    backSwipeIntent = 'blocked'
    return false
  }

  trackingBackSwipe = true
  readerBackDragging.value = true
  detailEntrySettling.value = false
  if (deltaX > 0) {
    backSwipeIntent = 'back'
    if (backSwipeTarget === 'source' && hasParkedDetailSourceState()) {
      detailRestoringFromSourceReader.value = true
    }
    detailReturningToFeed.value = backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value
    if (backSwipeTarget === 'detail' && detailOpenedFromSourceReader.value && readerSource.value) {
      sourceReaderVisible.value = true
      captureDetailSourceTransitionRects()
    }
  } else if (backSwipeTarget === 'detail' && detailItem.value?.source_id && !detailOpenedFromSourceReader.value) {
    backSwipeIntent = 'source'
    showSourceReaderUnderDetail()
  } else {
    backSwipeIntent = 'blocked'
  }
  trackingBackSwipeCandidate = false
  trackingEdgeSwipeCandidate = false
  trackingNavigationCloseCandidate = false
  trackingViewSwipeCandidate = false
  return true
}

function updateBackSwipe(deltaX: number, deltaY: number) {
  beginBackSwipeIfAllowed(deltaX, deltaY)

  if (!trackingBackSwipe) {
    return false
  }

  suppressFollowingClick()
  if (deltaX > 0) {
    backSwipeIntent = 'back'
    if (backSwipeTarget === 'source' && (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value)) {
      detailRestoringFromSourceReader.value = true
    } else {
      detailSourceExitProgress.value = 0
    }
    detailReturningToFeed.value = backSwipeTarget === 'detail' && !detailOpenedFromSourceReader.value
    if (backSwipeTarget === 'detail' && detailOpenedFromSourceReader.value && readerSource.value) {
      sourceReaderVisible.value = true
      captureDetailSourceTransitionRects()
    }
  } else if (
    backSwipeTarget === 'detail' &&
    detailItem.value?.source_id &&
    isRightHalf(touchStartX) &&
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
  const stretch = blockedSwipeStretch(deltaX)

  detailReaderStretch.value = 0
  pageSideStretch.value = 0

  if (intent === 'back' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = clamp(Math.max(0, offset) / Math.max(220, windowWidth.value * 0.52))
  } else if (intent === 'back' && backSwipeTarget === 'source') {
    if (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value) {
      detailRestoringFromSourceReader.value = true
      detailReaderTouchOffset.value = 0
      detailBackExitProgress.value = 0
      detailSourceExitProgress.value = 1 - clamp(Math.max(0, offset) / Math.max(220, windowWidth.value * 0.52))
    }
  } else if (intent === 'back' && backSwipeTarget === 'page') {
    pageSideOffset.value = Math.max(0, offset)
  } else if (intent === 'source' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = clamp(Math.max(0, -offset) / Math.max(220, windowWidth.value * 0.52))
  } else if (intent === 'blocked' && backSwipeTarget === 'detail') {
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailReaderStretch.value = stretch
  } else if (intent === 'blocked' && backSwipeTarget === 'source') {
    detailSourceExitProgress.value = hasParkedDetailSourceState() ? 1 : 0
  } else if (intent === 'blocked' && backSwipeTarget === 'page') {
    pageSideOffset.value = 0
    pageSideStretch.value = stretch
  }

  return true
}

function finishBackSwipe(deltaX: number, _deltaY: number) {
  const target = backSwipeTarget
  const intent = backSwipeIntent
  const shouldCommit =
    intent === 'back' && target === 'detail'
      ? deltaX > 0 && (detailBackExitProgress.value >= 0.42 || deltaX >= viewSwitchDistance)
      : intent === 'source' && target === 'detail'
        ? deltaX < 0 && (detailSourceExitProgress.value >= 0.42 || Math.abs(deltaX) >= viewSwitchDistance)
        : intent === 'back'
          ? deltaX > 0 && Math.abs(deltaX) >= viewSwitchDistance
          : false

  if (!shouldCommit) {
    if (intent === 'back' && target === 'detail') {
      restoreItemReaderExpansion()
      return
    }
    if (intent === 'source' && target === 'detail') {
      restoreDetailFromSourceSwipe()
      return
    }
    if (intent === 'back' && target === 'source' && (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value)) {
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
    if (hasParkedDetailSourceState() || detailRestoringFromSourceReader.value) {
      restoreDetailFromParkedSource()
      return
    }

    readerBackDragging.value = false
    detailRestoringFromSourceReader.value = false
    sourceReaderVisible.value = false
    if (!detailReaderOpen.value) {
      scheduleHiddenSourceReaderCleanup(340)
    }
    return
  }
  if (intent === 'back' && target === 'page') {
    pageSideOffset.value = windowWidth.value
    settleReaderMotion(220, () => {
      pageSideOffset.value = 0
    })
    goHome(false)
    return
  }

  resetBackSwipeOffset()
}

function finishViewSwipe(nextPath: string | null) {
  viewSettling.value = true
  window.clearTimeout(viewSwipeTimer)
  if (nextPath) {
    router.push(nextPath)
  }
  viewDragOffset.value = 0
  viewSwipeTimer = window.setTimeout(() => {
    viewSettling.value = false
  }, 260)
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

  if (sourceReaderOpen.value) {
    trackingBackSwipeCandidate = true
    backSwipeTarget = 'source'
    return
  }
  if (detailBlocksGestures()) {
    trackingBackSwipeCandidate = true
    backSwipeTarget = 'detail'
    if (sourceTimelinePreloadEnabled.value) {
      prepareDetailSourceReaderPreload()
    }
    return
  }
  if (!isFeedRoute.value && !navigationVisible.value) {
    trackingBackSwipeCandidate = true
    backSwipeTarget = 'page'
  }

  trackingEdgeSwipeCandidate = canStartNavigationOpen(touchStartX)
  trackingNavigationCloseCandidate =
    navigationOpen.value && touchStartX <= Math.max(navigationWidth.value + 40, windowWidth.value / 2)
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
    const handledBackSwipe = updateBackSwipe(deltaX, deltaY)
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
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingNavigationCloseCandidate && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    trackingNavigationClose = true
    trackingNavigationCloseCandidate = false
    trackingViewSwipeCandidate = false
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
  trackingNavigationCloseCandidate =
    navigationOpen.value && touchStartX <= Math.max(navigationWidth.value + 40, windowWidth.value / 2)
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
    navigationDragStarted = true
    navigationSettling.value = false
  }

  if (trackingNavigationCloseCandidate && deltaX < -8 && isNavigationDrag(deltaX, deltaY)) {
    trackingNavigationClose = true
    trackingNavigationCloseCandidate = false
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
  if (!trackingEdgeSwipe && !trackingNavigationClose && !trackingViewSwipe && !trackingBackSwipe) {
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

  if (canStartNavigationOpen(event.clientX)) {
    return
  }

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

function scrollDetailFrameTo(top: number) {
  detailFrameRef.value?.contentWindow?.postMessage(
    {
      type: 'messagefeed-detail-scroll-to',
      top: Math.max(0, top),
    },
    '*',
  )
}

function updateDetailProgressFromPointer(clientY: number) {
  const track = detailProgressTrackRef.value
  if (!track || detailScrollMax.value <= 0) {
    return
  }

  const rect = track.getBoundingClientRect()
  const progress = clamp((clientY - rect.top) / Math.max(1, rect.height))
  const nextScrollTop = detailScrollMax.value * progress
  detailScrollTop.value = nextScrollTop
  lastDetailScrollTop = nextScrollTop
  scrollDetailFrameTo(nextScrollTop)
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
    if (detailScrollTop.value > 0) {
      scrollDetailFrameTo(detailScrollTop.value)
    }
  })
}

function handleMessage(event: MessageEvent) {
  if (detailCommittedListReturn()) {
    return
  }

  if (event.data?.type === 'messagefeed-detail-scroll' && detailReaderOpen.value) {
    const payload = event.data as { scrollTop?: number; scrollHeight?: number; clientHeight?: number }
    const current = Number(payload.scrollTop ?? 0)
    if (Number.isFinite(current)) {
      detailScrollTop.value = current
      updateTopTabsByScroll(current, lastDetailScrollTop)
      lastDetailScrollTop = current
    }
    const scrollHeight = Number(payload.scrollHeight ?? 0)
    const clientHeight = Number(payload.clientHeight ?? 0)
    if (Number.isFinite(scrollHeight)) {
      detailScrollHeight.value = Math.max(0, scrollHeight)
    }
    if (Number.isFinite(clientHeight)) {
      detailScrollClientHeight.value = Math.max(0, clientHeight)
    }
    return
  }

  if (event.data?.type !== 'messagefeed-detail-gesture' || !detailReaderOpen.value) {
    return
  }

  const payload = event.data as {
    phase?: 'start' | 'move' | 'end' | 'cancel'
    startX?: number
    startY?: number
    dx?: number
    dy?: number
  }
  const startX = Number(payload.startX ?? 0)
  const startY = Number(payload.startY ?? 0)
  const deltaX = Number(payload.dx ?? 0)
  const deltaY = Number(payload.dy ?? 0)

  if (payload.phase === 'start') {
    touchStartX = startX
    touchStartY = startY
    trackingBackSwipeCandidate = true
    trackingBackSwipe = false
    backSwipeTarget = 'detail'
    if (sourceTimelinePreloadEnabled.value) {
      prepareDetailSourceReaderPreload()
    }
    return
  }

  if (payload.phase === 'move') {
    updateBackSwipe(deltaX, deltaY)
    return
  }

  if (payload.phase === 'end') {
    if (trackingBackSwipe) {
      finishBackSwipe(deltaX, deltaY)
      resetGestureTracking()
      return
    }
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
    return
  }

  feedChromeSettling.value = true
  window.clearTimeout(feedChromeSettleTimer)
  if (visible) {
    feedContentCollapsed.value = false
  }
  topChromeProgress.value = nextProgress
  feedChromeSettleTimer = window.setTimeout(() => {
    feedChromeSettling.value = false
  }, 800)
}

function currentContentScrollTop() {
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
  feedChromeSettling.value = false
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
    topChromeProgress.value = 1
    feedContentCollapsed.value = false
    return
  }

  topChromeProgress.value = clamp(topPullStartProgress - distance / feedHeaderHeight.value)
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
    feedContentCollapsed.value = !startedWithChrome
    if (startedWithChrome) {
      topChromeProgress.value = 1
    }
    return
  }

  if (topChromeProgress.value <= 0.04) {
    feedContentCollapsed.value = true
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

  if (feedPullActive.value || feedTopPulling.value || feedChromeSettling.value) {
    return
  }

  const delta = current - previous
  if (Math.abs(delta) < 3 || current < 0) {
    return
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
}

function handlePageContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  updateTopTabsByScroll(current, lastPageScrollTop)
  lastPageScrollTop = current
}

function handleSourceReaderScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  sourceReaderScrollTop.value = target.scrollTop
}

function handleDetailContentScroll(event: Event) {
  const target = event.currentTarget as HTMLElement | null
  if (!target) {
    return
  }

  const current = target.scrollTop
  detailScrollTop.value = current
  updateTopTabsByScroll(current, lastDetailScrollTop)
  lastDetailScrollTop = current
}

function resetPageTopPullTracking() {
  pageTopPullDistance = 0
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
  }, 360)
}

function handlePageTouchStart(event: TouchEvent) {
  if (
    isFeedRoute.value ||
    event.touches.length !== 1 ||
    currentContentScrollTop() > 0 ||
    isInteractiveTarget(event.target)
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
    pagePullSettling.value = false
    window.clearTimeout(pagePullSettleTimer)
    pagePullOffset.value = pageRubberBandOffset(deltaY)
  }
}

function handlePageTouchEnd() {
  if (trackingPageTopPull) {
    feedTopPulling.value = false
    setTopChromeVisible(true)
    settlePagePullOffset()
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
  },
)

watch(
  () => feedInteraction.pullRefreshing,
  (refreshing) => {
    if (refreshing) {
      refreshWasActive.value = true
      if (feedTopPullStartedWithChrome.value) {
        refreshStartedWithChrome.value = true
      }
    }
  },
)

watch(
  feedPullActive,
  (active) => {
    if (!active && refreshWasActive.value) {
      refreshStartedWithChrome.value = false
      feedTopPullStartedWithChrome.value = false
      feedContentCollapsed.value = true
      setTopChromeVisible(false)
      refreshWasActive.value = false
      feedRefreshSettling.value = true
      window.clearTimeout(feedRefreshSettleTimer)
      feedRefreshSettleTimer = window.setTimeout(() => {
        feedRefreshSettling.value = false
      }, 260)
    }

    if (!active && !refreshWasActive.value) {
      refreshStartedWithChrome.value = false
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
  window.addEventListener('keydown', handleKeydown)
  window.addEventListener('resize', handleResize)
  window.addEventListener('message', handleMessage)
  window.addEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
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
  window.removeEventListener('keydown', handleKeydown)
  window.removeEventListener('resize', handleResize)
  window.removeEventListener('message', handleMessage)
  window.removeEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
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
  window.clearTimeout(navigationTimer)
  window.clearTimeout(feedRefreshSettleTimer)
  window.clearTimeout(feedChromeSettleTimer)
  window.clearTimeout(pagePullSettleTimer)
  window.clearTimeout(suppressClickTimer)
  window.clearTimeout(sourceNoticeTimer)
  window.clearTimeout(readerMotionTimer)
  window.clearTimeout(detailEntryTimer)
  window.clearTimeout(morphingHeightUnlockTimer)
  window.clearTimeout(hiddenSourceCleanupTimer)
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
      <IconArrowLeft v-if="detailChromeVisible" />
      <IconHome v-else-if="showHomeShortcut" />
      <IconMenuUnfold v-else />
    </button>

    <button
      v-if="navigationVisible"
      class="nav-scrim"
      type="button"
      aria-label="关闭导航"
      :style="navigationScrimStyle"
      @click="closeNavigation"
    />

    <aside
      v-if="navigationVisible"
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
          @click="router.push('/settings'); closeNavigation()"
        >
          <IconSettings />
        </button>
      </div>
    </aside>

    <main class="app-main" :class="mainClass" :style="mainStyle">
      <header class="app-header" :class="headerClass" :style="headerStyle">
        <div class="app-header-slot" :class="{ 'app-header-slot--feed': isFeedRoute || detailChromeVisible }">
          <div v-if="isFeedRoute || detailChromeVisible" class="app-header-feed-stack">
            <div
              v-if="detailReaderOpen"
              class="feed-header-layer feed-header-layer--detail"
              :class="{ 'feed-header-layer--hidden': !detailHeaderVisible }"
            >
              <div v-if="detailItem" class="detail-header-title" :style="detailHeaderTitleStyle">
                <span>{{ detailItem.title }}</span>
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
                <IconRefresh />
              </span>
              <div class="feed-refresh-header__copy">
                <div class="feed-refresh-header__title">{{ pullStatusText }}</div>
                <div class="feed-refresh-header__meta">{{ pullStatusMeta }}</div>
              </div>
            </div>
          </div>
          <div v-else>
            <h1>{{ pageTitle }}</h1>
          </div>
        </div>
      </header>

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
                :active="route.name === 'subscriptions'"
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
                :active="route.name === 'recommendations'"
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
            <component :is="Component" @open-source="openSourceReader" />
          </router-view>
        </div>
      </section>
    </main>

    <section
      v-if="sourceReaderMounted && readerSource"
      class="reader-overlay reader-overlay--source"
      :class="{ 'reader-overlay--under-detail': sourceReaderUnderDetail }"
      :style="sourceReaderStyle"
    >
      <div
        v-if="sourceNotice"
        class="sources-toast reader-toast"
        :class="`sources-toast--${sourceNotice.type}`"
        role="status"
        aria-live="polite"
      >
        {{ sourceNotice.message }}
      </div>
      <header class="reader-overlay__header reader-overlay__header--source">
        <button class="reader-back-button" type="button" aria-label="返回" @click="closeSourceReader">
          <IconArrowLeft />
        </button>
        <div class="reader-overlay__source-stack">
          <div class="reader-source-layer" :class="{ 'reader-source-layer--hidden': sourcePullActive }">
            <div class="reader-overlay__title" :style="sourceTitleLayerStyle">
              <span ref="sourceTitleTextRef" :style="sourceTitleTextStyle">{{ readerSource.name }}</span>
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
              <IconRefresh />
            </span>
            <div class="feed-refresh-header__copy">
              <div class="feed-refresh-header__title">{{ pullStatusText }}</div>
              <div class="feed-refresh-header__meta">{{ pullStatusMeta }}</div>
            </div>
          </div>
        </div>
      </header>
      <div
        ref="sourceReaderContentRef"
        class="reader-overlay__content"
        @scroll.passive="handleSourceReaderScroll"
      >
        <SubscriptionFeedView
          mode="source"
          :source-kind="readerSource.kind"
          :source-id="readerSource.id"
          :active="true"
          :scroll-top="sourceReaderScrollTop"
          :top-chrome-progress="1"
          :header-height="feedHeaderHeight"
          :freeze-body-during-top-refresh="true"
          :morphing-item-id="morphingItemId"
          :morphing-height-lock-item-id="morphingHeightLockItemId"
          :morphing-item-height="morphingItemHeight"
          :morphing-preview-progress="feedItemPreviewProgress"
          @top-pull-start="noopTopPullStart"
          @top-pull-move="noopTopPullMove"
          @top-pull-end="noopTopPullEnd"
          @open-item="openItemReader"
        />
      </div>
    </section>

    <div
      v-if="sourceTitleRevealVisible && readerSource"
      class="source-title-reveal"
      :style="sourceTitleRevealStyle"
      aria-hidden="true"
    >
      <span>{{ readerSource.name }}</span>
      <small>{{ sourceToggleActive ? '已订阅' : '未订阅' }}</small>
    </div>

    <div
      v-if="sourceNameMorphVisible && detailItem"
      class="detail-source-morph"
      :style="sourceNameMorphStyle"
    >
      {{ detailItem.source_name || '未知来源' }}
    </div>

    <section
      v-if="detailReaderOpen"
      class="reader-overlay reader-overlay--detail"
      :class="{
        'reader-overlay--motion-settling': readerMotionSettling,
        'reader-overlay--returning-feed': detailReturningToFeed,
      }"
      :style="detailReaderStyle"
    >
      <div
        class="reader-transition-surface"
        :class="{
          'reader-transition-surface--entry-settling': detailEntrySettling,
          'reader-transition-surface--chrome-settling': feedChromeSettling,
        }"
        :style="detailTransitionSurfaceStyle"
      >
        <article v-if="detailItem" class="reader-morph-text" :style="detailMorphTextStyle">
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
        <div class="reader-detail-progress__track">
          <div class="reader-detail-progress__fill" :style="detailProgressFillStyle" />
          <div class="reader-detail-progress__thumb" :style="detailProgressThumbStyle" />
        </div>
      </div>
    </section>
  </div>
</template>
