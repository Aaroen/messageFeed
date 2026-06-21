<script setup lang="ts">
import type { StyleValue } from 'vue'

import AppFeedHeaderContent from '@/components/AppFeedHeaderContent.vue'
import AppPageHeaderContent from '@/components/AppPageHeaderContent.vue'
import TopChrome from '@/components/TopChrome.vue'
import type { ChromePhase } from '@/composables/useChromeState'

type ClassValue = string | Record<string, boolean> | Array<string | Record<string, boolean>>

type FeedTab = {
  key: string
  label: string
  path: string
}

type TopChromeChromeProps = {
  phase?: ChromePhase
  progress?: number
  rootClass?: ClassValue
  rootStyle?: StyleValue
}

type TopChromeFeedProps = {
  active?: boolean
  detailReaderOpen?: boolean
  detailHeaderVisible?: boolean
  detailHeaderLayerStyle?: StyleValue
  detailTitle?: string
  detailHeaderTitleStyle?: StyleValue
  detailHeaderPreviousTitle?: string
  detailHeaderPreviousTextStyle?: StyleValue
  detailHeaderCurrentTextStyle?: StyleValue
  isFeedRoute?: boolean
  feedTabs?: FeedTab[]
  activeKey?: string | symbol | null
  feedTabsLayerHidden?: boolean
  feedTabsLayerStyle?: StyleValue
  viewSwipeTargetVisible?: boolean
  feedTabsTargetLayerStyle?: StyleValue
  viewSwipeTargetKey?: string | null
  feedPullActive?: boolean
  pullStatusStyle?: StyleValue
  pullRefreshing?: boolean
  pullIconStyle?: StyleValue
  pullStatusText?: string
  pullStatusMeta?: string
}

type TopChromePageProps = {
  title?: string
  pullActive?: boolean
  titleLayerStyle?: StyleValue
  pullStatusStyle?: StyleValue
  pullRefreshing?: boolean
  pullIconStyle?: StyleValue
  pullStatusText?: string
  pullStatusMeta?: string
}

withDefaults(
  defineProps<{
    chrome?: TopChromeChromeProps
    feed?: TopChromeFeedProps
    page?: TopChromePageProps
  }>(),
  {
    chrome: () => ({}),
    feed: () => ({}),
    page: () => ({}),
  },
)

const emit = defineEmits<{
  (event: 'navigate', path: string): void
}>()
</script>

<template>
  <TopChrome
    variant="app"
    :phase="chrome.phase"
    :progress="chrome.progress"
    :root-class="chrome.rootClass"
    :root-style="chrome.rootStyle"
  >
    <div class="app-header-slot" :class="{ 'app-header-slot--feed': feed.active }">
      <AppFeedHeaderContent
        v-if="feed.active"
        :detail-reader-open="feed.detailReaderOpen"
        :detail-header-visible="feed.detailHeaderVisible"
        :detail-header-layer-style="feed.detailHeaderLayerStyle"
        :detail-title="feed.detailTitle"
        :detail-header-title-style="feed.detailHeaderTitleStyle"
        :detail-header-previous-title="feed.detailHeaderPreviousTitle"
        :detail-header-previous-text-style="feed.detailHeaderPreviousTextStyle"
        :detail-header-current-text-style="feed.detailHeaderCurrentTextStyle"
        :is-feed-route="feed.isFeedRoute"
        :feed-tabs="feed.feedTabs"
        :active-key="feed.activeKey"
        :feed-tabs-layer-hidden="feed.feedTabsLayerHidden"
        :feed-tabs-layer-style="feed.feedTabsLayerStyle"
        :view-swipe-target-visible="feed.viewSwipeTargetVisible"
        :feed-tabs-target-layer-style="feed.feedTabsTargetLayerStyle"
        :view-swipe-target-key="feed.viewSwipeTargetKey"
        :feed-pull-active="feed.feedPullActive"
        :pull-status-style="feed.pullStatusStyle"
        :pull-refreshing="feed.pullRefreshing"
        :pull-icon-style="feed.pullIconStyle"
        :pull-status-text="feed.pullStatusText"
        :pull-status-meta="feed.pullStatusMeta"
        @navigate="(path) => emit('navigate', path)"
      />
      <AppPageHeaderContent
        v-else
        :page-title="page.title"
        :page-pull-active="page.pullActive"
        :page-title-layer-style="page.titleLayerStyle"
        :page-pull-status-style="page.pullStatusStyle"
        :page-pull-refreshing="page.pullRefreshing"
        :page-pull-icon-style="page.pullIconStyle"
        :page-pull-status-text="page.pullStatusText"
        :page-pull-status-meta="page.pullStatusMeta"
      />
    </div>
  </TopChrome>
</template>
