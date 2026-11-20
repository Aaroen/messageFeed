<script setup lang="ts">
import { useId, type StyleValue } from 'vue'

const titleID = useId()

withDefaults(
  defineProps<{
    sourceName?: string
    sourceMeta?: string
    titleTextStyle?: StyleValue
    titleLayerStyle?: StyleValue
    mainLayerStyle?: StyleValue
    mainHidden?: boolean
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
    mainHidden: false,
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
      :aria-hidden="mainHidden ? 'true' : undefined"
    >
      <div class="reader-overlay__title" :style="titleLayerStyle">
        <button class="reader-source-name" type="button" :popovertarget="titleID" :tabindex="mainHidden ? -1 : undefined" :style="titleTextStyle">
          {{ sourceName }}
        </button>
        <small>{{ sourceMeta }}</small>
      </div>
      <button
        class="sources-button reader-source-toggle"
        :class="{ 'sources-button--active': toggleActive }"
        type="button"
        :disabled="toggleDisabled"
        :tabindex="mainHidden ? -1 : undefined"
        @pointerdown.stop
        @touchstart.stop
        @click="emit('toggle-subscription')"
      >
        {{ toggleLabel }}
      </button>
    </div>
  </div>
  <div :id="titleID" class="reader-source-name-popover" popover>{{ sourceName }}</div>
</template>
