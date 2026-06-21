<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'

import { formatAPIError } from '@/api/client'
import { fetchActiveSources, fetchSource, type FeedItem, listRecommendationItems, listTimelineItems } from '@/api/feed'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { usePullRefresh } from '@/composables/usePullRefresh'
import { useFeedInteractionStore } from '@/stores/feedInteraction'
import { type FeedListCacheEntry, useFeedListCacheStore } from '@/stores/feedListCache'

type FeedMode = 'subscriptions' | 'recommendations' | 'source'
type SourceKind = 'subscriptions' | 'recommendations'
type FeedNotice = { type: 'success' | 'warning'; message: string }

const props = withDefaults(
  defineProps<{
    mode?: FeedMode
    sourceKind?: SourceKind
    sourceId?: number
    active?: boolean
    scrollTop?: number
    topChromeProgress?: number
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
  topPullStart: [startedWithVisibleChrome: boolean]
  topPullMove: [distance: number]
  topPullEnd: [shouldRefresh: boolean]
  openItem: [item: FeedItem, sourceKind: SourceKind, originRect?: DOMRect]
}>()

const feedInteraction = useFeedInteractionStore()
const feedListCache = useFeedListCacheStore()
const motionTimings = useMotionTimings()
const items = ref<FeedItem[]>([])
const loading = ref(false)
const loadingMore = ref(false)
const error = ref('')
const feedNotice = ref<FeedNotice | null>(null)
const lastUpdatedAt = ref('')
const totalCount = ref(0)
const nextOffset = ref(0)
const reachedEnd = ref(false)
const feedPageRef = ref<HTMLElement | null>(null)
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
const pageSize = 10
const cacheTTLMS = 60 * 1000
const verticalLockRatio = 1.18
const motionCleanupBuffer = motionTimings.motionCleanupBuffer
const pullReleaseDelay = motionTimings.topRefreshReleaseDelay
const pullSettleDuration = motionTimings.topRefreshSettleDuration
const noticeRevealDelay = motionTimings.noticeRevealDelay
let touchStartChromeDistance = 0
let loadMoreSyncTimer = 0
let feedNoticeTimer = 0
let loadMoreObserver: IntersectionObserver | null = null

const viewKey = computed(() => `${props.mode}:${props.sourceKind}:${props.sourceId}`)
const hasItems = computed(() => items.value.length > 0)
const canLoadMore = computed(
  () => props.active && hasItems.value && !loading.value && !refreshing.value && !loadingMore.value && !reachedEnd.value,
)
const loadMoreTriggerIndex = computed(() => Math.max(items.value.length - 3, 0))
const isRecommendations = computed(() => props.mode === 'recommendations')
const isSourceMode = computed(() => props.mode === 'source')
const usesGlobalPullState = computed(() => !props.backgroundRefresh)
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
  'feed-list-page--dragging': pullDragging.value,
  'feed-list-page--settling': !pullDragging.value && pullOffset.value > 0,
  'feed-list-page--empty': !hasItems.value && !loading.value,
}))
const freezeBodyForTopChromePull = computed(
  () => props.freezeBodyDuringTopRefresh || pullStartedWithVisibleChrome.value,
)
const feedBodyShift = computed(() => {
  const shiftRatio = hasItems.value ? 0.34 : 0.2
  const maxShift = hasItems.value ? 38 : 18
  return Math.min(pullOffset.value * shiftRatio, maxShift)
})
const feedBodyStyle = computed(() => ({
  transform: freezeBodyForTopChromePull.value
    ? 'translate3d(0, 0, 0)'
    : `translate3d(0, ${cssPx(feedBodyShift.value)}, 0)`,
}))
const safeMorphingPreviewProgress = computed(() => Math.min(Math.max(props.morphingPreviewProgress, 0), 1))

function canWritePullState(force = false) {
  if (!usesGlobalPullState.value) {
    return false
  }
  return props.active || force || feedInteraction.pullViewKey === viewKey.value
}

function setPullState(
  payload: {
    offset: number
    active: boolean
    refreshing: boolean
    statusText: string
    statusMeta: string
  },
  force = false,
) {
  if (!canWritePullState(force)) {
    return
  }
  feedInteraction.setPullState({
    ...payload,
    viewKey: viewKey.value,
    lastUpdatedAt: lastUpdatedAt.value,
  })
}

function clearPullState(force = false) {
  setPullState(
    {
      offset: 0,
      active: false,
      refreshing: false,
      statusText: '',
      statusMeta: '',
    },
    force,
  )
}

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

function writeItemsToCache() {
  feedListCache.set(viewKey.value, {
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

function loadInitialItems() {
  const restored = restoreItemsFromCache()
  if (!props.active) {
    return
  }
  if (!restored) {
    void loadItems({ refresh: isSourceMode.value })
    return
  }
  if (shouldRefreshCache(restored)) {
    void loadItems({ refresh: true, background: true })
  }
}

function refreshStaleCacheInBackground() {
  if (!props.active || loading.value || refreshing.value || loadingMore.value) {
    return
  }
  const entry = feedListCache.get(viewKey.value)
  if (!entry || cacheEntryIsFresh(entry)) {
    return
  }
  void loadItems({ refresh: true, background: true })
}

function handleVisibilityRefresh() {
  if (document.visibilityState === 'visible') {
    refreshStaleCacheInBackground()
  }
}

function feedItemStyle(item: FeedItem) {
  const style: Record<string, string> = {}
  const locksHeight =
    (props.morphingItemId === item.id || props.morphingHeightLockItemId === item.id) && props.morphingItemHeight

  if (locksHeight && props.morphingItemHeight) {
    style.height = cssPx(props.morphingItemHeight)
  }

  if (props.morphingItemId === item.id) {
    const progress = safeMorphingPreviewProgress.value
    style['--feed-morph-preview-opacity'] = progress.toFixed(3)
    style['--feed-morph-preview-shift'] = cssPx((1 - progress) * 6)
    style['--feed-morph-preview-blur'] = `${((1 - progress) * 2.4).toFixed(2)}px`
  }

  return Object.keys(style).length > 0 ? style : undefined
}

function showFeedNotice(type: FeedNotice['type'], message: string, delayMS = 0) {
  const normalized = message.trim()
  if (!normalized) {
    feedNotice.value = null
    return
  }
  window.clearTimeout(feedNoticeTimer)
  const show = () => {
    feedNotice.value = { type, message: normalized }
    feedNoticeTimer = window.setTimeout(() => {
      feedNotice.value = null
    }, motionTimings.noticeDuration(type))
  }
  if (delayMS > 0) {
    feedNoticeTimer = window.setTimeout(show, delayMS)
    return
  }
  show()
}

function formatFetchErrors(errors: Array<{ source_name?: string; message: string }> = []) {
  const details = errors
    .map((item) => {
      const name = item.source_name?.trim() || '未知来源'
      const message = item.message.trim()
      return message ? `${name}：${message}` : name
    })
    .filter(Boolean)
    .slice(0, 3)
  if (!details.length) {
    return '服务未返回具体错误原因'
  }
  const overflow = errors.length > details.length ? `；另有 ${errors.length - details.length} 个失败来源` : ''
  return `${details.join('；')}${overflow}`
}

function refreshSuccessMessage() {
  if (isSourceMode.value) {
    return '刷新成功：已更新当前来源'
  }
  if (effectiveSourceKind.value === 'recommendations') {
    return '刷新成功：已更新当前推荐来源'
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
    if (result.requested_count === 0) {
      return { type: 'warning' as const, message: '刷新异常：当前没有可抓取的订阅源，请在订阅管理中开启或导入来源' }
    }
    if (result.failure_count > 0) {
      const prefix = result.success_count > 0 ? '刷新异常' : '刷新失败'
      return {
        type: 'warning' as const,
        message: `${prefix}：已刷新 ${result.success_count} 个订阅源，${result.failure_count} 个失败。失败原因：${formatFetchErrors(result.errors)}`,
      }
    }
    return { type: 'success' as const, message: refreshSuccessMessage() }
  } catch (err) {
    return { type: 'warning' as const, message: `刷新失败：${formatAPIError(err)}` }
  }
}

async function loadItems(options: { refresh?: boolean; append?: boolean; background?: boolean } = {}) {
  const isRefresh = Boolean(options.refresh)
  const isAppend = Boolean(options.append)
  const isBackground = Boolean(options.background)
  const isBackgroundRefresh = isBackground || props.backgroundRefresh
  if (loading.value || refreshing.value || loadingMore.value || (isAppend && !canLoadMore.value)) {
    return
  }
  error.value = ''
  if (isAppend) {
    loadingMore.value = true
  } else if (isRefresh && !isBackgroundRefresh) {
    pullRefresh.beginRefreshing()
  } else if (!isBackgroundRefresh) {
    loading.value = true
  }
  try {
    const refreshNotice = isRefresh ? await refreshSubscriptionSources() : null
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
      const reloaded = await loader(params)
      nextItems = reloaded.items
      nextTotal = reloaded.total
    }

    items.value = isAppend ? mergeItems(items.value, nextItems) : sortItemsDescending(nextItems)
    totalCount.value = nextTotal
    nextOffset.value = requestOffset + nextItems.length
    reachedEnd.value = nextItems.length < pageSize || nextOffset.value >= nextTotal
    lastUpdatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    writeItemsToCache()
    if (!isBackgroundRefresh && refreshNotice) {
      showFeedNotice(refreshNotice.type, refreshNotice.message, noticeRevealDelay)
    } else if (!isBackgroundRefresh && autoFetchNotice && autoFetchNotice.type === 'warning') {
      showFeedNotice(autoFetchNotice.type, autoFetchNotice.message)
    } else if (!isBackgroundRefresh && isRefresh) {
      showFeedNotice('success', refreshSuccessMessage(), noticeRevealDelay)
    }
  } catch (err) {
    const message = formatAPIError(err)
    if (isRefresh) {
      if (isBackgroundRefresh) {
        error.value = `刷新失败：${message}`
      } else {
        showFeedNotice('warning', `刷新失败：${message}`, noticeRevealDelay)
      }
    } else {
      error.value = `加载失败：${message}`
    }
  } finally {
    loading.value = false
    loadingMore.value = false
    scheduleLoadMoreObserver()

    if (!isRefresh) {
      pullRefresh.finishRefreshing()
      return
    }

    if (isBackgroundRefresh) {
      pullRefresh.finishBackgroundRefresh()
      return
    }

    setPullState({
      offset: pullOffset.value,
      active: true,
      refreshing: true,
      statusText: isSourceMode.value ? '抓取中' : '正在刷新',
      statusMeta: pullStatusMeta.value,
    })
    pullRefresh.settleRefreshCompletion({
      releaseDelayMS: pullReleaseDelay,
      settleDelayMS: pullSettleDuration + motionCleanupBuffer,
      afterRelease: clearPullState,
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
      if (entries.some((entry) => entry.isIntersecting)) {
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
  window.clearTimeout(loadMoreSyncTimer)
  loadMoreSyncTimer = window.setTimeout(() => {
    void nextTick(syncLoadMoreObserver)
  }, 0)
}

function openItem(item: FeedItem, event: MouseEvent) {
  const target = event.currentTarget
  const originRect = target instanceof HTMLElement ? target.closest('.feed-item')?.getBoundingClientRect() : undefined
  emit('openItem', item, effectiveSourceKind.value, originRect)
}

function isInteractiveTarget(target: EventTarget | null) {
  return target instanceof Element && Boolean(target.closest('button, a, input, textarea, select, [role="button"]'))
}

function resetPullTracking() {
  pullRefresh.finishGestureTracking()
}

function resetPullGesture(force = false) {
  pullRefresh.cancelGesture()
  clearPullState(force)
}

function handleTouchStart(event: TouchEvent) {
  if (
    !props.active ||
    props.scrollTop > 0 ||
    event.touches.length !== 1 ||
    loading.value ||
    refreshing.value
  ) {
    pullRefresh.cancelGesture()
    return
  }

  const touch = event.touches[0]
  touchStartChromeDistance = props.headerHeight * props.topChromeProgress
  pullRefresh.begin(props.freezeBodyDuringTopRefresh || props.topChromeProgress > 0.04)
  pullRefresh.beginGestureCandidate(touch.clientX, touch.clientY)
  emit('topPullStart', pullStartedWithVisibleChrome.value)
}

function handleTouchMove(event: TouchEvent) {
  if (
    !props.active ||
    ((!trackingPullCandidate.value && !trackingPull.value) || props.scrollTop > 0 || event.touches.length !== 1)
  ) {
    return
  }

  const touch = event.touches[0]
  const { deltaX, deltaY } = pullRefresh.gestureDelta(touch.clientX, touch.clientY)

  if (!trackingPull.value) {
    if (deltaY <= 0 || Math.abs(deltaX) > Math.abs(deltaY) * verticalLockRatio) {
      resetPullGesture()
      return
    }

    if (deltaY < 2 || Math.abs(deltaY) <= Math.abs(deltaX) * verticalLockRatio) {
      return
    }

    pullRefresh.beginGestureTracking()
  }

  if (trackingPull.value) {
    event.preventDefault()
    emit('topPullMove', Math.max(0, deltaY))
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
    resetPullGesture()
    emit('topPullEnd', false)
    return
  }

  const shouldRefresh = pullOffset.value >= pullThreshold
  resetPullTracking()

  if (shouldRefresh) {
    pullRefresh.commitRefreshOffset()
    emit('topPullEnd', true)
    void loadItems({ refresh: true })
    return
  }

  resetPullGesture()
  emit('topPullEnd', false)
}

function handleTouchCancel() {
  resetPullGesture()
  emit('topPullEnd', false)
}

onMounted(() => {
  loadInitialItems()
  window.addEventListener('focus', refreshStaleCacheInBackground)
  document.addEventListener('visibilitychange', handleVisibilityRefresh)
})

onUnmounted(() => {
  pullRefresh.clearTimers()
  window.clearTimeout(loadMoreSyncTimer)
  window.clearTimeout(feedNoticeTimer)
  stopLoadMoreObserver()
  window.removeEventListener('focus', refreshStaleCacheInBackground)
  document.removeEventListener('visibilitychange', handleVisibilityRefresh)
  if (usesGlobalPullState.value && feedInteraction.pullViewKey === viewKey.value) {
    feedInteraction.resetPullState()
  }
})

watch(
  () => props.active,
  (active) => {
    if (!active) {
      resetPullGesture(true)
      return
    }
    const restored = hasItems.value ? feedListCache.get(viewKey.value) : restoreItemsFromCache()
    if (!hasItems.value && !loading.value) {
      void loadItems({ refresh: isSourceMode.value })
      return
    }
    if (shouldRefreshCache(restored)) {
      void loadItems({ refresh: true, background: true })
    }
  },
)

watch(
  () => [props.mode, props.sourceKind, props.sourceId] as const,
  () => {
    if (props.active) {
      loadInitialItems()
    }
  },
)

watch(
  () => props.scrollTop,
  () => {
    maybeLoadMoreFromTrigger()
  },
)

watch(
  [() => items.value.length, () => props.active, loading, refreshing, loadingMore, reachedEnd],
  () => {
    scheduleLoadMoreObserver()
  },
  { flush: 'post' },
)

watch(
  [pullOffset, refreshing, lastUpdatedAt, () => props.active],
  () => {
    if (!usesGlobalPullState.value || !props.active || pullSettling.value) {
      return
    }

    if (!pullActive.value && !refreshing.value) {
      if (pullStartedWithVisibleChrome.value && trackingPull.value) {
        return
      }
      clearPullState()
      return
    }
    pullRefresh.clearTimers()
    setPullState({
      offset: pullOffset.value,
      active: pullActive.value,
      refreshing: refreshing.value,
      statusText: pullStatusText.value,
      statusMeta: pullStatusMeta.value,
    })
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

    <div class="feed-list-body" :style="feedBodyStyle">
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
          <div class="feed-item__meta">
            <span class="feed-item__source">
              {{ item.source_name || '未知来源' }}
            </span>
            <span>{{ formatDate(item.published_at || item.fetched_at) }}</span>
          </div>
          <button class="feed-item__read-target" type="button" @click="openItem(item, $event)">
            <h2>{{ item.title }}</h2>
            <p>{{ itemSummary(item) }}</p>
          </button>
          <div class="feed-item__actions">
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
