import { nextTick } from 'vue'
import type { LocationQueryRaw, RouteLocationNormalizedLoaded, Router } from 'vue-router'

import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

const readerURLQueryKeys = ['item', 'itemKind', 'source', 'sourceKind'] as const

type ReaderRouteSyncOptions = {
  route: RouteLocationNormalizedLoaded
  router: Router
  canSync: () => boolean
  getReaderSource: () => ReaderSource | null
  isSourceReaderOpen: () => boolean
  getDetailItemID: () => number | null | undefined
  getDetailSourceKind: () => FeedSourceKind
  setProgrammaticRouteNavigation: (active: boolean) => void
  syncVirtualHistoryState: (forcePush?: boolean) => void
}

export function browserRouteFullPath() {
  if (typeof window === 'undefined') {
    return ''
  }
  return `${window.location.pathname}${window.location.search}${window.location.hash}`
}

export function normalizeReaderRouteFullPath(fullPath: string) {
  const url = new URL(fullPath, 'https://messagefeed.local')
  for (const key of readerURLQueryKeys) {
    url.searchParams.delete(key)
  }
  const query = url.searchParams.toString()
  return `${url.pathname}${query ? `?${query}` : ''}${url.hash}`
}

export function readerRouteMatchesCurrent(currentRoutes: string[], snapshotRouteFullPath: string) {
  return currentRoutes
    .filter(Boolean)
    .some(
      (currentRoute) =>
        currentRoute === snapshotRouteFullPath ||
        normalizeReaderRouteFullPath(currentRoute) === normalizeReaderRouteFullPath(snapshotRouteFullPath),
    )
}

function readerQueryValue(value: unknown) {
  if (Array.isArray(value)) {
    return value.map((item) => String(item ?? '')).join('\u0001')
  }
  return value == null ? '' : String(value)
}

function readerQueriesEqual(left: Record<string, unknown>, right: Record<string, unknown>) {
  const keys = new Set([...Object.keys(left), ...Object.keys(right)])
  for (const key of keys) {
    if (readerQueryValue(left[key]) !== readerQueryValue(right[key])) {
      return false
    }
  }
  return true
}

export function useReaderRouteSync(options: ReaderRouteSyncOptions) {
  let syncing = false

  function readerQueryBase() {
    const query: LocationQueryRaw = { ...options.route.query }
    for (const key of readerURLQueryKeys) {
      delete query[key]
    }
    return query
  }

  function readerURLQuery() {
    const query = readerQueryBase()
    const readerSource = options.getReaderSource()
    if (options.isSourceReaderOpen() && readerSource) {
      query.source = String(readerSource.id)
      query.sourceKind = readerSource.kind
    }

    const detailItemID = options.getDetailItemID()
    if (detailItemID) {
      query.item = String(detailItemID)
      query.itemKind = options.getDetailSourceKind()
    }
    return query
  }

  async function syncURLToState() {
    if (!options.canSync() || syncing || !options.route.name) {
      return
    }

    const query = readerURLQuery()
    if (readerQueriesEqual(options.route.query, query)) {
      return
    }

    syncing = true
    options.setProgrammaticRouteNavigation(true)
    try {
      await options.router.replace({ name: options.route.name, query })
    } finally {
      window.setTimeout(() => {
        syncing = false
        options.setProgrammaticRouteNavigation(false)
      }, 0)
    }
  }

  function scheduleSync(forcePush = false) {
    void syncURLToState().finally(() => {
      nextTick(() => options.syncVirtualHistoryState(forcePush))
    })
  }

  return {
    syncURLToState,
    scheduleSync,
  }
}
