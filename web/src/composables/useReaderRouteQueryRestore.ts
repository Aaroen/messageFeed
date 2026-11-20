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
import { getCurrentAuth } from '@/api/auth'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'
import { readerSourceForItem } from '@/composables/useReaderSession'

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
  let generation: string | undefined
  while (offset < routeFeedLookupMaxItems) {
    let result = kind === 'recommendations' && generation
      ? await listRecommendationItems({ source_id: sourceID || undefined, limit: routeFeedLookupLimit, offset, generation })
      : await listRouteFeedPage(kind, sourceID, offset)
    generation = result.generation
    if (kind === 'recommendations' && generation && result.refresh_status === 'running' && sourceID) {
      const deadline = Date.now() + 18000
      while (result.refresh_status === 'running' && Date.now() < deadline) {
        await new Promise(resolve => window.setTimeout(resolve, 850))
        result = await listRecommendationItems({ source_id: sourceID, limit: routeFeedLookupLimit, offset, generation })
      }
    }
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
  const itemSource = readerSourceForItem(item, kind)
  if (itemSource && item && [item.source_id, item.catalog_id, item.subscription_id].includes(sourceID)) {
    return itemSource
  }

  if (kind === 'recommendations') {
    const catalog = await listSourceCatalog({ catalog_id: sourceID, limit: 1 })
    if (catalog.entries[0]) return { id: sourceID, name: catalog.entries[0].name, kind }
  }

  const auth = await getCurrentAuth()
  if (auth.authenticated) {
    const source = (await listSources()).find(entry => entry.id === sourceID)
    if (source) return { id: sourceID, name: source.name, kind: 'subscriptions' as const }
  }

  const page = await listTimelineItems({ source_id: sourceID, limit: 1 })
  return { id: sourceID, name: page.items[0]?.source_name || '未知来源', kind: 'subscriptions' as const }
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
      const itemCatalog = routeQueryID(options.route.query.itemCatalog)
      const result = itemID ? await resolveRouteFeedItem(itemID, preferredItemKind, itemCatalog || sourceID) : null
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
