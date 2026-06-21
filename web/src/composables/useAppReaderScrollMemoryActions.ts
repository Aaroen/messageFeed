type ReaderScrollMemorySurface = 'source' | 'detail'

type AppReaderScrollMemoryActionsOptions = {
  rememberScrollTop: (surface: ReaderScrollMemorySurface, scrollTop: number) => void
}

export function useAppReaderScrollMemoryActions(options: AppReaderScrollMemoryActionsOptions) {
  function rememberSourceScrollTop(scrollTop: number) {
    options.rememberScrollTop('source', scrollTop)
  }

  function rememberDetailScrollTop(scrollTop: number) {
    options.rememberScrollTop('detail', scrollTop)
  }

  return {
    rememberSourceScrollTop,
    rememberDetailScrollTop,
  }
}
