<script setup lang="ts">
import type { StyleValue } from 'vue'

import RefreshStatusLayer from '@/components/RefreshStatusLayer.vue'

type FeedTab = {
  key: string
  label: string
  path: string
}

withDefaults(
  defineProps<{
    detailReaderOpen?: boolean
    detailHeaderLayerStyle?: StyleValue
    detailTitle?: string
    detailHeaderTitleStyle?: StyleValue
    detailHeaderPreviousTitle?: string
    detailHeaderPreviousTextStyle?: StyleValue
    detailHeaderCurrentTextStyle?: StyleValue
    chromeInteractive?: boolean
    isFeedRoute?: boolean
    feedTabs?: FeedTab[]
    activeKey?: string | symbol | null
    feedTabsLayerStyle?: StyleValue
    feedTabsTargetLayerStyle?: StyleValue
    viewSwipeTargetKey?: string | null
    feedPullActive?: boolean
    pullStatusStyle?: StyleValue
    pullRefreshing?: boolean
    pullIconStyle?: StyleValue
    pullStatusText?: string
    pullStatusMeta?: string
  }>(),
  {
    detailReaderOpen: false,
    detailHeaderLayerStyle: undefined,
    detailTitle: '',
    detailHeaderTitleStyle: undefined,
    detailHeaderPreviousTitle: '',
    detailHeaderPreviousTextStyle: undefined,
    detailHeaderCurrentTextStyle: undefined,
    chromeInteractive: true,
    isFeedRoute: false,
    feedTabs: () => [],
    activeKey: null,
    feedTabsLayerStyle: undefined,
    feedTabsTargetLayerStyle: undefined,
    viewSwipeTargetKey: null,
    feedPullActive: false,
    pullStatusStyle: undefined,
    pullRefreshing: false,
    pullIconStyle: undefined,
    pullStatusText: '',
    pullStatusMeta: '',
  },
)

const emit = defineEmits<{
  (event: 'navigate', path: string): void
}>()
</script>

<template>
  <div class="app-header-feed-stack">
    <div
      v-if="detailReaderOpen"
      class="feed-header-layer feed-header-layer--detail"
      :style="detailHeaderLayerStyle"
      :aria-hidden="chromeInteractive ? undefined : 'true'"
    >
      <div v-if="detailTitle" class="detail-header-title" :style="detailHeaderTitleStyle">
        <span
          v-if="detailHeaderPreviousTitle"
          class="detail-header-title__text detail-header-title__text--previous"
          :style="detailHeaderPreviousTextStyle"
          aria-hidden="true"
        >
          {{ detailHeaderPreviousTitle }}
        </span>
        <span class="detail-header-title__text" :style="detailHeaderCurrentTextStyle">
          {{ detailTitle }}
        </span>
      </div>
    </div>
    <div
      v-if="isFeedRoute"
      class="feed-header-layer feed-header-layer--tabs"
      :style="feedTabsLayerStyle"
      :aria-hidden="!chromeInteractive || detailReaderOpen || feedPullActive ? 'true' : undefined"
    >
      <div class="feed-tabs" role="tablist" aria-label="阅读视图">
        <button
          v-for="tab in feedTabs"
          :key="tab.key"
          class="feed-tab"
          :class="{ 'feed-tab--active': activeKey === tab.key }"
          type="button"
          role="tab"
          :aria-selected="activeKey === tab.key"
          :tabindex="!chromeInteractive || detailReaderOpen || feedPullActive ? -1 : undefined"
          @pointerdown.stop
          @click="emit('navigate', tab.path)"
        >
          {{ tab.label }}
        </button>
      </div>
    </div>
    <div
      v-if="isFeedRoute"
      class="feed-header-layer feed-header-layer--tabs feed-header-layer--view-target"
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
    <RefreshStatusLayer
      v-if="isFeedRoute"
      root-class="feed-header-layer feed-header-layer--refresh"
      :hidden="detailReaderOpen || !feedPullActive"
      :root-style="pullStatusStyle"
      :refreshing="pullRefreshing"
      :icon-style="pullIconStyle"
      :title="pullStatusText"
      :meta="pullStatusMeta"
    />
  </div>
</template>
