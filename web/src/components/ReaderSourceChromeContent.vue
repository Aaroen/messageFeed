<script setup lang="ts">
import type { StyleValue } from 'vue'

import RefreshStatusLayer from '@/components/RefreshStatusLayer.vue'

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
    <RefreshStatusLayer
      root-class="reader-source-layer reader-source-layer--refresh"
      hidden-class="reader-source-layer--hidden"
      :hidden="!pullActive"
      :root-style="pullStatusStyle"
      :refreshing="pullRefreshing"
      :icon-style="pullIconStyle"
      :title="pullStatusText"
      :meta="pullStatusMeta"
    />
  </div>
</template>
