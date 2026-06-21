<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'

import type { PageViewExpose } from '@/composables/usePageOutletState'
import type { ReaderSource } from '@/composables/useReaderSession'

withDefaults(
  defineProps<{
    innerStyle?: StyleValue
  }>(),
  {
    innerStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'content-ref', element: HTMLElement | null): void
  (event: 'view-ref', view: PageViewExpose | null): void
  (event: 'content-scroll', value: Event): void
  (event: 'touch-start', value: TouchEvent): void
  (event: 'touch-move', value: TouchEvent): void
  (event: 'touch-end', value: TouchEvent): void
  (event: 'touch-cancel', value: TouchEvent): void
  (event: 'open-source', source: ReaderSource): void
}>()

function setContentRef(value: Element | ComponentPublicInstance | null) {
  emit('content-ref', value instanceof HTMLElement ? value : null)
}

function setViewRef(value: Element | ComponentPublicInstance | null) {
  emit('view-ref', (value ?? null) as PageViewExpose | null)
}

function handleOpenSource(source: ReaderSource) {
  emit('open-source', source)
}
</script>

<template>
  <section
    :ref="setContentRef"
    class="app-content app-content--page"
    @scroll.passive="(event) => emit('content-scroll', event)"
    @touchstart.passive="(event) => emit('touch-start', event)"
    @touchmove="(event) => emit('touch-move', event)"
    @touchend.passive="(event) => emit('touch-end', event)"
    @touchcancel.passive="(event) => emit('touch-cancel', event)"
  >
    <div class="page-content-inner" :style="innerStyle">
      <router-view v-slot="{ Component }">
        <component :is="Component" :ref="setViewRef" @open-source="handleOpenSource" />
      </router-view>
    </div>
  </section>
</template>
