<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { IconRefresh } from '@arco-design/web-vue/es/icon'

import { formatAPIError } from '@/api/client'
import { type FeedItem, listRecommendationItems, listTimelineItems } from '@/api/feed'
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
  openSource: [source: { id: number; name: string; kind: SourceKind }]
}>()

const feedInteraction = useFeedInteractionStore()
const items = ref<FeedItem[]>([])
const loading = ref(false)
const refreshing = ref(false)
const error = ref('')
const lastUpdatedAt = ref('')
const pullOffset = ref(0)
const pullDragging = ref(false)
const pullSettling = ref(false)
const pullStartedWithVisibleChrome = ref(false)

const pullThreshold = 76
const pullMaxOffset = 116
const verticalLockRatio = 1.18
let touchStartY = 0
let touchStartX = 0
let touchStartChromeDistance = 0
let trackingPullCandidate = false
let trackingPull = false
let pullSettleTimer = 0

const viewKey = computed(() => `${props.mode}:${props.sourceKind}:${props.sourceId}`)
const hasItems = computed(() => items.value.length > 0)
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
const pageClass = computed(() => ({
  'feed-list-page--pulling': pullActive.value,
  'feed-list-page--dragging': pullDragging.value,
  'feed-list-page--settling': !pullDragging.value && pullOffset.value > 0,
}))
const freezeBodyForTopChromePull = computed(
  () => props.freezeBodyDuringTopRefresh || pullStartedWithVisibleChrome.value,
)
const feedBodyStyle = computed(() => ({
  transform: freezeBodyForTopChromePull.value
    ? 'translate3d(0, 0, 0)'
    : `translate3d(0, ${Math.min(Math.round(pullOffset.value * 0.34), 38)}px, 0)`,
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

function itemSummary(item: FeedItem) {
  return item.content_text || item.content_snippet || '暂无摘要。'
}

function feedItemStyle(item: FeedItem) {
  const style: Record<string, string> = {}
  const locksHeight =
    (props.morphingItemId === item.id || props.morphingHeightLockItemId === item.id) && props.morphingItemHeight

  if (locksHeight && props.morphingItemHeight) {
    style.height = `${Math.round(props.morphingItemHeight)}px`
  }

  if (props.morphingItemId === item.id) {
    const progress = safeMorphingPreviewProgress.value
    style['--feed-morph-preview-opacity'] = progress.toFixed(3)
    style['--feed-morph-preview-shift'] = `${Math.round((1 - progress) * 6)}px`
  }

  return Object.keys(style).length > 0 ? style : undefined
}

async function loadItems(options: { refresh?: boolean } = {}) {
  if (loading.value || refreshing.value) {
    return
  }
  error.value = ''
  const isRefresh = Boolean(options.refresh)
  if (isRefresh) {
    refreshing.value = true
  } else {
    loading.value = true
  }
  try {
    const loader = effectiveSourceKind.value === 'recommendations' ? listRecommendationItems : listTimelineItems
    const params = {
      limit: 40,
      offset: 0,
      ...(isSourceMode.value && props.sourceId > 0 ? { source_id: props.sourceId } : {}),
    }
    const result = await loader(params)
    items.value = result.items
    lastUpdatedAt.value = new Date().toLocaleTimeString('zh-CN', { hour12: false })
  } catch (err) {
    error.value = formatAPIError(err)
  } finally {
    loading.value = false
    pullDragging.value = false

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
      }, 260)
    }, 120)
  }
}

function openItem(item: FeedItem, event: MouseEvent) {
  const target = event.currentTarget
  const originRect = target instanceof HTMLElement ? target.closest('.feed-item')?.getBoundingClientRect() : undefined
  emit('openItem', item, effectiveSourceKind.value, originRect)
}

function openSource(item: FeedItem) {
  if (!item.source_id) {
    return
  }
  emit('openSource', {
    id: item.source_id,
    name: item.source_name || '未知来源',
    kind: effectiveSourceKind.value,
  })
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
  loadItems()
})

onUnmounted(() => {
  window.clearTimeout(pullSettleTimer)
  if (usesGlobalPullState.value && feedInteraction.pullViewKey === viewKey.value) {
    feedInteraction.resetPullState()
  }
})

watch(
  () => props.active,
  (active) => {
    if (!active) {
      resetPullGesture(true)
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
          :style="{ transform: refreshing ? undefined : `rotate(${Math.round(pullProgress * 300)}deg)` }"
        >
          <IconRefresh />
        </span>
        <span class="feed-local-refresh__copy">
          <strong>{{ pullStatusText }}</strong>
          <small>{{ pullStatusMeta }}</small>
        </span>
      </div>

      <a-alert v-if="error" class="feed-alert" type="warning" :content="error" show-icon />

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
          <div class="feed-item__meta">
            <button class="feed-item__source" type="button" @click.stop="openSource(item)">
              {{ item.source_name || '未知来源' }}
            </button>
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
      </div>
    </div>
  </section>
</template>
