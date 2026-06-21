import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderMorphVisibilityStateOptions = {
  readerSource: ReadableRef<ReaderSource | null>
  sourceToggleActive: ReadableRef<boolean>
  sourceTitleRevealVisible: ReadableRef<boolean>
  sourceTitleRevealStyle: ReadableRef<StyleValue>
  detailItem: ReadableRef<FeedItem | null>
  sourceNameMorphVisible: ReadableRef<boolean>
  sourceNameMorphStyle: ReadableRef<StyleValue>
  detailMorphTextVisible: ReadableRef<boolean>
  detailMorphTextStyle: ReadableRef<StyleValue>
  detailMorphSourceLabelStyle: ReadableRef<StyleValue>
  detailDisplayDate: ReadableRef<string>
  detailMorphSummaryVisible: ReadableRef<boolean>
  detailPreviewSummary: ReadableRef<string>
}

export type AppReaderMorphVisibilityState = {
  sourceTitleRevealMounted: ReadableRef<boolean>
  sourceTitleRevealVisible: ReadableRef<boolean>
  sourceTitleRevealStyle: ReadableRef<StyleValue>
  sourceTitle: ReadableRef<string>
  sourceMeta: ReadableRef<string>
  sourceNameMorphMounted: ReadableRef<boolean>
  sourceNameMorphVisible: ReadableRef<boolean>
  sourceNameMorphStyle: ReadableRef<StyleValue>
  sourceNameMorphText: ReadableRef<string>
  detailMorphVisible: ReadableRef<boolean>
  detailMorphTextStyle: ReadableRef<StyleValue>
  detailMorphSourceLabelStyle: ReadableRef<StyleValue>
  detailDisplayDate: ReadableRef<string>
  detailMorphSummaryVisible: ReadableRef<boolean>
  detailPreviewSummary: ReadableRef<string>
}

export function useAppReaderMorphVisibilityState(
  options: AppReaderMorphVisibilityStateOptions,
): AppReaderMorphVisibilityState {
  const sourceTitle = computed(() => options.readerSource.value?.name || '')
  const sourceMeta = computed(() => (options.sourceToggleActive.value ? '已订阅' : '未订阅'))
  const sourceTitleRevealMounted = computed(
    () => Boolean(options.readerSource.value) && options.sourceTitleRevealVisible.value,
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
    sourceTitleRevealVisible: options.sourceTitleRevealVisible,
    sourceTitleRevealStyle: options.sourceTitleRevealStyle,
    sourceTitle,
    sourceMeta,
    sourceNameMorphMounted,
    sourceNameMorphVisible: options.sourceNameMorphVisible,
    sourceNameMorphStyle: options.sourceNameMorphStyle,
    sourceNameMorphText,
    detailMorphVisible: options.detailMorphTextVisible,
    detailMorphTextStyle: options.detailMorphTextStyle,
    detailMorphSourceLabelStyle: options.detailMorphSourceLabelStyle,
    detailDisplayDate: options.detailDisplayDate,
    detailMorphSummaryVisible: options.detailMorphSummaryVisible,
    detailPreviewSummary: options.detailPreviewSummary,
  }
}
