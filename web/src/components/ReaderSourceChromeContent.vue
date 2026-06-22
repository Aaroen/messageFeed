<script setup lang="ts">
import type { StyleValue } from 'vue'

withDefaults(
  defineProps<{
    sourceName?: string
    sourceMeta?: string
    titleTextStyle?: StyleValue
    titleLayerStyle?: StyleValue
    mainLayerStyle?: StyleValue
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
  </div>
</template>
