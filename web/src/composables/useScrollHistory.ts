export type ScrollHistorySurface = 'feed' | 'page' | 'source' | 'detail'

function normalizeScrollTop(value: number | null | undefined) {
  return typeof value === 'number' && Number.isFinite(value) ? Math.max(0, value) : 0
}

export function useScrollHistory() {
  const scrollTopBySurface: Record<ScrollHistorySurface, number> = {
    feed: 0,
    page: 0,
    source: 0,
    detail: 0,
  }

  function set(surface: ScrollHistorySurface, scrollTop: number | null | undefined) {
    const normalized = normalizeScrollTop(scrollTop)
    scrollTopBySurface[surface] = normalized
    return normalized
  }

  function update(surface: ScrollHistorySurface, scrollTop: number | null | undefined) {
    const previous = scrollTopBySurface[surface]
    const current = set(surface, scrollTop)
    return { current, previous }
  }

  return {
    set,
    update,
  }
}
