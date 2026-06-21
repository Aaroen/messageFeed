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

withDefaults(
  defineProps<{
    phase?: ChromePhase
    progress?: number
    rootClass?: ClassValue
    rootStyle?: StyleValue
    feedHeaderActive?: boolean
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
    pageTitle?: string
    pagePullActive?: boolean
    pageTitleLayerStyle?: StyleValue
    pagePullStatusStyle?: StyleValue
    pagePullRefreshing?: boolean
    pagePullIconStyle?: StyleValue
    pagePullStatusText?: string
    pagePullStatusMeta?: string
  }>(),
  {
    phase: 'visible',
    progress: 1,
    rootClass: '',
    rootStyle: undefined,
    feedHeaderActive: false,
    detailReaderOpen: false,
    detailHeaderVisible: false,
    detailHeaderLayerStyle: undefined,
    detailTitle: '',
    detailHeaderTitleStyle: undefined,
    detailHeaderPreviousTitle: '',
    detailHeaderPreviousTextStyle: undefined,
    detailHeaderCurrentTextStyle: undefined,
    isFeedRoute: false,
    feedTabs: () => [],
    activeKey: null,
    feedTabsLayerHidden: true,
    feedTabsLayerStyle: undefined,
    viewSwipeTargetVisible: false,
    feedTabsTargetLayerStyle: undefined,
    viewSwipeTargetKey: null,
    feedPullActive: false,
    pullStatusStyle: undefined,
    pullRefreshing: false,
    pullIconStyle: undefined,
    pullStatusText: '',
    pullStatusMeta: '',
    pageTitle: '',
    pagePullActive: false,
    pageTitleLayerStyle: undefined,
    pagePullStatusStyle: undefined,
    pagePullRefreshing: false,
    pagePullIconStyle: undefined,
    pagePullStatusText: '',
    pagePullStatusMeta: '',
  },
)

const emit = defineEmits<{
  (event: 'navigate', path: string): void
}>()
</script>

<template>
  <TopChrome
    variant="app"
    :phase="phase"
    :progress="progress"
    :root-class="rootClass"
    :root-style="rootStyle"
  >
    <div class="app-header-slot" :class="{ 'app-header-slot--feed': feedHeaderActive }">
      <AppFeedHeaderContent
        v-if="feedHeaderActive"
        :detail-reader-open="detailReaderOpen"
        :detail-header-visible="detailHeaderVisible"
        :detail-header-layer-style="detailHeaderLayerStyle"
        :detail-title="detailTitle"
        :detail-header-title-style="detailHeaderTitleStyle"
        :detail-header-previous-title="detailHeaderPreviousTitle"
        :detail-header-previous-text-style="detailHeaderPreviousTextStyle"
        :detail-header-current-text-style="detailHeaderCurrentTextStyle"
        :is-feed-route="isFeedRoute"
        :feed-tabs="feedTabs"
        :active-key="activeKey"
        :feed-tabs-layer-hidden="feedTabsLayerHidden"
        :feed-tabs-layer-style="feedTabsLayerStyle"
        :view-swipe-target-visible="viewSwipeTargetVisible"
        :feed-tabs-target-layer-style="feedTabsTargetLayerStyle"
        :view-swipe-target-key="viewSwipeTargetKey"
        :feed-pull-active="feedPullActive"
        :pull-status-style="pullStatusStyle"
        :pull-refreshing="pullRefreshing"
        :pull-icon-style="pullIconStyle"
        :pull-status-text="pullStatusText"
        :pull-status-meta="pullStatusMeta"
        @navigate="(path) => emit('navigate', path)"
      />
      <AppPageHeaderContent
        v-else
        :page-title="pageTitle"
        :page-pull-active="pagePullActive"
        :page-title-layer-style="pageTitleLayerStyle"
        :page-pull-status-style="pagePullStatusStyle"
        :page-pull-refreshing="pagePullRefreshing"
        :page-pull-icon-style="pagePullIconStyle"
        :page-pull-status-text="pagePullStatusText"
        :page-pull-status-meta="pagePullStatusMeta"
      />
    </div>
  </TopChrome>
</template>
