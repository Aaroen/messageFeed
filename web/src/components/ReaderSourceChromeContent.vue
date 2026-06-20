<script setup lang="ts">
import type { StyleValue } from 'vue'
import { IconSync } from '@arco-design/web-vue/es/icon'

withDefaults(
  defineProps<{
    sourceName?: string
    sourceMeta?: string
    titleTextStyle?: StyleValue
    titleLayerStyle?: StyleValue
    mainLayerStyle?: StyleValue
    pullStatusStyle?: StyleValue
    pullIconStyle?: StyleValue
    pullActive?: boolean
    pullRefreshing?: boolean
    pullStatusText?: string
    pullStatusMeta?: string
    toggleActive?: boolean
    toggleLabel?: string
    toggleDisabled?: boolean
  }>(),
  {
    sourceName: '',
    sourceMeta: '',
    titleTextStyle: undefined,
    titleLayerStyle: undefined,
    mainLayerStyle: undefined,
    pullStatusStyle: undefined,
    pullIconStyle: undefined,
    pullActive: false,
    pullRefreshing: false,
    pullStatusText: '',
    pullStatusMeta: '',
    toggleActive: false,
    toggleLabel: '',
    toggleDisabled: false,
  },
)

const emit = defineEmits<{
  (event: 'toggle-subscription'): void
}>()
</script>

<template>
  <div class="reader-overlay__source-stack">
    <div
      class="reader-source-layer"
      :class="{ 'reader-source-layer--hidden': pullActive }"
      :style="mainLayerStyle"
    >
      <div class="reader-overlay__title" :style="titleLayerStyle">
        <span :style="titleTextStyle">{{ sourceName }}</span>
        <small>{{ sourceMeta }}</small>
      </div>
      <button
        class="sources-button reader-source-toggle"
        :class="{ 'sources-button--active': toggleActive }"
        type="button"
        :disabled="toggleDisabled"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('toggle-subscription')"
      >
        {{ toggleLabel }}
      </button>
    </div>
    <div
      class="reader-source-layer reader-source-layer--refresh"
      :class="{ 'reader-source-layer--hidden': !pullActive }"
      :style="pullStatusStyle"
      aria-live="polite"
    >
      <span
        class="feed-refresh-header__icon"
        :class="{ 'feed-refresh-header__icon--refreshing': pullRefreshing }"
        :style="pullIconStyle"
      >
        <IconSync />
      </span>
      <div class="feed-refresh-header__copy">
        <div class="feed-refresh-header__title">{{ pullStatusText }}</div>
        <div class="feed-refresh-header__meta">{{ pullStatusMeta }}</div>
      </div>
    </div>
  </div>
</template>
