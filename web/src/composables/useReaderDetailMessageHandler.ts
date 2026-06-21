type ReadableRef<T> = {
  readonly value: T
}

type ViewportOffset = {
  left: number
  top: number
}

type DetailGesturePayload = {
  phase?: 'start' | 'move' | 'end' | 'cancel'
  source?: string
  startX?: number
  startY?: number
  x?: number
  dx?: number
  dy?: number
}

type DetailScrollPayload = {
  scrollTop?: number
  scrollHeight?: number
  clientHeight?: number
}

type ReaderDetailMessageHandlerOptions = {
  detailReaderOpen: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
  readerBackSwipeTrackingActive: ReadableRef<boolean>
  detailCommittedListReturn: () => boolean
  isCurrentDetailFrameMessageSource: (source: MessageEventSource | null) => boolean
  updateDetailFrameContentHeight: (scrollHeight: number) => void
  syncDetailContainerMetrics: () => void
  detailFrameViewportOffset: () => ViewportOffset
  beginDetailGestureCandidate: (startX: number, startY: number) => void
  updateBackSwipe: (
    deltaX: number,
    deltaY: number,
    fromDetailFrame: boolean,
    currentX: number,
  ) => boolean
  finishBackSwipe: (deltaX: number, deltaY: number) => void
  cancelBackSwipe: () => void
  resetGestureTracking: () => void
}

export function useReaderDetailMessageHandler(options: ReaderDetailMessageHandlerOptions) {
  let metricsFrame = 0

  function clearMetricsFrame() {
    if (typeof window !== 'undefined' && metricsFrame !== 0) {
      window.cancelAnimationFrame(metricsFrame)
    }
    metricsFrame = 0
  }

  function handleDetailScrollMessage(data: DetailScrollPayload) {
    const scrollHeight = Number(data.scrollHeight ?? 0)
    clearMetricsFrame()
    metricsFrame = window.requestAnimationFrame(() => {
      metricsFrame = 0
      if (!options.detailReaderOpen.value || options.detailCommittedListReturn()) {
        return
      }
      if (Number.isFinite(scrollHeight)) {
        options.updateDetailFrameContentHeight(scrollHeight)
      }
      options.syncDetailContainerMetrics()
    })
  }

  function handleDetailGestureMessage(payload: DetailGesturePayload) {
    if (options.navigationVisible.value) {
      return
    }

    const fromDetailFrame = payload.source === 'detail-frame'
    const frameOffset = fromDetailFrame ? options.detailFrameViewportOffset() : { left: 0, top: 0 }
    const startX = Number(payload.startX ?? 0) + frameOffset.left
    const startY = Number(payload.startY ?? 0) + frameOffset.top
    const deltaX = Number(payload.dx ?? 0)
    const deltaY = Number(payload.dy ?? 0)
    const currentX = Number(payload.x ?? Number(payload.startX ?? 0) + deltaX) + frameOffset.left

    if (payload.phase === 'start') {
      options.beginDetailGestureCandidate(startX, startY)
      return
    }

    if (payload.phase === 'move') {
      options.updateBackSwipe(deltaX, deltaY, fromDetailFrame, currentX)
      return
    }

    if (payload.phase === 'end') {
      if (options.readerBackSwipeTrackingActive.value) {
        options.finishBackSwipe(deltaX, deltaY)
        options.resetGestureTracking()
        return
      }
      options.resetGestureTracking()
      return
    }

    if (payload.phase === 'cancel') {
      if (options.readerBackSwipeTrackingActive.value) {
        options.cancelBackSwipe()
      }
      options.resetGestureTracking()
    }
  }

  function handleMessage(event: MessageEvent) {
    if (options.detailCommittedListReturn()) {
      return
    }

    if (!options.isCurrentDetailFrameMessageSource(event.source)) {
      return
    }

    if (event.data?.type === 'messagefeed-detail-scroll' && options.detailReaderOpen.value) {
      handleDetailScrollMessage(event.data as DetailScrollPayload)
      return
    }

    if (event.data?.type !== 'messagefeed-detail-gesture' || !options.detailReaderOpen.value) {
      return
    }

    handleDetailGestureMessage(event.data as DetailGesturePayload)
  }

  return {
    handleMessage,
    clearMetricsFrame,
  }
}
