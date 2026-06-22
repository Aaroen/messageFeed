<script setup lang="ts">
import type { StyleValue } from 'vue'

import RefreshStatusLayer from '@/components/RefreshStatusLayer.vue'

withDefaults(
  defineProps<{
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
</script>

<template>
  <div class="app-header-page-stack">
    <div
      class="feed-header-layer feed-header-layer--tabs"
      :style="pageTitleLayerStyle"
    >
      <h1>{{ pageTitle }}</h1>
    </div>
    <RefreshStatusLayer
      root-class="feed-header-layer feed-header-layer--refresh"
      :hidden="!pagePullActive"
      :root-style="pagePullStatusStyle"
      :refreshing="pagePullRefreshing"
      :icon-style="pagePullIconStyle"
      :title="pagePullStatusText"
      :meta="pagePullStatusMeta"
    />
  </div>
</template>
