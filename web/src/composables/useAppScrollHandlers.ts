type ScrollHistorySurface = 'feed' | 'page' | 'source' | 'detail'

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
  updateTopTabsByScroll: (surface: ScrollHistorySurface, current: number, previous: number) => void
  scheduleReaderSessionSave: () => void
}

function scrollEventTarget(event: Event) {
  return event.currentTarget as HTMLElement | null
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
    options.updateTopTabsByScroll(surface, scrollUpdate.current, scrollUpdate.previous)
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
