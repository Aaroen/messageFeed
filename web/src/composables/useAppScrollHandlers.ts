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
  updateTopTabsByScroll: (current: number, previous: number) => void
  scheduleReaderSessionSave: () => void
}

function scrollEventTarget(event: Event) {
  return event.currentTarget as HTMLElement | null
}

export function useAppScrollHandlers(options: AppScrollHandlersOptions) {
  function handleFeedContentScroll(event: Event) {
    const target = scrollEventTarget(event)
    if (!target) {
      return
    }

    const current = target.scrollTop
    const scrollUpdate = options.scrollHistory.update('feed', current)
    options.updateFeedScrollTop(current)
    options.updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
    options.scheduleReaderSessionSave()
  }

  function handlePageContentScroll(event: Event) {
    const target = scrollEventTarget(event)
    if (!target) {
      return
    }

    const current = target.scrollTop
    const scrollUpdate = options.scrollHistory.update('page', current)
    options.updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
    options.scheduleReaderSessionSave()
  }

  function handleSourceReaderScroll(event: Event) {
    const target = scrollEventTarget(event)
    if (!target) {
      return
    }

    const current = target.scrollTop
    const scrollUpdate = options.scrollHistory.update('source', current)
    options.updateSourceReaderScrollTop(current)
    options.updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
    options.scheduleReaderSessionSave()
  }

  function handleDetailContentScroll(event: Event) {
    const target = scrollEventTarget(event)
    if (!target) {
      return
    }

    const current = target.scrollTop
    const scrollUpdate = options.scrollHistory.update('detail', current)
    options.updateDetailScrollMetrics(current, target.scrollHeight, target.clientHeight)
    options.updateTopTabsByScroll(scrollUpdate.current, scrollUpdate.previous)
    options.scheduleReaderSessionSave()
  }

  return {
    handleFeedContentScroll,
    handlePageContentScroll,
    handleSourceReaderScroll,
    handleDetailContentScroll,
  }
}
