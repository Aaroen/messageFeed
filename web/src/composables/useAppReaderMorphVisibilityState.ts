import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppReaderMotionState } from '@/composables/useAppReaderMotionState'
import type { ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderMorphVisibilityStateOptions = {
  readerSource: ReadableRef<ReaderSource | null>
  sourceToggleActive: ReadableRef<boolean>
  readerMotion: AppReaderMotionState
  detailItem: ReadableRef<FeedItem | null>
  sourceNameMorphVisible: ReadableRef<boolean>
  detailMorphTextVisible: ReadableRef<boolean>
  detailMorphSummaryVisible: ReadableRef<boolean>
}

export type AppReaderMorphVisibilityState = {
  sourceTitleRevealMounted: ReadableRef<boolean>
  sourceTitleRevealStyle: ReadableRef<StyleValue>
  sourceTitle: ReadableRef<string>
  sourceMeta: ReadableRef<string>
  sourceNameMorphMounted: ReadableRef<boolean>
  sourceNameMorphStyle: ReadableRef<StyleValue>
  sourceNameMorphText: ReadableRef<string>
  detailMorphVisible: ReadableRef<boolean>
  detailMorphTextStyle: ReadableRef<StyleValue>
  detailMorphMetaStyle: ReadableRef<StyleValue>
  detailMorphTitleStyle: ReadableRef<StyleValue>
  detailMorphSummaryStyle: ReadableRef<StyleValue>
  detailMorphSourceLabelStyle: ReadableRef<StyleValue>
  detailDisplayDate: ReadableRef<string>
  detailMorphSummaryVisible: ReadableRef<boolean>
  detailPreviewSummary: ReadableRef<string>
}

export function useAppReaderMorphVisibilityState(
  options: AppReaderMorphVisibilityStateOptions,
): AppReaderMorphVisibilityState {
  const readerMotion = options.readerMotion
  const sourceTitle = computed(() => options.readerSource.value?.name || '')
  const sourceMeta = computed(() => (options.sourceToggleActive.value ? '已订阅' : '未订阅'))
  const sourceTitleRevealMounted = computed(
    () => Boolean(options.readerSource.value) && readerMotion.sourceTitleRevealVisible.value,
  )
  const sourceNameMorphText = computed(() => {
    const itemSourceName = options.detailItem.value?.source_name?.trim()
    const readerSourceName = options.readerSource.value?.name.trim()
    return itemSourceName || readerSourceName || '未知来源'
  })
  const sourceNameMorphMounted = computed(
    () => Boolean(options.detailItem.value) && options.sourceNameMorphVisible.value,
  )

  return {
    sourceTitleRevealMounted,
    sourceTitleRevealStyle: readerMotion.sourceTitleRevealStyle,
    sourceTitle,
    sourceMeta,
    sourceNameMorphMounted,
    sourceNameMorphStyle: readerMotion.sourceNameMorphStyle,
    sourceNameMorphText,
    detailMorphVisible: options.detailMorphTextVisible,
    detailMorphTextStyle: readerMotion.detailMorphTextStyle,
    detailMorphMetaStyle: readerMotion.detailMorphMetaStyle,
    detailMorphTitleStyle: readerMotion.detailMorphTitleStyle,
    detailMorphSummaryStyle: readerMotion.detailMorphSummaryStyle,
    detailMorphSourceLabelStyle: readerMotion.detailMorphSourceLabelStyle,
    detailDisplayDate: readerMotion.detailDisplayDate,
    detailMorphSummaryVisible: options.detailMorphSummaryVisible,
    detailPreviewSummary: readerMotion.detailPreviewSummary,
  }
}
