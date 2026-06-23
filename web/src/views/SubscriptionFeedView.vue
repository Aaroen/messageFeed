<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

import { formatAPIError } from '@/api/client'
import { fetchActiveSources, fetchSource, type FeedItem, listRecommendationItems, listTimelineItems } from '@/api/feed'
import { clampProgress } from '@/composables/feedChromeMetrics'
import { useFeedPullRefreshCompletionAction } from '@/composables/useFeedPullRefreshCompletionAction'
import { useGestureDirection } from '@/composables/useGestureDirection'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { usePullRefresh } from '@/composables/usePullRefresh'
import { useRefreshLayoutFreeze } from '@/composables/useRefreshLayoutFreeze'
import { useRequestToken } from '@/composables/useRequestToken'
import { useTimedNotice } from '@/composables/useTimedNotice'
import { useFeedInteractionStore } from '@/stores/feedInteraction'
import { type FeedListCacheEntry, useFeedListCacheStore } from '@/stores/feedListCache'
import { subscriptionFeedFetchNotice } from '@/utils/sourceFetchMessages'

type FeedMode = 'subscriptions' | 'recommendations' | 'source'
type SourceKind = 'subscriptions' | 'recommendations'
type FeedNoticeType = 'success' | 'warning'
type FeedNotice = { type: FeedNoticeType; message: string }

const props = withDefaults(
  defineProps<{
    mode?: FeedMode
    sourceKind?: SourceKind
    sourceId?: number
    active?: boolean
    scrollTop?: number
    topChromeProgress?: number
    topChromeContentCollapsed?: boolean
    headerHeight?: number
    freezeBodyDuringTopRefresh?: boolean
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    morphingPreviewProgress?: number
    backgroundRefresh?: boolean
  }>(),
  {
    mode: 'subscriptions',
    sourceKind: 'subscriptions',
    sourceId: 0,
    active: true,
    scrollTop: 0,
    topChromeProgress: 1,
    topChromeContentCollapsed: false,
    headerHeight: 86,
    freezeBodyDuringTopRefresh: false,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    morphingPreviewProgress: 0,
    backgroundRefresh: false,
  },
)

const emit = defineEmits<{
  'top-pull-start': [startedWithVisibleChrome: boolean]
  'top-pull-move': [distance: number]
  'top-pull-end': [shouldRefresh: boolean]
  'open-item': [item: FeedItem, sourceKind: SourceKind, originRect?: DOMRect]
}>()

const feedInteraction = useFeedInteractionStore()
const feedListCache = useFeedListCacheStore()
const motionTimings = useMotionTimings()
const items = ref<FeedItem[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const backgroundRefreshing = ref(false)
const lastUpdatedAt = ref('')
const totalCount = ref(0)
const nextOffset = ref(0)
const reachedEnd = ref(false)
const feedPageRef = ref<HTMLElement | null>(null)
const feedBodyRef = ref<HTMLElement | null>(null)
const initialSubscriptionFetchAttempted = ref(false)

const pullThreshold = 76
const pullMaxOffset = 116
const emptyPullMaxOffset = 88
const pullRefresh = usePullRefresh({
  threshold: pullThreshold,
  maxOffset: pullMaxOffset,
  emptyMaxOffset: emptyPullMaxOffset,
})
const pullOffset = pullRefresh.offset
const pullDragging = pullRefresh.dragging
const pullSettling = pullRefresh.settling
const refreshing = pullRefresh.refreshing
const pullStartedWithVisibleChrome = pullRefresh.startedWithVisibleChrome
const trackingPullCandidate = pullRefresh.gestureCandidate
const trackingPull = pullRefresh.gestureTracking
const topPullGestureDirection = useGestureDirection({ viewDragThreshold: 8 })
const pageSize = 10
const cacheTTLMS = 60 * 1000
const noticeRevealDelay = motionTimings.noticeRevealDelay
const feedNoticeRuntime = useTimedNotice<FeedNoticeType>({
  duration: motionTimings.noticeDuration,
  canShow: () => !props.backgroundRefresh,
})
const feedNotice = feedNoticeRuntime.notice
let touchStartChromeDistance = 0
let loadMoreSyncTimer = 0
let loadMoreSyncToken = 0
const loadRequestToken = useRequestToken({ isActive: () => !disposed })
const backgroundRefreshRequestToken = useRequestToken()
let topPullStartedNotified = false
let disposed = false
let loadMoreObserver: IntersectionObserver | null = null

const viewKey = computed(() => `${props.mode}:${props.sourceKind}:${props.sourceId}`)
const cacheRevision = computed(() => feedListCache.revisions[viewKey.value] ?? 0)
const hasItems = computed(() => items.value.length > 0)
const canLoadMore = computed(
  () =>
    props.active &&
    !props.backgroundRefresh &&
    hasItems.value &&
    !loading.value &&
    !refreshing.value &&
    !loadingMore.value &&
    !reachedEnd.value,
)
const loadMoreTriggerIndex = computed(() => Math.max(items.value.length - 3, 0))
const isRecommendations = computed(() => props.mode === 'recommendations')
const isSourceMode = computed(() => props.mode === 'source')
const usesGlobalPullState = computed(() => !props.backgroundRefresh)
const shouldRefreshVisibleSource = computed(() => isSourceMode.value && !props.backgroundRefresh)
const effectiveSourceKind = computed<SourceKind>(() => {
  if (isSourceMode.value) {
    return props.sourceKind
  }
  return props.mode === 'recommendations' ? 'recommendations' : 'subscriptions'
})
const copy = computed(() => {
  if (isSourceMode.value) {
    return {
      mark: '源',
      loadingTitle: '抓取中',
      emptyTitle: '暂无来源内容',
      emptyDescription: '该来源暂时没有返回可展示内容。',
    }
  }
  if (isRecommendations.value || effectiveSourceKind.value === 'recommendations') {
    return {
      mark: '荐',
      loadingTitle: '正在加载推荐内容',
      emptyTitle: '暂无推荐内容',
      emptyDescription: '官方源暂时没有返回可展示内容。',
    }
  }
  return {
    mark: '订',
    loadingTitle: '正在加载订阅内容',
    emptyTitle: '暂无订阅内容',
    emptyDescription: '请先在订阅管理中启用来源并执行抓取。',
  }
})
const pullProgress = pullRefresh.progress
const pullActive = pullRefresh.active
const safeTopChromeProgress = computed(() => clampProgress(props.topChromeProgress))
const pullStatusText = computed(() => {
  if (refreshing.value) {
    return isSourceMode.value ? '抓取中' : '正在刷新'
  }
  return pullProgress.value >= 1 ? '释放刷新' : '下拉刷新'
})
const pullStatusMeta = computed(() => (lastUpdatedAt.value ? `最近更新 ${lastUpdatedAt.value}` : '尚未更新'))
const errorMessage = computed(() => error.value.trim())
const pageClass = computed(() => ({
  'feed-list-page--pulling': pullActive.value,
  'feed-list-page--empty': !hasItems.value && !loading.value,
}))
const freezeBodyForTopChromePull = computed(
  () => props.freezeBodyDuringTopRefresh || pullStartedWithVisibleChrome.value,
)
const feedBodyPullFeedbackActive = computed(
  () => pullDragging.value && !refreshing.value && !freezeBodyForTopChromePull.value,
)
const feedBodyShift = computed(() => {
  if (!feedBodyPullFeedbackActive.value) {
    return 0
  }

  const shiftRatio = hasItems.value ? 0.34 : 0.2
  const maxShift = hasItems.value ? 38 : 18
  return Math.min(pullOffset.value * shiftRatio, maxShift)
})
const refreshLayoutFreeze = useRefreshLayoutFreeze({ targetRef: feedBodyRef })
const feedBodyStyle = computed(() => ({
  transform: `translate3d(0, ${cssPx(feedBodyShift.value)}, 0)`,
  transition: feedBodyPullFeedbackActive.value ? 'none' : 'transform var(--motion-fast) var(--ease-standard)',
  ...refreshLayoutFreeze.style.value,
}))
const safeMorphingPreviewProgress = computed(() => clampProgress(props.morphingPreviewProgress))

const feedPullRefreshCompletion = useFeedPullRefreshCompletionAction({
  usesGlobalPullState,
  active: computed(() => props.active),
  viewKey,
  lastUpdatedAt,
  pullOffset,
  pullActive,
  pullSettling,
  refreshing,
  pullStartedWithVisibleChrome,
  trackingPull,
  pullStatusText,
  pullStatusMeta,
  isSourceMode,
  feedInteraction,
  pullRefresh,
})
const clearPullState = feedPullRefreshCompletion.clearPullState

function formatDate(value?: string) {
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

function itemSummary(item: FeedItem) {
  return plainPreviewText(item.summary, item.content_snippet, item.content_text, item.content_html) || '暂无摘要。'
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function itemTime(item: FeedItem) {
  const value = item.published_at || item.fetched_at
  const timestamp = value ? new Date(value).getTime() : 0
  return Number.isFinite(timestamp) ? timestamp : 0
}

function compareItemsDescending(left: FeedItem, right: FeedItem) {
  const timeDelta = itemTime(right) - itemTime(left)
  return timeDelta === 0 ? left.id - right.id : timeDelta
}

function sortItemsDescending(nextItems: FeedItem[]) {
  return [...nextItems].sort(compareItemsDescending)
}

function mergeItems(currentItems: FeedItem[], nextItems: FeedItem[]) {
  const itemMap = new Map<number, FeedItem>()
  for (const item of currentItems) {
    itemMap.set(item.id, item)
  }
  for (const item of nextItems) {
    itemMap.set(item.id, item)
  }
  return sortItemsDescending(Array.from(itemMap.values()))
}

function cacheEntryIsFresh(entry: FeedListCacheEntry) {
  return Date.now() - entry.cachedAt <= cacheTTLMS
}

function restoreItemsFromCache() {
  const entry = feedListCache.get(viewKey.value)
  if (!entry) {
    return null
  }

  items.value = sortItemsDescending(entry.items)
  totalCount.value = entry.total
  nextOffset.value = entry.nextOffset
  reachedEnd.value = entry.reachedEnd
  lastUpdatedAt.value = entry.lastUpdatedAt
  error.value = ''
  scheduleLoadMoreObserver()
  return entry
}

function writeItemsToCache(cacheKey = viewKey.value) {
  feedListCache.set(cacheKey, {
    items: items.value,
    total: totalCount.value,
    nextOffset: nextOffset.value,
    reachedEnd: reachedEnd.value,
    lastUpdatedAt: lastUpdatedAt.value,
    cachedAt: Date.now(),
  })
}

function shouldRefreshCache(entry: FeedListCacheEntry | null) {
  return !entry || !cacheEntryIsFresh(entry)
}

function resetListState() {
  items.value = []
  totalCount.value = 0
  nextOffset.value = 0
  reachedEnd.value = false
  lastUpdatedAt.value = ''
  initialSubscriptionFetchAttempted.value = false
}

function loadInitialItems() {
  const restored = restoreItemsFromCache()
  if (!props.active) {
    return
  }
  if (!restored) {
    void loadItems({ refresh: isSourceMode.value })
    return
  }
  if (shouldRefreshVisibleSource.value) {
    void loadItems({ refresh: true })
    return
  }
  if (shouldRefreshCache(restored)) {
    void loadItems({ refresh: true, background: true })
  }
}

function nextLoadRequestToken() {
  return loadRequestToken.next()
}

function invalidateLoadRequests() {
  loadRequestToken.invalidate()
  clearBackgroundRefresh()
}

function loadRequestIsCurrent(token: number, requestViewKey: string) {
  return loadRequestToken.isCurrent(token) && requestViewKey === viewKey.value
}

function refreshStaleCacheInBackground() {
  if (!props.active || loading.value || refreshing.value || loadingMore.value || backgroundRefreshing.value) {
    return
  }
  const entry = feedListCache.get(viewKey.value)
  if (!entry || cacheEntryIsFresh(entry)) {
    return
  }
  void loadItems({ refresh: true, background: true })
}

function beginBackgroundRefresh(token: number) {
  backgroundRefreshRequestToken.set(token)
  backgroundRefreshing.value = true
}

function clearBackgroundRefresh(token?: number) {
  if (!backgroundRefreshRequestToken.isCurrent(token)) {
    return
  }
  backgroundRefreshRequestToken.set(0)
  backgroundRefreshing.value = false
}

function handleVisibilityRefresh() {
  if (document.visibilityState === 'visible') {
    refreshStaleCacheInBackground()
  }
}

function refreshVisibleSourceIfEmpty() {
  if (
    !shouldRefreshVisibleSource.value ||
    backgroundRefreshing.value ||
    hasItems.value ||
    loading.value ||
    refreshing.value ||
    loadingMore.value
  ) {
    return
  }

  void loadItems({ refresh: true })
}

function feedItemStyle(item: FeedItem) {
  const style: Record<string, string> = {}
  const locksHeight =
    (props.morphingItemId === item.id || props.morphingHeightLockItemId === item.id) && props.morphingItemHeight

  if (locksHeight && props.morphingItemHeight) {
    style.height = cssPx(props.morphingItemHeight)
  }

  return Object.keys(style).length > 0 ? style : undefined
}

function feedItemPreviewStyle(item: FeedItem) {
  if (props.morphingItemId !== item.id) {
    return undefined
  }

  const progress = safeMorphingPreviewProgress.value
  return {
    filter: `blur(${((1 - progress) * 2.4).toFixed(2)}px)`,
    opacity: progress.toFixed(3),
    pointerEvents: 'none' as const,
    transform: `translate3d(0, ${cssPx((1 - progress) * 6)}, 0)`,
  }
}

function clearFeedNotice() {
  feedNoticeRuntime.clear()
}

function showFeedNotice(type: FeedNotice['type'], message: string, delayMS = 0) {
  feedNoticeRuntime.show(type, message, undefined, delayMS)
}

function refreshSuccessMessage() {
  if (effectiveSourceKind.value === 'recommendations') {
    return '刷新成功：已更新当前推荐来源'
  }
  if (isSourceMode.value) {
    return '刷新成功：已更新当前来源'
  }
  return '刷新成功：已更新订阅内容'
}

async function refreshSubscriptionSources() {
  if (effectiveSourceKind.value !== 'subscriptions') {
    return null
  }
  try {
    if (isSourceMode.value && props.sourceId > 0) {
      await fetchSource(props.sourceId)
      return { type: 'success' as const, message: refreshSuccessMessage() }
    }

    const result = await fetchActiveSources()
    return subscriptionFeedFetchNotice(result, refreshSuccessMessage())
  } catch (err) {
    return { type: 'warning' as const, message: `刷新失败：${formatAPIError(err)}` }
  }
}

function completeBlockedForegroundRefresh() {
  if (!pullActive.value || refreshing.value) {
    return
  }

  feedPullRefreshCompletion.completeLoad({
    isRefresh: true,
    isBackgroundRefresh: false,
  })
}

async function loadItems(options: { refresh?: boolean; append?: boolean; background?: boolean } = {}) {
  const isRefresh = Boolean(options.refresh)
  const isAppend = Boolean(options.append)
  const isBackground = Boolean(options.background)
  const isBackgroundRefresh = isBackground || props.backgroundRefresh
  if (
    loading.value ||
    refreshing.value ||
    loadingMore.value ||
    (isBackgroundRefresh && backgroundRefreshing.value) ||
    (isAppend && !canLoadMore.value)
  ) {
    if (isRefresh && !isBackgroundRefresh) {
      completeBlockedForegroundRefresh()
    }
    return
  }
  const requestViewKey = viewKey.value
  const requestToken = nextLoadRequestToken()
  if (isBackgroundRefresh) {
    beginBackgroundRefresh(requestToken)
  } else {
    clearBackgroundRefresh()
  }
  const releaseRefreshLayoutFreeze =
    isRefresh && !isBackgroundRefresh ? refreshLayoutFreeze.capture() : undefined
  error.value = ''
  if (isAppend) {
    loadingMore.value = true
  } else if (isRefresh && !isBackgroundRefresh) {
    if (shouldRefreshVisibleSource.value && pullOffset.value <= 0) {
      pullRefresh.commitRefreshOffset()
    }
    clearFeedNotice()
    pullRefresh.beginRefreshing()
  } else if (!isBackgroundRefresh) {
    loading.value = true
  }
  try {
    const refreshNotice = isRefresh ? await refreshSubscriptionSources() : null
    if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
      return
    }
    const loader = effectiveSourceKind.value === 'recommendations' ? listRecommendationItems : listTimelineItems
    const requestOffset = isAppend ? nextOffset.value : 0
    const params = {
      limit: pageSize,
      offset: requestOffset,
      order: 'desc' as const,
      ...(isRefresh && effectiveSourceKind.value === 'recommendations' ? { refresh: true } : {}),
      ...(isSourceMode.value && props.sourceId > 0 ? { source_id: props.sourceId } : {}),
    }
    const result = await loader(params)
    if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
      return
    }
    let nextItems = result.items
    let nextTotal = result.total
    let autoFetchNotice: FeedNotice | null = null
    if (
      !isRefresh &&
      !isAppend &&
      !isSourceMode.value &&
      effectiveSourceKind.value === 'subscriptions' &&
      result.total === 0 &&
      !initialSubscriptionFetchAttempted.value
    ) {
      initialSubscriptionFetchAttempted.value = true
      autoFetchNotice = await refreshSubscriptionSources()
      if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
        return
      }
      const reloaded = await loader(params)
      if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
        return
      }
      nextItems = reloaded.items
      nextTotal = reloaded.total
    }

    items.value = isAppend ? mergeItems(items.value, nextItems) : sortItemsDescending(nextItems)
    nextOffset.value = requestOffset + nextItems.length
    totalCount.value = Math.max(nextTotal, nextOffset.value)
    reachedEnd.value =
      nextItems.length < pageSize ||
      (effectiveSourceKind.value !== 'recommendations' && nextOffset.value >= totalCount.value)
    lastUpdatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    writeItemsToCache(requestViewKey)
    if (!isBackgroundRefresh && refreshNotice) {
      showFeedNotice(refreshNotice.type, refreshNotice.message, noticeRevealDelay)
    } else if (!isBackgroundRefresh && autoFetchNotice && autoFetchNotice.type === 'warning') {
      showFeedNotice(autoFetchNotice.type, autoFetchNotice.message)
    } else if (!isBackgroundRefresh && isRefresh) {
      showFeedNotice('success', refreshSuccessMessage(), noticeRevealDelay)
    }
  } catch (err) {
    if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
      return
    }
    const message = formatAPIError(err)
    if (isRefresh) {
      if (isBackgroundRefresh) {
        if (!hasItems.value) {
          error.value = `刷新失败：${message}`
        }
      } else {
        showFeedNotice('warning', `刷新失败：${message}`, noticeRevealDelay)
      }
    } else if (isAppend) {
      showFeedNotice('warning', `加载更多失败：${message}`)
    } else {
      error.value = `加载失败：${message}`
    }
  } finally {
    if (isBackgroundRefresh) {
      clearBackgroundRefresh(requestToken)
    }
    if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
      releaseRefreshLayoutFreeze?.()
      return
    }
    loading.value = false
    loadingMore.value = false
    scheduleLoadMoreObserver()

    feedPullRefreshCompletion.completeLoad({
      isRefresh,
      isBackgroundRefresh,
      afterSettled: releaseRefreshLayoutFreeze,
    })
  }
}

function loadMoreRoot() {
  const root = feedPageRef.value?.closest('.app-content, .reader-overlay__content')
  return root instanceof HTMLElement ? root : null
}

function loadMoreTriggerElement() {
  return feedPageRef.value?.querySelector<HTMLElement>('[data-load-more-trigger="true"]') ?? null
}

function stopLoadMoreObserver() {
  loadMoreObserver?.disconnect()
  loadMoreObserver = null
}

function clearLoadMoreSyncTimer() {
  window.clearTimeout(loadMoreSyncTimer)
  loadMoreSyncTimer = 0
  loadMoreSyncToken += 1
}

function maybeLoadMoreFromTrigger() {
  if (!canLoadMore.value) {
    return
  }
  const trigger = loadMoreTriggerElement()
  if (!trigger) {
    return
  }

  const root = loadMoreRoot()
  const triggerRect = trigger.getBoundingClientRect()
  const rootRect = root?.getBoundingClientRect()
  const viewportBottom = rootRect?.bottom ?? window.innerHeight
  if (triggerRect.top <= viewportBottom) {
    void loadItems({ append: true })
  }
}

function syncLoadMoreObserver() {
  if (disposed) {
    return
  }

  stopLoadMoreObserver()
  if (!canLoadMore.value || typeof IntersectionObserver === 'undefined') {
    return
  }

  const trigger = loadMoreTriggerElement()
  if (!trigger) {
    return
  }

  loadMoreObserver = new IntersectionObserver(
    (entries) => {
      if (!disposed && canLoadMore.value && entries.some((entry) => entry.isIntersecting)) {
        void loadItems({ append: true })
      }
    },
    {
      root: loadMoreRoot(),
      threshold: 0.01,
    },
  )
  loadMoreObserver.observe(trigger)
  maybeLoadMoreFromTrigger()
}

function scheduleLoadMoreObserver() {
  clearLoadMoreSyncTimer()
  const syncToken = loadMoreSyncToken + 1
  loadMoreSyncToken = syncToken
  loadMoreSyncTimer = window.setTimeout(() => {
    loadMoreSyncTimer = 0
    if (disposed || syncToken !== loadMoreSyncToken) {
      return
    }
    void nextTick(() => {
      if (disposed || syncToken !== loadMoreSyncToken) {
        return
      }
      syncLoadMoreObserver()
    })
  }, 0)
}

function openItem(item: FeedItem, event: MouseEvent) {
  const target = event.currentTarget
  const originRect = target instanceof HTMLElement ? target.closest('.feed-item')?.getBoundingClientRect() : undefined
  emit('open-item', item, effectiveSourceKind.value, originRect)
}

function resetPullTracking() {
  pullRefresh.finishGestureTracking()
}

function resetPullGesture(force = false) {
  pullRefresh.cancelGesture()
  clearPullState(force)
  topPullStartedNotified = false
}

function cancelPullGesture(force = false) {
  const shouldNotifyTopPullEnd = topPullStartedNotified
  resetPullGesture(force)
  if (shouldNotifyTopPullEnd) {
    emit('top-pull-end', false)
  }
}

function notifyTopPullStart() {
  if (topPullStartedNotified) {
    return
  }
  topPullStartedNotified = true
  emit('top-pull-start', pullStartedWithVisibleChrome.value)
}

function resetTransientLoadState(
  options: { clearList?: boolean; clearNotice?: boolean; pullOwnerViewKey?: string } = {},
) {
  invalidateLoadRequests()
  loading.value = false
  loadingMore.value = false
  error.value = ''
  pullRefresh.reset()
  refreshLayoutFreeze.release()
  clearLoadMoreSyncTimer()
  stopLoadMoreObserver()
  clearPullState(true, options.pullOwnerViewKey)
  if (options.clearList) {
    resetListState()
  }
  if (options.clearNotice) {
    clearFeedNotice()
  }
}

function handleTouchStart(event: TouchEvent) {
  if (
    !props.active ||
    props.scrollTop > 0 ||
    event.touches.length !== 1 ||
    loading.value ||
    refreshing.value ||
    loadingMore.value
  ) {
    cancelPullGesture()
    return
  }

  const touch = event.touches[0]
  const startedWithLayoutChrome =
    props.freezeBodyDuringTopRefresh || (!props.topChromeContentCollapsed && safeTopChromeProgress.value > 0.04)
  topPullStartedNotified = false
  touchStartChromeDistance = startedWithLayoutChrome ? props.headerHeight * safeTopChromeProgress.value : 0
  pullRefresh.begin(startedWithLayoutChrome)
  pullRefresh.beginGestureCandidate(touch.clientX, touch.clientY)
}

function handleTouchMove(event: TouchEvent) {
  if (!props.active || event.touches.length !== 1) {
    if (trackingPullCandidate.value || trackingPull.value) {
      cancelPullGesture()
    }
    return
  }

  if (!trackingPullCandidate.value && !trackingPull.value) {
    return
  }

  if (props.scrollTop > 0) {
    cancelPullGesture()
    return
  }

  const touch = event.touches[0]
  const { deltaX, deltaY } = pullRefresh.gestureDelta(touch.clientX, touch.clientY)

  if (!trackingPull.value) {
    if (topPullGestureDirection.shouldCancelTopPull(deltaX, deltaY)) {
      cancelPullGesture()
      return
    }

    if (topPullGestureDirection.shouldWaitForTopPull(deltaX, deltaY)) {
      return
    }

    pullRefresh.beginGestureTracking()
    notifyTopPullStart()
    clearFeedNotice()
  }

  if (trackingPull.value) {
    event.preventDefault()
    emit('top-pull-move', Math.max(0, deltaY))
    const refreshDistance = Math.max(0, deltaY - touchStartChromeDistance)
    if (refreshDistance <= 0) {
      pullRefresh.setGestureOffset(0)
      clearPullState()
      return
    }
    pullRefresh.setGestureOffset(pullRefresh.rubberBandDistance(refreshDistance, hasItems.value))
  }
}

function handleTouchEnd() {
  if (!trackingPull.value) {
    const shouldNotifyTopPullEnd = topPullStartedNotified
    resetPullGesture()
    if (shouldNotifyTopPullEnd) {
      emit('top-pull-end', false)
    }
    return
  }

  const shouldRefresh = pullOffset.value >= pullThreshold
  resetPullTracking()

  if (shouldRefresh) {
    pullRefresh.commitRefreshOffset()
    emit('top-pull-end', true)
    topPullStartedNotified = false
    void loadItems({ refresh: true })
    return
  }

  resetPullGesture()
  emit('top-pull-end', false)
}

function handleTouchCancel() {
  const shouldNotifyTopPullEnd = topPullStartedNotified
  resetPullGesture()
  if (shouldNotifyTopPullEnd) {
    emit('top-pull-end', false)
  }
}

onMounted(() => {
  loadInitialItems()
  window.addEventListener('focus', refreshStaleCacheInBackground)
  document.addEventListener('visibilitychange', handleVisibilityRefresh)
})

onUnmounted(() => {
  disposed = true
  invalidateLoadRequests()
  pullRefresh.reset()
  refreshLayoutFreeze.release()
  clearLoadMoreSyncTimer()
  feedNoticeRuntime.dispose()
  stopLoadMoreObserver()
  window.removeEventListener('focus', refreshStaleCacheInBackground)
  document.removeEventListener('visibilitychange', handleVisibilityRefresh)
  clearPullState(true)
})

watch(
  () => props.active,
  (active) => {
    if (!active) {
      if (!isSourceMode.value && effectiveSourceKind.value === 'subscriptions') {
        initialSubscriptionFetchAttempted.value = false
      }
      resetTransientLoadState({ clearNotice: true })
      return
    }
    const restored = hasItems.value ? feedListCache.get(viewKey.value) : restoreItemsFromCache()
    if (!hasItems.value && !loading.value) {
      void loadItems({ refresh: isSourceMode.value })
      return
    }
    if (shouldRefreshVisibleSource.value) {
      void loadItems({ refresh: true })
      return
    }
    if (shouldRefreshCache(restored)) {
      void loadItems({ refresh: true, background: true })
    }
  },
)

watch(
  viewKey,
  (_nextViewKey, previousViewKey) => {
    resetTransientLoadState({
      clearList: true,
      clearNotice: true,
      pullOwnerViewKey: previousViewKey,
    })
    if (props.active) {
      loadInitialItems()
    }
  },
)

watch(
  cacheRevision,
  () => {
    if (!props.active || loading.value || refreshing.value || loadingMore.value || backgroundRefreshing.value) {
      return
    }
    if (!hasItems.value) {
      void loadItems({ refresh: isSourceMode.value })
      return
    }
    void loadItems({ refresh: true, background: true })
  },
)

watch(
  () => props.scrollTop,
  () => {
    maybeLoadMoreFromTrigger()
  },
)

watch(
  [
    () => items.value.length,
    () => props.active,
    () => props.backgroundRefresh,
    loading,
    refreshing,
    loadingMore,
    reachedEnd,
  ],
  () => {
    scheduleLoadMoreObserver()
  },
  { flush: 'post' },
)

watch(
  [() => props.backgroundRefresh, backgroundRefreshing],
  () => {
    refreshVisibleSourceIfEmpty()
  },
)

watch(
  () => props.backgroundRefresh,
  (backgroundRefresh) => {
    if (backgroundRefresh) {
      clearFeedNotice()
      clearPullState(true)
    }
  },
)

watch(
  [pullOffset, refreshing, lastUpdatedAt, () => props.active],
  () => {
    feedPullRefreshCompletion.syncPullState()
  },
  { immediate: true },
)
</script>

<template>
  <section
    ref="feedPageRef"
    class="feed-list-page"
    :class="pageClass"
    :data-feed-mode="mode"
    @touchstart.passive="handleTouchStart"
    @touchmove="handleTouchMove"
    @touchend.passive="handleTouchEnd"
    @touchcancel.passive="handleTouchCancel"
  >
    <Teleport to="body">
      <div
        v-if="feedNotice"
        class="sources-toast feed-toast"
        :class="`sources-toast--${feedNotice.type}`"
        role="status"
        aria-live="polite"
      >
        {{ feedNotice.message }}
      </div>
    </Teleport>

    <div ref="feedBodyRef" class="feed-list-body" :style="feedBodyStyle">
      <a-alert v-if="errorMessage" class="feed-alert" type="warning" :content="errorMessage" show-icon />

      <section v-if="loading && !hasItems" class="empty-surface">
        <div class="empty-surface__mark">{{ copy.mark }}</div>
        <h2>{{ copy.loadingTitle }}</h2>
        <p>请稍候。</p>
      </section>

      <section v-else-if="!hasItems" class="empty-surface">
        <div class="empty-surface__mark">{{ copy.mark }}</div>
        <h2>{{ copy.emptyTitle }}</h2>
        <p>{{ copy.emptyDescription }}</p>
      </section>

      <div v-else class="feed-item-list">
        <article
          v-for="(item, index) in items"
          :key="item.id"
          class="feed-item"
          :class="{ 'feed-item--morphing': morphingItemId === item.id }"
          :data-feed-item-id="item.id"
          :data-load-more-trigger="index === loadMoreTriggerIndex ? 'true' : undefined"
          :style="feedItemStyle(item)"
        >
          <div class="feed-item__meta" :style="feedItemPreviewStyle(item)">
            <span class="feed-item__source">
              {{ item.source_name || '未知来源' }}
            </span>
            <span>{{ formatDate(item.published_at || item.fetched_at) }}</span>
          </div>
          <button
            class="feed-item__read-target"
            type="button"
            :style="feedItemPreviewStyle(item)"
            @click="openItem(item, $event)"
          >
            <h2>{{ item.title }}</h2>
            <p>{{ itemSummary(item) }}</p>
          </button>
          <div class="feed-item__actions" :style="feedItemPreviewStyle(item)">
            <a :href="item.url" target="_blank" rel="noreferrer">阅读原文</a>
          </div>
        </article>
        <div v-if="loadingMore" class="feed-load-more" role="status" aria-label="加载更多">
          <span class="feed-loading-dots" aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
        </div>
        <div
          v-else-if="reachedEnd && hasItems && totalCount > pageSize"
          class="feed-load-more feed-load-more--end"
          role="status"
          aria-live="polite"
        >
          已加载全部
        </div>
      </div>
    </div>
  </section>
</template>
