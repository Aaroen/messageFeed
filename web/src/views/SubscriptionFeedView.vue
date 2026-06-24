<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

import { formatAPIError } from '@/api/client'
import {
  fetchSource,
  type FeedItem,
  type Source,
  listSources,
  listRecommendationItems,
  listTimelineItems,
  setItemFavorite,
  setItemHidden,
  setItemRead,
} from '@/api/feed'
import ActionBar from '@/components/ActionBar.vue'
import { clampProgress, feedVisibleContentTopOffset } from '@/composables/feedChromeMetrics'
import { useFeedPullRefreshCompletionAction } from '@/composables/useFeedPullRefreshCompletionAction'
import { useGestureDirection } from '@/composables/useGestureDirection'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { usePullRefresh } from '@/composables/usePullRefresh'
import { useRefreshLayoutFreeze } from '@/composables/useRefreshLayoutFreeze'
import { useRefreshNoticeSequence } from '@/composables/useRefreshNoticeSequence'
import { useRequestToken } from '@/composables/useRequestToken'
import { useTimedNotice } from '@/composables/useTimedNotice'
import { useFeedInteractionStore } from '@/stores/feedInteraction'
import { type FeedListCacheEntry, useFeedListCacheStore } from '@/stores/feedListCache'

type FeedMode = 'subscriptions' | 'recommendations' | 'source'
type SourceKind = 'subscriptions' | 'recommendations'
type FeedNoticeType = 'success' | 'warning'
type FeedNotice = { type: FeedNoticeType; message: string }
type TriStateFilter = 'all' | 'true' | 'false'
type HiddenFilter = 'visible' | 'all' | 'hidden'
type ItemStateKey = 'read' | 'favorite' | 'hidden'

const props = withDefaults(
  defineProps<{
    mode?: FeedMode
    sourceKind?: SourceKind
    sourceId?: number
    sourceTimelineId?: number
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
    sourceTimelineId: 0,
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
const sources = ref<Source[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const filtersLoading = ref(false)
const error = ref('')
const filterError = ref('')
const backgroundRefreshing = ref(false)
const lastUpdatedAt = ref('')
const totalCount = ref(0)
const nextOffset = ref(0)
const reachedEnd = ref(false)
const feedPageRef = ref<HTMLElement | null>(null)
const feedBodyRef = ref<HTMLElement | null>(null)
const selectedSourceID = ref(0)
const readFilter = ref<TriStateFilter>('all')
const favoriteFilter = ref<TriStateFilter>('all')
const hiddenFilter = ref<HiddenFilter>('visible')
const itemActionBusy = ref<Record<number, ItemStateKey | ''>>({})

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
const pageSize = 50
const loadMorePreloadMargin = 240
const cacheTTLMS = 60 * 1000
const feedNoticeRuntime = useTimedNotice<FeedNoticeType>({
  duration: motionTimings.noticeDuration,
  canShow: () => !props.backgroundRefresh,
})
const feedNotice = feedNoticeRuntime.notice
const refreshNoticeSequence = useRefreshNoticeSequence<FeedNoticeType>({ show: showFeedNotice })
let touchStartChromeDistance = 0
let loadMoreSyncTimer = 0
let loadMoreSyncToken = 0
const loadRequestToken = useRequestToken({ isActive: () => !disposed })
const backgroundRefreshRequestToken = useRequestToken()
let topPullStartedNotified = false
let disposed = false
let loadMoreObserver: IntersectionObserver | null = null

const filterKey = computed(() => {
  if (props.mode !== 'subscriptions') {
    return 'filter:none'
  }
  return [
    `source=${selectedSourceID.value}`,
    `read=${readFilter.value}`,
    `favorite=${favoriteFilter.value}`,
    `hidden=${hiddenFilter.value}`,
  ].join(';')
})
const viewKey = computed(
  () => `${props.mode}:${props.sourceKind}:${props.sourceId}:${props.sourceTimelineId}:${filterKey.value}`,
)
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
const isRecommendations = computed(() => props.mode === 'recommendations')
const isSourceMode = computed(() => props.mode === 'source')
const usesGlobalPullState = computed(() => !props.backgroundRefresh)
const shouldRefreshVisibleSource = computed(() => isSourceMode.value && !props.backgroundRefresh)
const effectiveSourceKind = computed<SourceKind>(() => {
  if (isSourceMode.value) {
    if (props.sourceKind === 'recommendations' && props.sourceTimelineId > 0) {
      return 'subscriptions'
    }
    return props.sourceKind
  }
  return props.mode === 'recommendations' ? 'recommendations' : 'subscriptions'
})
const effectiveSourceId = computed(() => {
  if (isSourceMode.value && props.sourceKind === 'recommendations' && props.sourceTimelineId > 0) {
    return props.sourceTimelineId
  }
  return props.sourceId
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
    return '正在刷新'
  }
  return pullProgress.value >= 1 ? '释放刷新' : '下拉刷新'
})
const pullStatusMeta = computed(() => (lastUpdatedAt.value ? `最近更新 ${lastUpdatedAt.value}` : '尚未更新'))
const errorMessage = computed(() => error.value.trim())
const filterErrorMessage = computed(() => filterError.value.trim())
const filtersVisible = computed(() => props.mode === 'subscriptions' && !isSourceMode.value)
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

function triStateBool(value: TriStateFilter) {
  if (value === 'true') {
    return true
  }
  if (value === 'false') {
    return false
  }
  return undefined
}

function hiddenFilterParams(value: HiddenFilter) {
  if (value === 'hidden') {
    return { is_hidden: true, include_hidden: true }
  }
  if (value === 'all') {
    return { include_hidden: true }
  }
  return { is_hidden: false }
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

function currentFiltersMatch(item: FeedItem) {
  if (!filtersVisible.value) {
    return true
  }
  if (selectedSourceID.value > 0 && item.source_id !== selectedSourceID.value) {
    return false
  }
  const isRead = triStateBool(readFilter.value)
  if (typeof isRead === 'boolean' && item.is_read !== isRead) {
    return false
  }
  const isFavorite = triStateBool(favoriteFilter.value)
  if (typeof isFavorite === 'boolean' && item.is_favorite !== isFavorite) {
    return false
  }
  if (hiddenFilter.value === 'visible' && item.is_hidden) {
    return false
  }
  if (hiddenFilter.value === 'hidden' && !item.is_hidden) {
    return false
  }
  return true
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
}

async function loadFilterSources() {
  if (!filtersVisible.value || sources.value.length || filtersLoading.value) {
    return
  }
  filtersLoading.value = true
  filterError.value = ''
  try {
    sources.value = (await listSources()).filter((source) => source.status === 'active')
  } catch (err) {
    filterError.value = `筛选项加载失败：${formatAPIError(err)}`
  } finally {
    filtersLoading.value = false
  }
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

function currentItemIDSet() {
  return new Set(items.value.map((item) => item.id))
}

function localRefreshNotice(previousItemIDs: Set<number>, nextItems: FeedItem[]): FeedNotice {
  const addedItems = nextItems.filter((item) => !previousItemIDs.has(item.id))
  if (!addedItems.length) {
    return { type: 'success', message: '暂无更新内容' }
  }

  if (isSourceMode.value || effectiveSourceKind.value === 'recommendations') {
    return { type: 'success', message: `已更新 ${addedItems.length} 条内容` }
  }

  const sourceIDs = new Set(addedItems.map((item) => item.source_id).filter((sourceID) => sourceID > 0))
  const sourceCount = Math.max(sourceIDs.size, 1)
  return {
    type: 'success',
    message: `已更新 ${sourceCount} 个订阅源的 ${addedItems.length} 条内容`,
  }
}

function shouldFetchEmptySubscribedSource(isAppend: boolean, requestOffset: number, nextItems: FeedItem[]) {
  return (
    !isAppend &&
    requestOffset === 0 &&
    nextItems.length === 0 &&
    isSourceMode.value &&
    effectiveSourceKind.value === 'subscriptions' &&
    effectiveSourceId.value > 0
  )
}

function shouldMarkReachedEnd(nextItems: FeedItem[], nextOffsetValue: number, responseTotal: number) {
  if (nextItems.length < pageSize) {
    return true
  }
  if (!Number.isFinite(responseTotal) || responseTotal <= 0) {
    return false
  }
  return nextOffsetValue >= responseTotal
}

function scrollRefreshContainerToTop() {
  const root = feedPageRef.value?.closest('.app-content, .reader-overlay__content')
  if (root instanceof HTMLElement) {
    root.scrollTo({ top: feedVisibleContentTopOffset(props.headerHeight), behavior: 'auto' })
  }
}

function releaseRefreshLayoutAfterSettled(release?: () => void) {
  release?.()
  scrollRefreshContainerToTop()
  window.requestAnimationFrame(() => {
    scrollRefreshContainerToTop()
    window.requestAnimationFrame(scrollRefreshContainerToTop)
  })
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

async function loadItems(
  options: { refresh?: boolean; append?: boolean; background?: boolean } = {},
) {
  const isRefresh = Boolean(options.refresh)
  const isAppend = Boolean(options.append)
  const isBackground = Boolean(options.background)
  const isBackgroundRefresh = isBackground || props.backgroundRefresh
  const sequencesRefreshNotice = isRefresh && !isBackgroundRefresh
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
  if (sequencesRefreshNotice) {
    refreshNoticeSequence.begin()
  }
  const previousItemIDs = isRefresh && !isAppend ? currentItemIDSet() : new Set<number>()
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
    const loader = effectiveSourceKind.value === 'recommendations' ? listRecommendationItems : listTimelineItems
    const requestOffset = isAppend ? nextOffset.value : 0
    const params = {
      limit: pageSize,
      offset: requestOffset,
      order: 'desc' as const,
      ...(filtersVisible.value && selectedSourceID.value > 0 ? { source_id: selectedSourceID.value } : {}),
      ...(filtersVisible.value && typeof triStateBool(readFilter.value) === 'boolean'
        ? { is_read: triStateBool(readFilter.value) }
        : {}),
      ...(filtersVisible.value && typeof triStateBool(favoriteFilter.value) === 'boolean'
        ? { is_favorite: triStateBool(favoriteFilter.value) }
        : {}),
      ...(filtersVisible.value ? hiddenFilterParams(hiddenFilter.value) : {}),
      ...(isRefresh && isSourceMode.value && effectiveSourceKind.value === 'recommendations' ? { refresh: true } : {}),
      ...(isSourceMode.value && effectiveSourceId.value > 0 ? { source_id: effectiveSourceId.value } : {}),
    }
    let result = await loader(params)
    if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
      return
    }
    if (shouldFetchEmptySubscribedSource(isAppend, requestOffset, result.items)) {
      await fetchSource(effectiveSourceId.value)
      if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
        return
      }
      result = await loader(params)
      if (!loadRequestIsCurrent(requestToken, requestViewKey)) {
        return
      }
    }
    const nextItems = result.items
    const nextTotal = result.total

    items.value = isAppend ? mergeItems(items.value, nextItems) : sortItemsDescending(nextItems)
    nextOffset.value = requestOffset + nextItems.length
    totalCount.value = Math.max(nextTotal, nextOffset.value)
    reachedEnd.value = shouldMarkReachedEnd(nextItems, nextOffset.value, nextTotal)
    lastUpdatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    writeItemsToCache(requestViewKey)
    if (sequencesRefreshNotice) {
      refreshNoticeSequence.showAfterRefreshReleased(localRefreshNotice(previousItemIDs, items.value))
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
        refreshNoticeSequence.showAfterRefreshReleased({ type: 'warning', message: `刷新失败：${message}` })
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
      if (sequencesRefreshNotice) {
        refreshNoticeSequence.cancel()
      }
      releaseRefreshLayoutFreeze?.()
      return
    }
    loading.value = false
    loadingMore.value = false
    scheduleLoadMoreObserver()

    feedPullRefreshCompletion.completeLoad({
      isRefresh,
      isBackgroundRefresh,
      afterRelease: sequencesRefreshNotice
        ? () => refreshNoticeSequence.completeRefreshRelease()
        : undefined,
      afterSettled: sequencesRefreshNotice
        ? () => releaseRefreshLayoutAfterSettled(releaseRefreshLayoutFreeze)
        : releaseRefreshLayoutFreeze,
    })
  }
}

function updateCachedItemState(itemID: number, patch: Partial<Pick<FeedItem, 'is_read' | 'is_favorite' | 'is_hidden'>>) {
  const previousLength = items.value.length
  const nextItems = items.value
    .map((item) => (item.id === itemID ? { ...item, ...patch } : item))
    .filter(currentFiltersMatch)
  items.value = sortItemsDescending(nextItems)
  nextOffset.value = items.value.length
  if (items.value.length < previousLength) {
    totalCount.value = Math.max(items.value.length, totalCount.value - (previousLength - items.value.length))
  }
  writeItemsToCache()
}

function restoreCachedItemState(
  item: FeedItem,
  patch: Partial<Pick<FeedItem, 'is_read' | 'is_favorite' | 'is_hidden'>>,
) {
  const restoredItem = { ...item, ...patch }
  let restored = false
  const nextItems = items.value.map((current) => {
    if (current.id !== item.id) {
      return current
    }
    restored = true
    return restoredItem
  })
  if (!restored && currentFiltersMatch(restoredItem)) {
    nextItems.push(restoredItem)
  }
  items.value = sortItemsDescending(nextItems.filter(currentFiltersMatch))
  nextOffset.value = items.value.length
  totalCount.value = Math.max(totalCount.value, items.value.length)
  writeItemsToCache()
}

async function updateItemState(item: FeedItem, key: ItemStateKey) {
  if (itemActionBusy.value[item.id]) {
    return
  }
  itemActionBusy.value = { ...itemActionBusy.value, [item.id]: key }
  const previousState = {
    is_read: item.is_read,
    is_favorite: item.is_favorite,
    is_hidden: item.is_hidden,
  }
  try {
    if (key === 'read') {
      const nextValue = !item.is_read
      updateCachedItemState(item.id, { is_read: nextValue })
      const state = await setItemRead(item.id, nextValue)
      updateCachedItemState(item.id, { is_read: state.is_read })
      return
    }
    if (key === 'favorite') {
      const nextValue = !item.is_favorite
      updateCachedItemState(item.id, { is_favorite: nextValue })
      const state = await setItemFavorite(item.id, nextValue)
      updateCachedItemState(item.id, { is_favorite: state.is_favorite })
      return
    }
    const nextValue = !item.is_hidden
    updateCachedItemState(item.id, { is_hidden: nextValue })
    const state = await setItemHidden(item.id, nextValue)
    updateCachedItemState(item.id, { is_hidden: state.is_hidden })
  } catch (err) {
    restoreCachedItemState(item, previousState)
    showFeedNotice('warning', `状态更新失败：${formatAPIError(err)}`)
  } finally {
    itemActionBusy.value = { ...itemActionBusy.value, [item.id]: '' }
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
  if (triggerRect.top <= viewportBottom + loadMorePreloadMargin) {
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
      rootMargin: `${loadMorePreloadMargin}px 0px`,
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

function notifyTopPullEndIfStarted() {
  if (!topPullStartedNotified) {
    return
  }

  topPullStartedNotified = false
  emit('top-pull-end', false)
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
  notifyTopPullEndIfStarted()
  invalidateLoadRequests()
  loading.value = false
  loadingMore.value = false
  error.value = ''
  pullRefresh.reset()
  refreshLayoutFreeze.release()
  refreshNoticeSequence.cancel()
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

function touchStartsBelowTopChrome(touch: Touch) {
  return touch.clientY >= Math.max(0, props.headerHeight)
}

function feedIsAtPullStart() {
  return props.scrollTop <= feedVisibleContentTopOffset(props.headerHeight) + 1
}

function handleTouchStart(event: TouchEvent) {
  if (
    !props.active ||
    !feedIsAtPullStart() ||
    event.touches.length !== 1 ||
    loading.value ||
    refreshing.value ||
    loadingMore.value
  ) {
    cancelPullGesture()
    return
  }

  const touch = event.touches[0]
  if (!touchStartsBelowTopChrome(touch)) {
    cancelPullGesture()
    return
  }

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

  if (!feedIsAtPullStart()) {
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
    if (event.cancelable) {
      event.preventDefault()
    }
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
  void loadFilterSources()
  loadInitialItems()
  window.addEventListener('focus', refreshStaleCacheInBackground)
  document.addEventListener('visibilitychange', handleVisibilityRefresh)
})

onUnmounted(() => {
  disposed = true
  notifyTopPullEndIfStarted()
  invalidateLoadRequests()
  pullRefresh.reset()
  refreshLayoutFreeze.release()
  refreshNoticeSequence.cancel()
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
      void loadFilterSources()
      loadInitialItems()
    }
  },
)

watch(filtersVisible, (visible) => {
  if (visible) {
    void loadFilterSources()
  }
})

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
      <Transition name="sources-toast-motion">
        <div
          v-if="feedNotice"
          class="sources-toast feed-toast"
          :class="`sources-toast--${feedNotice.type}`"
          role="status"
          aria-live="polite"
        >
          {{ feedNotice.message }}
        </div>
      </Transition>
    </Teleport>

    <div ref="feedBodyRef" class="feed-list-body" :style="feedBodyStyle">
      <section v-if="filtersVisible" class="feed-filter-bar" aria-label="时间线筛选">
        <label class="feed-filter-bar__field">
          <span>来源</span>
          <select v-model.number="selectedSourceID" :disabled="filtersLoading">
            <option :value="0">全部来源</option>
            <option v-for="source in sources" :key="source.id" :value="source.id">
              {{ source.name }}
            </option>
          </select>
        </label>
        <label class="feed-filter-bar__field">
          <span>阅读</span>
          <select v-model="readFilter">
            <option value="all">全部</option>
            <option value="false">未读</option>
            <option value="true">已读</option>
          </select>
        </label>
        <label class="feed-filter-bar__field">
          <span>收藏</span>
          <select v-model="favoriteFilter">
            <option value="all">全部</option>
            <option value="true">已收藏</option>
            <option value="false">未收藏</option>
          </select>
        </label>
        <label class="feed-filter-bar__field">
          <span>隐藏</span>
          <select v-model="hiddenFilter">
            <option value="visible">可见</option>
            <option value="all">全部</option>
            <option value="hidden">已隐藏</option>
          </select>
        </label>
      </section>
      <a-alert
        v-if="filterErrorMessage"
        class="feed-alert"
        type="warning"
        :content="filterErrorMessage"
        show-icon
      />
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
          v-for="item in items"
          :key="item.id"
          class="feed-item"
          :class="{ 'feed-item--morphing': morphingItemId === item.id }"
          :data-feed-item-id="item.id"
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
          <div
            class="feed-item__actions"
            :style="feedItemPreviewStyle(item)"
            @pointerdown.stop
            @touchstart.stop
          >
            <ActionBar
              compact
              :is-read="item.is_read"
              :is-favorite="item.is_favorite"
              :is-hidden="item.is_hidden"
              :busy-key="itemActionBusy[item.id]"
              @toggle-read="updateItemState(item, 'read')"
              @toggle-favorite="updateItemState(item, 'favorite')"
              @toggle-hidden="updateItemState(item, 'hidden')"
            />
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
        <button
          v-else-if="canLoadMore"
          class="feed-load-more feed-load-more--button"
          type="button"
          data-load-more-trigger="true"
          @click="loadItems({ append: true })"
        >
          加载更多
        </button>
        <div
          v-else-if="reachedEnd && hasItems"
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
