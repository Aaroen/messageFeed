<script setup lang="ts">
import { computed, type StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import SubscriptionFeedView from '@/views/SubscriptionFeedView.vue'

type FeedSourceKind = 'subscriptions' | 'recommendations'

const props = withDefaults(
  defineProps<{
    activeKey?: string | symbol | null
    detailReaderOpen?: boolean
    sourceReaderOpen?: boolean
    feedTrackStyle?: StyleValue
    feedScrollTop?: number
    topChromeProgress?: number
    feedHeaderHeight?: number
    freezeBodyDuringTopRefresh?: boolean
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    feedItemPreviewProgress?: number
  }>(),
  {
    activeKey: null,
    detailReaderOpen: false,
    sourceReaderOpen: false,
    feedTrackStyle: undefined,
    feedScrollTop: 0,
    topChromeProgress: 1,
    feedHeaderHeight: 86,
    freezeBodyDuringTopRefresh: false,
    morphingItemId: null,
    morphingHeightLockItemId: null,
    morphingItemHeight: null,
    feedItemPreviewProgress: 0,
  },
)

const emit = defineEmits<{
  (event: 'top-pull-start', startedWithVisibleChrome: boolean): void
  (event: 'top-pull-move', distance: number): void
  (event: 'top-pull-end', shouldRefresh: boolean): void
  (event: 'open-item', item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect): void
}>()

const readerClosed = computed(() => !props.detailReaderOpen && !props.sourceReaderOpen)
const subscriptionsActive = computed(() => props.activeKey === 'subscriptions' && readerClosed.value)
const recommendationsActive = computed(() => props.activeKey === 'recommendations' && readerClosed.value)

function handleOpenItem(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
  emit('open-item', item, sourceKind, originRect)
}
</script>

<template>
  <div class="feed-stage">
    <div class="feed-track" :style="feedTrackStyle">
      <section class="feed-pane" :aria-hidden="!subscriptionsActive" :inert="!subscriptionsActive">
        <SubscriptionFeedView
          mode="subscriptions"
          :active="subscriptionsActive"
          :scroll-top="feedScrollTop"
          :top-chrome-progress="topChromeProgress"
          :header-height="feedHeaderHeight"
          :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
          :morphing-item-id="morphingItemId"
          :morphing-height-lock-item-id="morphingHeightLockItemId"
          :morphing-item-height="morphingItemHeight"
          :morphing-preview-progress="feedItemPreviewProgress"
          @top-pull-start="emit('top-pull-start', $event)"
          @top-pull-move="emit('top-pull-move', $event)"
          @top-pull-end="emit('top-pull-end', $event)"
          @open-item="handleOpenItem"
        />
      </section>
      <section class="feed-pane" :aria-hidden="!recommendationsActive" :inert="!recommendationsActive">
        <SubscriptionFeedView
          mode="recommendations"
          :active="recommendationsActive"
          :scroll-top="feedScrollTop"
          :top-chrome-progress="topChromeProgress"
          :header-height="feedHeaderHeight"
          :freeze-body-during-top-refresh="freezeBodyDuringTopRefresh"
          :morphing-item-id="morphingItemId"
          :morphing-height-lock-item-id="morphingHeightLockItemId"
          :morphing-item-height="morphingItemHeight"
          :morphing-preview-progress="feedItemPreviewProgress"
          @top-pull-start="emit('top-pull-start', $event)"
          @top-pull-move="emit('top-pull-move', $event)"
          @top-pull-end="emit('top-pull-end', $event)"
          @open-item="handleOpenItem"
        />
      </section>
    </div>
  </div>
</template>
