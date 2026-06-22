<script setup lang="ts">
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import ReaderDetailOverlayContent from '@/components/ReaderDetailOverlayContent.vue'

withDefaults(
  defineProps<{
    detailTransitionStyle?: StyleValue
    detailItem?: FeedItem | null
    detailMorphVisible?: boolean
    detailMorphTextStyle?: StyleValue
    detailMorphSourceLabelStyle?: StyleValue
    detailDisplayDate?: string
    detailMorphSummaryVisible?: boolean
    detailPreviewSummary?: string
    detailContentStyle?: StyleValue
    detailInlineMetaStyle?: StyleValue
    detailFrameStyle?: StyleValue
    detailActionsStyle?: StyleValue
    detailLoading?: boolean
    detailError?: string
    detailSrcdoc?: string
    detailInlineSourceStyle?: StyleValue
    detailProgressVisible?: boolean
    detailReadingProgress?: number
    detailProgressStyle?: StyleValue
    detailProgressFillStyle?: StyleValue
    detailProgressThumbStyle?: StyleValue
  }>(),
  {
    detailTransitionStyle: undefined,
    detailItem: null,
    detailMorphVisible: false,
    detailMorphTextStyle: undefined,
    detailMorphSourceLabelStyle: undefined,
    detailDisplayDate: '',
    detailMorphSummaryVisible: false,
    detailPreviewSummary: '',
    detailContentStyle: undefined,
    detailInlineMetaStyle: undefined,
    detailFrameStyle: undefined,
    detailActionsStyle: undefined,
    detailLoading: false,
    detailError: '',
    detailSrcdoc: '',
    detailInlineSourceStyle: undefined,
    detailProgressVisible: false,
    detailReadingProgress: 0,
    detailProgressStyle: undefined,
    detailProgressFillStyle: undefined,
    detailProgressThumbStyle: undefined,
  },
)

const emit = defineEmits<{
  (event: 'detail-content-ref', element: HTMLElement | null): void
  (event: 'detail-content-scroll', value: Event): void
  (event: 'detail-inline-source-ref', element: HTMLElement | null): void
  (event: 'detail-frame-ref', element: HTMLIFrameElement | null): void
  (event: 'detail-frame-load'): void
  (event: 'detail-progress-drag-start'): void
  (event: 'detail-progress-drag-end'): void
  (event: 'detail-progress-change', progress: number): void
}>()
</script>

<template>
  <ReaderDetailOverlayContent
    :transition-style="detailTransitionStyle"
    :item="detailItem"
    :morph-visible="detailMorphVisible"
    :morph-text-style="detailMorphTextStyle"
    :morph-source-label-style="detailMorphSourceLabelStyle"
    :display-date="detailDisplayDate"
    :morph-summary-visible="detailMorphSummaryVisible"
    :preview-summary="detailPreviewSummary"
    :content-style="detailContentStyle"
    :inline-meta-style="detailInlineMetaStyle"
    :frame-style="detailFrameStyle"
    :actions-style="detailActionsStyle"
    :loading="detailLoading"
    :error="detailError"
    :srcdoc="detailSrcdoc"
    :inline-source-style="detailInlineSourceStyle"
    :progress-visible="detailProgressVisible"
    :reading-progress="detailReadingProgress"
    :progress-style="detailProgressStyle"
    :progress-fill-style="detailProgressFillStyle"
    :progress-thumb-style="detailProgressThumbStyle"
    @content-ref="(element) => emit('detail-content-ref', element)"
    @content-scroll="(event) => emit('detail-content-scroll', event)"
    @inline-source-ref="(element) => emit('detail-inline-source-ref', element)"
    @frame-ref="(element) => emit('detail-frame-ref', element)"
    @frame-load="emit('detail-frame-load')"
    @progress-drag-start="emit('detail-progress-drag-start')"
    @progress-drag-end="emit('detail-progress-drag-end')"
    @progress-change="(progress) => emit('detail-progress-change', progress)"
  />
</template>
