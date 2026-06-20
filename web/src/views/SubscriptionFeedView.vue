<script setup lang="ts">
import { computed, nextTick, onMounted, onUnmounted, ref, watch } from 'vue'
import { IconRefresh } from '@arco-design/web-vue/es/icon'

import { formatAPIError } from '@/api/client'
import { fetchActiveSources, fetchSource, type FeedItem, listRecommendationItems, listTimelineItems } from '@/api/feed'
import { useFeedInteractionStore } from '@/stores/feedInteraction'

type FeedMode = 'subscriptions' | 'recommendations' | 'source'
type SourceKind = 'subscriptions' | 'recommendations'

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
  },
)

const emit = defineEmits<{
  topPullStart: [startedWithVisibleChrome: boolean]
  topPullMove: [distance: number]
  topPullEnd: [shouldRefresh: boolean]
  openItem: [item: FeedItem, sourceKind: SourceKind, originRect?: DOMRect]
}>()

const feedInteraction = useFeedInteractionStore()
const items = ref<FeedItem[]>([])
const loading = ref(false)
const refreshing = ref(false)
const loadingMore = ref(false)
const error = ref('')
const feedNotice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
const lastUpdatedAt = ref('')
const totalCount = ref(0)
const nextOffset = ref(0)
const reachedEnd = ref(false)
const pullOffset = ref(0)
const pullDragging = ref(false)
const pullSettling = ref(false)
const pullStartedWithVisibleChrome = ref(false)
const feedPageRef = ref<HTMLElement | null>(null)

const pullThreshold = 76
const pullMaxOffset = 116
const pageSize = 10
const verticalLockRatio = 1.18
const motionCleanupBuffer = 96
let touchStartY = 0
let touchStartX = 0
let touchStartChromeDistance = 0
let trackingPullCandidate = false
let trackingPull = false
let pullSettleTimer = 0
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
const usesGlobalPullState = computed(() => true)
const effectiveSourceKind = computed<SourceKind>(() => {
  if (isSourceMode.value) {
    return props.sourceKind
  }
  return props.mode === 'recommendations' ? 'recommendations' : 'subscriptions'
})
const copy = computed(() => {
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
const pullProgress = computed(() => Math.min(pullOffset.value / pullThreshold, 1))
const pullActive = computed(() => pullOffset.value > 0 || refreshing.value)
const pullStatusText = computed(() => {
  if (refreshing.value) {
    return '正在刷新'
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
const feedBodyStyle = computed(() => ({
  transform: freezeBodyForTopChromePull.value
    ? 'translate3d(0, 0, 0)'
    : `translate3d(0, ${cssPx(Math.min(pullOffset.value * 0.34, 38))}, 0)`,
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

function cssRotate(degrees: number) {
  return `rotate(${(Number.isFinite(degrees) ? degrees : 0).toFixed(2)}deg)`
}

function itemTime(item: FeedItem) {
  const value = item.published_at || item.fetched_at
  const timestamp = value ? new Date(value).getTime() : 0
  return Number.isFinite(timestamp) ? timestamp : 0
}

function compareItemsAscending(left: FeedItem, right: FeedItem) {
  const timeDelta = itemTime(left) - itemTime(right)
  return timeDelta === 0 ? left.id - right.id : timeDelta
}

function sortItemsAscending(nextItems: FeedItem[]) {
  return [...nextItems].sort(compareItemsAscending)
}

function mergeItems(currentItems: FeedItem[], nextItems: FeedItem[]) {
  const itemMap = new Map<number, FeedItem>()
  for (const item of currentItems) {
    itemMap.set(item.id, item)
  }
  for (const item of nextItems) {
    itemMap.set(item.id, item)
  }
  return sortItemsAscending(Array.from(itemMap.values()))
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

function showFeedNotice(type: 'success' | 'warning', message: string) {
  const normalized = message.trim()
  if (!normalized) {
    feedNotice.value = null
    return
  }
  feedNotice.value = { type, message: normalized }
  window.clearTimeout(feedNoticeTimer)
  feedNoticeTimer = window.setTimeout(() => {
    feedNotice.value = null
  }, 2600)
}

async function refreshSubscriptionSources() {
  if (effectiveSourceKind.value !== 'subscriptions') {
    return null
  }
  try {
    if (isSourceMode.value && props.sourceId > 0) {
      await fetchSource(props.sourceId)
      return { type: 'success' as const, message: '已刷新当前订阅源' }
    }

    const result = await fetchActiveSources()
    if (result.requested_count === 0) {
      return { type: 'warning' as const, message: '没有可抓取的订阅源' }
    }
    if (result.failure_count > 0) {
      return { type: 'warning' as const, message: `已刷新 ${result.success_count} 个订阅源，${result.failure_count} 个失败` }
    }
    return { type: 'success' as const, message: `已刷新 ${result.success_count} 个订阅源` }
  } catch (err) {
    return { type: 'warning' as const, message: formatAPIError(err) }
  }
}

async function loadItems(options: { refresh?: boolean; append?: boolean } = {}) {
  const isRefresh = Boolean(options.refresh)
  const isAppend = Boolean(options.append)
  if (loading.value || refreshing.value || loadingMore.value || (isAppend && !canLoadMore.value)) {
    return
  }
  error.value = ''
  if (isAppend) {
    loadingMore.value = true
  } else if (isRefresh) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  try {
    const refreshNotice = isRefresh ? await refreshSubscriptionSources() : null
    const loader = effectiveSourceKind.value === 'recommendations' ? listRecommendationItems : listTimelineItems
    const requestOffset = isAppend ? nextOffset.value : 0
    const params = {
      limit: pageSize,
      offset: requestOffset,
      order: 'asc' as const,
      ...(isRefresh && effectiveSourceKind.value === 'recommendations' ? { refresh: true } : {}),
      ...(isSourceMode.value && props.sourceId > 0 ? { source_id: props.sourceId } : {}),
    }
    const result = await loader(params)
    items.value = isAppend ? mergeItems(items.value, result.items) : sortItemsAscending(result.items)
    totalCount.value = result.total
    nextOffset.value = requestOffset + result.items.length
    reachedEnd.value = result.items.length < pageSize || nextOffset.value >= result.total
    lastUpdatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
    if (refreshNotice) {
      showFeedNotice(refreshNotice.type, refreshNotice.message)
    }
  } catch (err) {
    error.value = formatAPIError(err)
  } finally {
    loading.value = false
    loadingMore.value = false
    pullDragging.value = false
    scheduleLoadMoreObserver()

    if (!isRefresh) {
      refreshing.value = false
      return
    }

    pullSettling.value = true
    window.clearTimeout(pullSettleTimer)
    setPullState({
      offset: pullOffset.value,
      active: true,
      refreshing: true,
      statusText: '正在刷新',
      statusMeta: pullStatusMeta.value,
    })
    window.setTimeout(() => {
      pullOffset.value = 0
      refreshing.value = false
      clearPullState()
      pullStartedWithVisibleChrome.value = false
      pullSettleTimer = window.setTimeout(() => {
        pullSettling.value = false
      }, 260 + motionCleanupBuffer)
    }, 120)
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
  trackingPullCandidate = false
  trackingPull = false
  pullDragging.value = false
}

function resetPullGesture(force = false) {
  resetPullTracking()
  pullOffset.value = 0
  clearPullState(force)
  pullStartedWithVisibleChrome.value = false
}

function rubberBandPullDistance(distance: number) {
  if (distance <= pullThreshold) {
    return distance
  }
  return Math.min(pullThreshold + (distance - pullThreshold) * 0.42, pullMaxOffset)
}

function handleTouchStart(event: TouchEvent) {
  if (
    !props.active ||
    (!hasItems.value && !loading.value) ||
    props.scrollTop > 0 ||
    event.touches.length !== 1 ||
    loading.value ||
    refreshing.value
  ) {
    resetPullTracking()
    pullStartedWithVisibleChrome.value = false
    return
  }

  const touch = event.touches[0]
  touchStartX = touch.clientX
  touchStartY = touch.clientY
  touchStartChromeDistance = props.headerHeight * props.topChromeProgress
  pullStartedWithVisibleChrome.value =
    props.freezeBodyDuringTopRefresh || props.topChromeProgress > 0.04
  trackingPullCandidate = true
  trackingPull = false
  emit('topPullStart', pullStartedWithVisibleChrome.value)
}

function handleTouchMove(event: TouchEvent) {
  if (!props.active || ((!trackingPullCandidate && !trackingPull) || props.scrollTop > 0 || event.touches.length !== 1)) {
    return
  }

  const touch = event.touches[0]
  const deltaX = touch.clientX - touchStartX
  const deltaY = touch.clientY - touchStartY

  if (!trackingPull) {
    if (deltaY <= 0 || Math.abs(deltaX) > Math.abs(deltaY) * verticalLockRatio) {
      resetPullGesture()
      return
    }

    if (deltaY < 2 || Math.abs(deltaY) <= Math.abs(deltaX) * verticalLockRatio) {
      return
    }

    trackingPull = true
    trackingPullCandidate = false
    pullDragging.value = true
  }

  if (trackingPull) {
    event.preventDefault()
    emit('topPullMove', Math.max(0, deltaY))
    const refreshDistance = Math.max(0, deltaY - touchStartChromeDistance)
    if (refreshDistance <= 0) {
      pullOffset.value = 0
      clearPullState()
      return
    }
    pullOffset.value = rubberBandPullDistance(refreshDistance)
  }
}

function handleTouchEnd() {
  if (!trackingPull) {
    resetPullGesture()
    emit('topPullEnd', false)
    return
  }

  const shouldRefresh = pullOffset.value >= pullThreshold
  resetPullTracking()

  if (shouldRefresh) {
    pullOffset.value = pullThreshold
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
  if (props.active) {
    void loadItems()
  }
})

onUnmounted(() => {
  window.clearTimeout(pullSettleTimer)
  window.clearTimeout(loadMoreSyncTimer)
  window.clearTimeout(feedNoticeTimer)
  stopLoadMoreObserver()
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
    if (!hasItems.value && !loading.value) {
      void loadItems()
    }
  },
)

watch(
  () => [props.mode, props.sourceKind, props.sourceId] as const,
  () => {
    if (props.active) {
      void loadItems()
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
      if (pullStartedWithVisibleChrome.value && trackingPull) {
        return
      }
      clearPullState()
      return
    }
    window.clearTimeout(pullSettleTimer)
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
    <div class="feed-list-body" :style="feedBodyStyle">
      <div
        v-if="false"
        class="feed-local-refresh"
        :class="{ 'feed-local-refresh--refreshing': refreshing }"
        aria-live="polite"
      >
        <span
          class="feed-local-refresh__icon"
          :style="{ transform: refreshing ? undefined : cssRotate(pullProgress * 300) }"
        >
          <IconRefresh />
        </span>
        <span class="feed-local-refresh__copy">
          <strong>{{ pullStatusText }}</strong>
          <small>{{ pullStatusMeta }}</small>
        </span>
      </div>

      <a-alert v-if="feedNotice?.message" class="feed-alert" :type="feedNotice.type" :content="feedNotice.message" show-icon />
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
