<script setup lang="ts">
import type { ComponentPublicInstance, StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import ReaderDetailContentInner from '@/components/ReaderDetailContentInner.vue'
import ReaderDetailMorphText from '@/components/ReaderDetailMorphText.vue'
import ReaderDetailProgress from '@/components/ReaderDetailProgress.vue'
import ReaderDetailTransitionSurface from '@/components/ReaderDetailTransitionSurface.vue'

withDefaults(
  defineProps<{
    transitionStyle?: StyleValue
    item?: FeedItem | null
    morphVisible?: boolean
    morphTextStyle?: StyleValue
    morphSourceLabelStyle?: StyleValue
    displayDate?: string
    morphSummaryVisible?: boolean
    previewSummary?: string
    contentStyle?: StyleValue
    loading?: boolean
    error?: string
    srcdoc?: string
    inlineSourceStyle?: StyleValue
    progressVisible?: boolean
    progressDragging?: boolean
    readingProgress?: number
    progressStyle?: StyleValue
    progressFillStyle?: StyleValue
    progressThumbStyle?: StyleValue
  }>(),
  {
    transitionStyle: undefined,
    item: null,
    morphVisible: false,
    morphTextStyle: undefined,
    morphSourceLabelStyle: undefined,
    displayDate: '',
    morphSummaryVisible: false,
    previewSummary: '',
    contentStyle: undefined,
    loading: false,
    error: '',
    srcdoc: '',
    inlineSourceStyle: undefined,
    progressVisible: false,
    progressDragging: false,
    readingProgress: 0,
    progressStyle: undefined,
    progressFillStyle: undefined,
    progressThumbStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'content-ref', element: HTMLElement | null): void
  (event: 'content-scroll', value: Event): void
  (event: 'inline-source-ref', element: HTMLElement | null): void
  (event: 'frame-ref', element: HTMLIFrameElement | null): void
  (event: 'frame-load'): void
  (event: 'progress-drag-start'): void
  (event: 'progress-drag-end'): void
  (event: 'progress-change', progress: number): void
}>()

function domElement(value: Element | ComponentPublicInstance | null) {
  return value instanceof HTMLElement ? value : null
}

function setContentRef(value: Element | ComponentPublicInstance | null) {
  emit('content-ref', domElement(value))
}
</script>

<template>
  <ReaderDetailTransitionSurface
    :root-style="transitionStyle"
  >
    <ReaderDetailMorphText
      :item="item"
      :visible="morphVisible"
      :root-style="morphTextStyle"
      :source-label-style="morphSourceLabelStyle"
      :display-date="displayDate"
      :summary-visible="morphSummaryVisible"
      :summary="previewSummary"
    />
    <div
      :ref="setContentRef"
      class="reader-overlay__content reader-detail"
      :style="contentStyle"
      @scroll.passive="(event) => emit('content-scroll', event)"
    >
      <ReaderDetailContentInner
        :item="item"
        :loading="loading"
        :error="error"
        :display-date="displayDate"
        :srcdoc="srcdoc"
        :inline-source-style="inlineSourceStyle"
        @inline-source-ref="(element) => emit('inline-source-ref', element)"
        @frame-ref="(element) => emit('frame-ref', element)"
        @frame-load="emit('frame-load')"
      />
    </div>
  </ReaderDetailTransitionSurface>
  <ReaderDetailProgress
    :visible="progressVisible"
    :dragging="progressDragging"
    :progress="readingProgress"
    :root-style="progressStyle"
    :fill-style="progressFillStyle"
    :thumb-style="progressThumbStyle"
    @drag-start="emit('progress-drag-start')"
    @drag-end="emit('progress-drag-end')"
    @progress-change="(progress) => emit('progress-change', progress)"
  />
</template>
