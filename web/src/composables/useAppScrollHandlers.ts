type ScrollHistorySurface = 'feed' | 'page' | 'source' | 'detail'

type ScrollChromeContext = {
  topRangeHasContent: boolean
}

type ScrollHistoryController = {
  update: (
    surface: ScrollHistorySurface,
    scrollTop: number | null | undefined,
  ) => { current: number; previous: number }
}

type AppScrollHandlersOptions = {
  scrollHistory: ScrollHistoryController
  updateFeedScrollTop: (scrollTop: number) => void
  updateSourceReaderScrollTop: (scrollTop: number) => void
  updateDetailScrollMetrics: (scrollTop: number, scrollHeight: number, clientHeight: number) => void
  updateTopTabsByScroll: (
    surface: ScrollHistorySurface,
    current: number,
    previous: number,
    context: ScrollChromeContext,
  ) => void
  scheduleReaderSessionSave: () => void
}

function scrollEventTarget(event: Event) {
  return event.currentTarget as HTMLElement | null
}

const topRangeContentSelector = [
  '.feed-item',
  '.empty-surface',
  '.feed-alert',
  '.source-row',
  '.sources-toolbar',
  '.sources-panel',
].join(',')

function pxValue(value: string | null) {
  const parsed = Number.parseFloat(value ?? '')
  return Number.isFinite(parsed) ? parsed : 0
}

function topRangeHeight(target: HTMLElement) {
  const style = getComputedStyle(target)
  return (
    pxValue(style.getPropertyValue('--feed-header-height')) ||
    pxValue(style.getPropertyValue('--feed-header-space')) ||
    86
  )
}

function elementIntersectsTopRange(element: Element, top: number, bottom: number) {
  const rect = element.getBoundingClientRect()
  if (rect.width <= 0 || rect.height <= 0) {
    return false
  }
  return rect.top < bottom && rect.bottom > top
}

function topRangeHasContent(target: HTMLElement) {
  const targetRect = target.getBoundingClientRect()
  const rangeTop = Math.max(targetRect.top, 0)
  const rangeBottom = Math.min(targetRect.bottom, rangeTop + topRangeHeight(target))
  if (rangeBottom <= rangeTop) {
    return false
  }

  return Array.from(target.querySelectorAll(topRangeContentSelector)).some((element) =>
    elementIntersectsTopRange(element, rangeTop, rangeBottom),
  )
}

export function useAppScrollHandlers(options: AppScrollHandlersOptions) {
  function syncSurfaceScroll(surface: ScrollHistorySurface, target: HTMLElement | null) {
    if (!target) {
      return
    }

    const current = target.scrollTop
    const scrollUpdate = options.scrollHistory.update(surface, current)
    if (surface === 'feed') {
      options.updateFeedScrollTop(current)
    } else if (surface === 'source') {
      options.updateSourceReaderScrollTop(current)
    } else if (surface === 'detail') {
      options.updateDetailScrollMetrics(current, target.scrollHeight, target.clientHeight)
    }
    options.updateTopTabsByScroll(surface, scrollUpdate.current, scrollUpdate.previous, {
      topRangeHasContent: topRangeHasContent(target),
    })
    options.scheduleReaderSessionSave()
  }

  function handleFeedContentScroll(event: Event) {
    syncSurfaceScroll('feed', scrollEventTarget(event))
  }

  function handlePageContentScroll(event: Event) {
    syncSurfaceScroll('page', scrollEventTarget(event))
  }

  function handleSourceReaderScroll(event: Event) {
    syncSurfaceScroll('source', scrollEventTarget(event))
  }

  function handleDetailContentScroll(event: Event) {
    syncSurfaceScroll('detail', scrollEventTarget(event))
  }

  return {
    syncSurfaceScroll,
    handleFeedContentScroll,
    handlePageContentScroll,
    handleSourceReaderScroll,
    handleDetailContentScroll,
  }
}
