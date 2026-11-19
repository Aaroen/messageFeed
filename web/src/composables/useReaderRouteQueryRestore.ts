import { ref } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

import {
  getFeedItem,
  listRecommendationItems,
  listSourceCatalog,
  listSources,
  listTimelineItems,
  type FeedItem,
} from '@/api/feed'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

const routeFeedLookupLimit = 100
const routeFeedLookupMaxItems = 500

type ReaderRouteQueryRestoreOptions = {
  route: RouteLocationNormalizedLoaded
  openSourceReader: (source: ReaderSource) => void
  openItemReader: (item: FeedItem, sourceKind: FeedSourceKind) => Promise<void> | void
  restoreReaderSession: () => Promise<void> | void
}

function routeQueryText(value: unknown) {
  const nextValue = Array.isArray(value) ? value[0] : value
  return typeof nextValue === 'string' ? nextValue.trim() : ''
}

function routeQueryID(value: unknown) {
  const parsed = Number.parseInt(routeQueryText(value), 10)
  return Number.isFinite(parsed) && parsed > 0 ? parsed : 0
}

function routeQuerySourceKind(value: unknown, fallback: FeedSourceKind): FeedSourceKind {
  const text = routeQueryText(value)
  if (text === 'subscriptions' || text === 'recommendations') {
    return text
  }
  return fallback
}

function readerSourceFromItem(item: FeedItem, kind: FeedSourceKind): ReaderSource | null {
  if (!item.source_id) {
    return null
  }
  return {
    id: item.source_id,
    name: item.source_name || '未知来源',
    kind,
  }
}

function routeFeedKindCandidates(kind: FeedSourceKind): FeedSourceKind[] {
  return kind === 'subscriptions'
    ? ['subscriptions', 'recommendations']
    : ['recommendations', 'subscriptions']
}

async function listRouteFeedPage(kind: FeedSourceKind, sourceID: number, offset: number) {
  const params = {
    limit: routeFeedLookupLimit,
    offset,
    order: 'desc' as const,
    ...(sourceID ? { source_id: sourceID } : {}),
  }
  return kind === 'subscriptions' ? listTimelineItems(params) : listRecommendationItems(params)
}

async function findRouteFeedItemInKind(itemID: number, kind: FeedSourceKind, sourceID: number) {
  let offset = 0
  while (offset < routeFeedLookupMaxItems) {
    const result = await listRouteFeedPage(kind, sourceID, offset)
    const item = result.items.find((entry) => entry.id === itemID)
    if (item) {
      return item
    }
    if (result.items.length < routeFeedLookupLimit || offset + result.items.length >= result.total) {
      return null
    }
    offset += routeFeedLookupLimit
  }
  return null
}

async function resolveRouteFeedItem(itemID: number, preferredKind: FeedSourceKind, sourceID: number) {
  try {
    return {
      item: await getFeedItem(itemID),
      kind: preferredKind,
    }
  } catch {
    // Some recommendation entries are only available through feed list endpoints.
  }

  for (const kind of routeFeedKindCandidates(preferredKind)) {
    try {
      const item = await findRouteFeedItemInKind(itemID, kind, sourceID)
      if (item) {
        return { item, kind }
      }
    } catch {
      continue
    }
  }
  return null
}

async function resolveRouteReaderSource(sourceID: number, kind: FeedSourceKind, item: FeedItem | null) {
  const itemSource = item && item.source_id === sourceID ? readerSourceFromItem(item, kind) : null
  if (itemSource) {
    return itemSource
  }

  try {
    const [sources, catalogResult] = await Promise.all([
      listSources(),
      listSourceCatalog({ limit: 200, offset: 0 }),
    ])
    const source = sources.find((entry) => entry.id === sourceID)
    const catalogEntry =
      catalogResult.entries.find((entry) => entry.id === sourceID) ??
      catalogResult.entries.find((entry) => entry.source_id === sourceID)
    return {
      id: sourceID,
      name: source?.name || catalogEntry?.name || '未知来源',
      kind,
    }
  } catch {
    return {
      id: sourceID,
      name: '未知来源',
      kind,
    }
  }
}

export function useReaderRouteQueryRestore(options: ReaderRouteQueryRestoreOptions) {
  const restoring = ref(false)
  let restoreSequence = 0

  function feedSourceKindFromRoute(): FeedSourceKind {
    return options.route.name === 'subscriptions' ? 'subscriptions' : 'recommendations'
  }

  async function restoreReaderRouteQueryState() {
    const sequence = ++restoreSequence
    const fullPath = options.route.fullPath
    const itemID = routeQueryID(options.route.query.item)
    const sourceID = routeQueryID(options.route.query.source)
    if (!itemID && !sourceID) {
      restoring.value = false
      return false
    }

    restoring.value = true
    try {
      const preferredItemKind = routeQuerySourceKind(options.route.query.itemKind, feedSourceKindFromRoute())
      const result = itemID ? await resolveRouteFeedItem(itemID, preferredItemKind, sourceID) : null
      const itemKind = result?.kind ?? preferredItemKind
      const sourceKind = routeQuerySourceKind(options.route.query.sourceKind, itemKind)
      const source = sourceID ? await resolveRouteReaderSource(sourceID, sourceKind, result?.item ?? null) : null
      if (sequence !== restoreSequence || options.route.fullPath !== fullPath) return false

      if (source) options.openSourceReader(source)
      if (result) await options.openItemReader(result.item, itemKind)
      return Boolean(source || result)
    } finally {
      if (sequence === restoreSequence) restoring.value = false
    }
  }

  async function restoreReaderStateOnLoad() {
    if (await restoreReaderRouteQueryState()) {
      return
    }
    await options.restoreReaderSession()
  }

  return {
    restoring,
    restoreReaderRouteQueryState,
    restoreReaderStateOnLoad,
  }
}
