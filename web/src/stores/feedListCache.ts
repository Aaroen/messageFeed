import { defineStore } from 'pinia'

import type { FeedItem } from '@/api/feed'

export type FeedListCacheEntry = {
  items: FeedItem[]
  total: number
  nextOffset: number
  reachedEnd: boolean
  lastUpdatedAt: string
  cachedAt: number
}

function cloneItems(items: FeedItem[]) {
  return items.map((item) => ({ ...item }))
}

function cloneEntry(entry: FeedListCacheEntry): FeedListCacheEntry {
  return {
    ...entry,
    items: cloneItems(entry.items),
  }
}

export const useFeedListCacheStore = defineStore('feedListCache', {
  state: () => ({
    entries: {} as Record<string, FeedListCacheEntry>,
  }),
  actions: {
    get(key: string) {
      const entry = this.entries[key]
      return entry ? cloneEntry(entry) : null
    },
    set(key: string, entry: FeedListCacheEntry) {
      this.entries[key] = cloneEntry(entry)
    },
    remove(key: string) {
      delete this.entries[key]
    },
  },
})
