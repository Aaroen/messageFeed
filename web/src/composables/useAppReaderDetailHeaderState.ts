import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppReaderMotionState } from '@/composables/useAppReaderMotionState'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderDetailHeaderStateOptions = {
  chromeVisible: ReadableRef<boolean>
  readerOpen: ReadableRef<boolean>
  visible: ReadableRef<boolean>
  layerStyle: ReadableRef<StyleValue>
  item: ReadableRef<FeedItem | null>
  readerMotion: AppReaderMotionState
  previousTitle: ReadableRef<string>
}

export type AppReaderDetailHeaderState = {
  chromeVisible: ReadableRef<boolean>
  readerOpen: ReadableRef<boolean>
  visible: ReadableRef<boolean>
  layerStyle: ReadableRef<StyleValue>
  title: ReadableRef<string>
  titleStyle: ReadableRef<StyleValue>
  previousTitle: ReadableRef<string>
  previousTextStyle: ReadableRef<StyleValue>
  currentTextStyle: ReadableRef<StyleValue>
}

export function useAppReaderDetailHeaderState(
  options: AppReaderDetailHeaderStateOptions,
): AppReaderDetailHeaderState {
  const readerMotion = options.readerMotion
  const title = computed(() => options.item.value?.title || '')

  return {
    chromeVisible: options.chromeVisible,
    readerOpen: options.readerOpen,
    visible: options.visible,
    layerStyle: options.layerStyle,
    title,
    titleStyle: readerMotion.detailHeaderTitleStyle,
    previousTitle: options.previousTitle,
    previousTextStyle: readerMotion.detailHeaderPreviousTextStyle,
    currentTextStyle: readerMotion.detailHeaderCurrentTextStyle,
  }
}
