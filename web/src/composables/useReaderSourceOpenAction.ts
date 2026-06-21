import { nextTick } from 'vue'

import type { ReaderSource } from '@/composables/useReaderSession'

type OpenSourceReaderOptions = {
  visible?: boolean
}

type OpenSourceReaderStateResult = {
  nextVisible: boolean
  sourceChanged: boolean
  resetScroll: boolean
  captureTransition: boolean
  loadSubscription: boolean
}

type ReaderSourceOpenActionOptions = {
  openSourceReaderState: (
    source: ReaderSource,
    options: OpenSourceReaderOptions,
  ) => OpenSourceReaderStateResult
  getReaderSource: () => ReaderSource | null
  clearHiddenSourceCleanupTimer: () => void
  setTopChromeVisible: (visible: boolean) => void
  captureDetailSourceTransitionRects: (retry?: number, options?: { force?: boolean; lock?: boolean }) => void
  loadSourceReaderSubscription: (source: ReaderSource) => Promise<unknown> | unknown
  resetSourceSubscriptionState: () => void
  rememberSourceScrollTop: (scrollTop: number) => void
  scrollSourceReaderContentElementTo: (scrollTop: number) => void
}

export function useReaderSourceOpenAction(options: ReaderSourceOpenActionOptions) {
  let sourceOpenToken = 0

  function readerSourceMatches(source: ReaderSource) {
    const currentSource = options.getReaderSource()
    return currentSource?.id === source.id && currentSource.kind === source.kind
  }

  function sourceOpenIsCurrent(token: number, source: ReaderSource) {
    return token === sourceOpenToken && readerSourceMatches(source)
  }

  function openSourceReader(source: ReaderSource, actionOptions: OpenSourceReaderOptions = {}) {
    sourceOpenToken += 1
    const token = sourceOpenToken

    options.clearHiddenSourceCleanupTimer()
    const nextVisible = actionOptions.visible ?? true
    if (nextVisible) {
      options.setTopChromeVisible(true)
    }

    const result = options.openSourceReaderState(source, { visible: nextVisible })
    if (!result.sourceChanged) {
      if (result.captureTransition) {
        options.captureDetailSourceTransitionRects(12, { lock: true })
      }
      if (result.loadSubscription) {
        void options.loadSourceReaderSubscription(source)
      }
      return
    }

    options.resetSourceSubscriptionState()
    if (result.resetScroll) {
      options.rememberSourceScrollTop(0)
    }
    nextTick(() => {
      if (!sourceOpenIsCurrent(token, source)) {
        return
      }
      if (result.resetScroll) {
        options.scrollSourceReaderContentElementTo(0)
      }
      if (result.captureTransition) {
        options.captureDetailSourceTransitionRects(12, { lock: true })
      }
    })
    if (result.loadSubscription) {
      void options.loadSourceReaderSubscription(source)
    }
  }

  return {
    openSourceReader,
  }
}
