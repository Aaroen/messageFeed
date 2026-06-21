import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderDetailHeaderStateOptions = {
  chromeVisible: ReadableRef<boolean>
  readerOpen: ReadableRef<boolean>
  visible: ReadableRef<boolean>
  layerStyle: ReadableRef<StyleValue>
  item: ReadableRef<FeedItem | null>
  titleStyle: ReadableRef<StyleValue>
  previousTitle: ReadableRef<string>
  previousTextStyle: ReadableRef<StyleValue>
  currentTextStyle: ReadableRef<StyleValue>
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
  const title = computed(() => options.item.value?.title || '')

  return {
    chromeVisible: options.chromeVisible,
    readerOpen: options.readerOpen,
    visible: options.visible,
    layerStyle: options.layerStyle,
    title,
    titleStyle: options.titleStyle,
    previousTitle: options.previousTitle,
    previousTextStyle: options.previousTextStyle,
    currentTextStyle: options.currentTextStyle,
  }
}
