import { nextTick } from 'vue'
import type { LocationQueryRaw, RouteLocationNormalizedLoaded, Router } from 'vue-router'

import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

const readerURLQueryKeys = ['item', 'itemKind', 'itemCatalog', 'source', 'sourceKind'] as const

type ReaderRouteSyncOptions = {
  route: RouteLocationNormalizedLoaded
  router: Router
  canSync: () => boolean
  getReaderSource: () => ReaderSource | null
  isSourceReaderOpen: () => boolean
  getDetailItemID: () => number | null | undefined
  getDetailCatalogID: () => number | undefined
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
  let releaseTimer = 0
  let syncToken = 0
  let historySyncToken = 0
  let pendingSync = false
  let pendingForcePush = false
  let activeHistoryForcePush = false

  function clearReleaseTimer() {
    if (typeof window !== 'undefined' && releaseTimer !== 0) {
      window.clearTimeout(releaseTimer)
    }
    releaseTimer = 0
  }

  function clearTimer() {
    syncToken += 1
    historySyncToken += 1
    pendingSync = false
    pendingForcePush = false
    activeHistoryForcePush = false
    clearReleaseTimer()
    syncing = false
    options.setProgrammaticRouteNavigation(false)
  }

  function queuePendingSync(forcePush: boolean) {
    pendingSync = true
    pendingForcePush = pendingForcePush || forcePush || activeHistoryForcePush
    historySyncToken += 1
  }

  function consumePendingSync() {
    if (!pendingSync) {
      return null
    }

    const forcePush = pendingForcePush
    pendingSync = false
    pendingForcePush = false
    return forcePush
  }

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
      const catalogID = options.getDetailCatalogID()
      if (catalogID) query.itemCatalog = String(catalogID)
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

    syncToken += 1
    const token = syncToken
    clearReleaseTimer()
    syncing = true
    options.setProgrammaticRouteNavigation(true)
    try {
      const target = { name: options.route.name, query, hash: options.route.hash }
      const isPublicFeed = options.route.name === 'subscriptions' || options.route.name === 'recommendations'
      const wasReading = Boolean(options.route.query.item || options.route.query.source)
      const willRead = Boolean(query.item || query.source)
      if (isPublicFeed && !wasReading && willRead) {
        await options.router.push({ ...target, state: { messagefeedReaderOrigin: options.route.fullPath } })
      } else if (isPublicFeed && wasReading && !willRead &&
        window.history.state?.messagefeedReaderOrigin === options.router.resolve(target).fullPath &&
        window.history.state.back === window.history.state.messagefeedReaderOrigin) {
        options.router.back()
      } else {
        await options.router.replace(target)
      }
    } finally {
      clearReleaseTimer()
      releaseTimer = window.setTimeout(() => {
        releaseTimer = 0
        if (token !== syncToken) {
          return
        }
        syncing = false
        options.setProgrammaticRouteNavigation(false)
        const forcePendingSync = consumePendingSync()
        activeHistoryForcePush = false
        if (forcePendingSync !== null) {
          scheduleSync(forcePendingSync)
        }
      }, 0)
    }
  }

  function scheduleSync(forcePush = false) {
    if (syncing) {
      queuePendingSync(forcePush)
      return
    }

    historySyncToken += 1
    const token = historySyncToken
    activeHistoryForcePush = forcePush
    void syncURLToState().finally(() => {
      nextTick(() => {
        if (token !== historySyncToken) {
          return
        }
        const historyForcePush = forcePush || activeHistoryForcePush
        activeHistoryForcePush = false
        options.syncVirtualHistoryState(historyForcePush)
      })
    })
  }

  return {
    syncURLToState,
    scheduleSync,
    clearTimer,
  }
}
