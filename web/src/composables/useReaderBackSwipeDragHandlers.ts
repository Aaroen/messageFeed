import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderBackSwipeIntentSideEffectState = {
  returningToFeed: boolean
  revealSourceReader: boolean
}

type ReaderBackSwipeIntentOptions = {
  resetSourceExit?: boolean
  prepareBlocked?: boolean
  beforeSourceReturnIntent?: () => void
  afterSourceReturnIntent?: () => void
  beforeDetailBackPrepare?: (state: ReaderBackSwipeIntentSideEffectState) => void
  afterDetailBackPrepare?: (state: ReaderBackSwipeIntentSideEffectState) => void
  afterDetailSourceIntent?: () => void
}

type ReaderBackSwipeVisualOptions = {
  resetPageStretch?: () => void
  resetPageOffset?: () => void
  applyPageStretch?: (stretch: number) => void
}

type ReaderBackSwipeDragMetrics = {
  currentX: number
  startX: number
  width: number
}

type GestureOriginState = {
  begin: (startX: number, startY: number, progress: number) => void
  originX: () => number
}

type ReaderBackSwipeDragHandlersOptions = {
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  navigationProgress: ReadableRef<number>
  sourceTimelinePreloadEnabled: ReadableRef<boolean>
  detailItem: ReadableRef<FeedItem | null>
  readerSource: ReadableRef<ReaderSource | null>
  detailSourceKind: ReadableRef<FeedSourceKind>
  readerBackSwipeCandidateActive: ReadableRef<boolean>
  readerBackSwipeTrackingActive: ReadableRef<boolean>
  windowWidth: ReadableRef<number>
  chromeSettleDuration: number
  resolveDelay: (duration: number) => number
  gestureOrigin: GestureOriginState
  resetGestureTracking: () => void
  beginReaderBackSwipeCandidateState: (target: 'detail') => void
  prepareSourceReaderReturnDragState: (options?: { onDetailScrollTop?: (scrollTop: number) => void }) => boolean
  rememberDetailScrollTop: (scrollTop: number) => void
  captureVisibleSourceReturnTarget: () => boolean
  openSourceReader: (source: ReaderSource, options?: { visible?: boolean }) => void
  beginReaderBackSwipeDragState: (deltaX: number, options?: ReaderBackSwipeIntentOptions) => unknown
  updateReaderBackSwipeDragState: (
    deltaX: number,
    metrics: ReaderBackSwipeDragMetrics,
    options?: {
      intent?: ReaderBackSwipeIntentOptions
      visual?: ReaderBackSwipeVisualOptions
    },
  ) => unknown
  beginBackSwipeTransition: (deltaX: number) => void
  syncBackSwipeTransition: (deltaX: number) => void
  cancelNavigationCandidates: () => void
  cancelViewSwipeCandidate: () => void
  isBackHorizontalSwipe: (deltaX: number, deltaY: number) => boolean
  suppressFollowingClick: () => void
  beginTopChromeGestureReturn: (options: { settleDelayMS: number }) => void
  refreshDetailFeedOriginRect: (lock?: boolean) => void
  captureDetailSourceTransitionRects: (retry?: number, options?: { force?: boolean; lock?: boolean }) => void
  showSourceReaderUnderDetail: () => void
  setPageSideStretch: (stretch: number) => void
  setPageSideOffset: (offset: number) => void
}

export function useReaderBackSwipeDragHandlers(options: ReaderBackSwipeDragHandlersOptions) {
  function showTopChromeForSourceReturn() {
    if (options.topChromeProgress.value < 0.99 || options.feedContentCollapsed.value) {
      options.beginTopChromeGestureReturn({
        settleDelayMS: options.resolveDelay(options.chromeSettleDuration),
      })
    }
  }

  function prepareSourceReaderReturnDrag() {
    const ready = options.prepareSourceReaderReturnDragState({
      onDetailScrollTop: options.rememberDetailScrollTop,
    })
    if (!ready) {
      return false
    }

    return options.captureVisibleSourceReturnTarget()
  }

  function prepareDetailSourceReaderPreload() {
    const item = options.detailItem.value
    if (!item?.source_id || options.readerSource.value) {
      return
    }

    options.openSourceReader(
      {
        id: item.source_id,
        name: item.source_name || '未知来源',
        kind: options.detailSourceKind.value,
      },
      { visible: false },
    )
  }

  function beginDetailGestureCandidate(startX: number, startY: number) {
    options.resetGestureTracking()
    options.gestureOrigin.begin(startX, startY, options.navigationProgress.value)
    options.beginReaderBackSwipeCandidateState('detail')
    if (options.sourceTimelinePreloadEnabled.value) {
      prepareDetailSourceReaderPreload()
    }
  }

  function isDetailFrameHorizontalSwipe(deltaX: number, deltaY: number) {
    return Math.abs(deltaX) > 3 && Math.abs(deltaX) > Math.abs(deltaY) * 0.52
  }

  function readerBackSwipeIntentOptions(
    intentOptions: { resetSourceExit?: boolean; prepareBlocked?: boolean } = {},
  ): ReaderBackSwipeIntentOptions {
    return {
      ...intentOptions,
      beforeSourceReturnIntent: () => {
        prepareSourceReaderReturnDrag()
      },
      afterSourceReturnIntent: () => {
        showTopChromeForSourceReturn()
      },
      beforeDetailBackPrepare: ({ returningToFeed }) => {
        if (!returningToFeed) {
          return
        }
        options.refreshDetailFeedOriginRect(true)
      },
      afterDetailBackPrepare: ({ revealSourceReader }) => {
        if (!revealSourceReader) {
          return
        }
        options.captureDetailSourceTransitionRects(12, { lock: true })
      },
      afterDetailSourceIntent: () => {
        options.showSourceReaderUnderDetail()
      },
    }
  }

  function beginBackSwipeIfAllowed(deltaX: number, deltaY: number, fromDetailFrame = false) {
    const horizontal = fromDetailFrame
      ? isDetailFrameHorizontalSwipe(deltaX, deltaY)
      : options.isBackHorizontalSwipe(deltaX, deltaY)
    if (!options.readerBackSwipeCandidateActive.value || !horizontal) {
      return false
    }

    options.beginReaderBackSwipeDragState(deltaX, readerBackSwipeIntentOptions())
    options.beginBackSwipeTransition(deltaX)
    options.cancelNavigationCandidates()
    options.cancelViewSwipeCandidate()
    return true
  }

  function updateBackSwipe(
    deltaX: number,
    deltaY: number,
    fromDetailFrame = false,
    currentX = options.gestureOrigin.originX() + deltaX,
  ) {
    beginBackSwipeIfAllowed(deltaX, deltaY, fromDetailFrame)

    if (!options.readerBackSwipeTrackingActive.value) {
      return false
    }

    options.suppressFollowingClick()
    options.updateReaderBackSwipeDragState(
      deltaX,
      { currentX, startX: options.gestureOrigin.originX(), width: options.windowWidth.value },
      {
        intent: readerBackSwipeIntentOptions({ resetSourceExit: true, prepareBlocked: true }),
        visual: {
          resetPageStretch: () => {
            options.setPageSideStretch(0)
          },
          resetPageOffset: () => {
            options.setPageSideOffset(0)
          },
          applyPageStretch: (nextStretch: number) => {
            options.setPageSideStretch(nextStretch)
          },
        },
      },
    )

    options.syncBackSwipeTransition(deltaX)
    return true
  }

  return {
    beginDetailGestureCandidate,
    updateBackSwipe,
  }
}
