import type { ComponentPublicInstance } from 'vue'

import type { PageViewExpose } from '@/composables/usePageOutletState'

type ReadableRef<T> = {
  readonly value: T
}

type AppElementRefsOptions = {
  detailFrameRef: ReadableRef<HTMLIFrameElement | null>
  setSourceReaderContentElement: (element: HTMLElement | null) => void
  setFeedContentElement: (element: HTMLElement | null) => void
  setPageContentElement: (element: HTMLElement | null) => void
  setPageViewInstance: (view: PageViewExpose | null) => void
  setDetailContentElement: (element: HTMLElement | null) => void
  setDetailInlineSourceElement: (element: HTMLElement | null) => void
  setDetailFrameElement: (element: HTMLIFrameElement | null) => void
}

export function useAppElementRefs(options: AppElementRefsOptions) {
  function setSourceReaderContentElement(element: HTMLElement | null) {
    options.setSourceReaderContentElement(element)
  }

  function setFeedContentElement(element: Element | ComponentPublicInstance | null) {
    options.setFeedContentElement(element instanceof HTMLElement ? element : null)
  }

  function setPageContentElement(element: HTMLElement | null) {
    options.setPageContentElement(element)
  }

  function setPageViewInstance(view: PageViewExpose | null) {
    options.setPageViewInstance(view)
  }

  function detailFrameViewportOffset() {
    const rect = options.detailFrameRef.value?.getBoundingClientRect()
    return {
      left: rect?.left ?? 0,
      top: rect?.top ?? 0,
    }
  }

  function setDetailContentElement(element: HTMLElement | null) {
    options.setDetailContentElement(element)
  }

  function setDetailInlineSourceElement(element: HTMLElement | null) {
    options.setDetailInlineSourceElement(element)
  }

  function setDetailFrameElement(element: HTMLIFrameElement | null) {
    options.setDetailFrameElement(element)
  }

  return {
    setSourceReaderContentElement,
    setFeedContentElement,
    setPageContentElement,
    setPageViewInstance,
    detailFrameViewportOffset,
    setDetailContentElement,
    setDetailInlineSourceElement,
    setDetailFrameElement,
  }
}
