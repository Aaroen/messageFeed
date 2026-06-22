import { clampProgress } from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderDetailProgressHandlersOptions = {
  detailReaderOpen: ReadableRef<boolean>
  detailItemID: ReadableRef<number | null>
  detailContentRef: ReadableRef<HTMLElement | null>
  detailScrollMax: ReadableRef<number>
  detailScrollTop: ReadableRef<number>
  updateDetailScrollMetrics: (scrollTop: number, scrollHeight: number, clientHeight: number) => void
  updateDetailScrollTop: (scrollTop: number) => void
  rememberDetailScrollTop: (scrollTop: number) => void
  scrollDetailContentElementTo: (scrollTop: number) => void
  suppressFollowingClick: () => void
  setDetailProgressDragging: (dragging: boolean) => void
}

export function useReaderDetailProgressHandlers(options: ReaderDetailProgressHandlersOptions) {
  let frame = 0

  function clearFrame() {
    if (typeof window !== 'undefined' && frame !== 0) {
      window.cancelAnimationFrame(frame)
    }
    frame = 0
  }

  function syncDetailContainerMetrics() {
    const container = options.detailContentRef.value
    if (!container) {
      return
    }

    options.updateDetailScrollMetrics(container.scrollTop, container.scrollHeight, container.clientHeight)
  }

  function scrollDetailContentTo(top: number) {
    const container = options.detailContentRef.value
    if (!container) {
      return
    }

    container.scrollTop = Math.max(0, top)
    syncDetailContainerMetrics()
  }

  function handleDetailProgressChange(progress: number) {
    if (options.detailScrollMax.value <= 0) {
      return
    }

    const nextScrollTop = options.detailScrollMax.value * clampProgress(progress)
    options.updateDetailScrollTop(nextScrollTop)
    options.rememberDetailScrollTop(nextScrollTop)
    scrollDetailContentTo(nextScrollTop)
  }

  function handleDetailProgressDragStart() {
    options.suppressFollowingClick()
    options.setDetailProgressDragging(true)
  }

  function handleDetailProgressDragEnd() {
    options.setDetailProgressDragging(false)
  }

  function handleDetailFrameLoad() {
    const itemID = options.detailItemID.value
    clearFrame()
    frame = window.requestAnimationFrame(() => {
      frame = 0
      if (!options.detailReaderOpen.value || itemID !== options.detailItemID.value) {
        return
      }
      syncDetailContainerMetrics()
      if (options.detailScrollTop.value > 0) {
        options.scrollDetailContentElementTo(options.detailScrollTop.value)
      }
    })
  }

  return {
    syncDetailContainerMetrics,
    scrollDetailContentTo,
    handleDetailProgressChange,
    handleDetailProgressDragStart,
    handleDetailProgressDragEnd,
    handleDetailFrameLoad,
    clearFrame,
  }
}
