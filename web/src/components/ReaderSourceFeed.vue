<script setup lang="ts">
import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'
import SubscriptionFeedView from '@/views/SubscriptionFeedView.vue'

withDefaults(
  defineProps<{
    readerSource?: ReaderSource | null
    scrollTop?: number
    topChromeProgress?: number
    headerHeight?: number
    morphingItemId?: number | null
    morphingHeightLockItemId?: number | null
    morphingItemHeight?: number | null
    morphingPreviewProgress?: number
    backgroundRefresh?: boolean
  }>(),
  {
    readerSource: null,
    scrollTop: 0,
    topChromeProgress: 1,
    headerHeight: 86,
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
  'open-item': [item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect]
}>()
</script>

<template>
  <SubscriptionFeedView
    v-if="readerSource"
    mode="source"
    :source-kind="readerSource.kind"
    :source-id="readerSource.id"
    :active="true"
    :scroll-top="scrollTop"
    :top-chrome-progress="topChromeProgress"
    :header-height="headerHeight"
    :freeze-body-during-top-refresh="true"
    :morphing-item-id="morphingItemId"
    :morphing-height-lock-item-id="morphingHeightLockItemId"
    :morphing-item-height="morphingItemHeight"
    :morphing-preview-progress="morphingPreviewProgress"
    :background-refresh="backgroundRefresh"
    @top-pull-start="(startedWithVisibleChrome) => emit('top-pull-start', startedWithVisibleChrome)"
    @top-pull-move="(distance) => emit('top-pull-move', distance)"
    @top-pull-end="(shouldRefresh) => emit('top-pull-end', shouldRefresh)"
    @open-item="(item, sourceKind, originRect) => emit('open-item', item, sourceKind, originRect)"
  />
</template>
