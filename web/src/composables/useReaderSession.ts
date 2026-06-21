import { nextTick, ref } from 'vue'

import type { FeedItem } from '@/api/feed'

export type FeedSourceKind = 'subscriptions' | 'recommendations'

export type ReaderSource = {
  id: number
  name: string
  kind: FeedSourceKind
}

export type RectSnapshot = {
  left: number
  top: number
  width: number
  height: number
}

export type ParkedDetailSnapshot = {
  item: FeedItem
  sourceKind: FeedSourceKind
  originRect: RectSnapshot | null
  sourceItemTargetRect: RectSnapshot | null
  sourceNameOriginRect: RectSnapshot | null
  sourceNameTargetRect: RectSnapshot | null
  morphingItemHeight: number | null
  scrollTop: number
}

export type ReaderSessionSnapshot = {
  savedAt: number
  routeFullPath: string
  feedScrollTop: number
  sourceReaderScrollTop: number
  detailScrollTop: number
  topChromeProgress: number
  feedContentCollapsed: boolean
  readerSource: ReaderSource | null
  sourceReaderVisible: boolean
  detailItem: FeedItem | null
  detailSourceKind: FeedSourceKind
  detailOpenedFromSourceReader: boolean
  detailListReturnCommitted: boolean
  detailSourceExitProgress: number
  sourceReaderReturnMode: 'detail' | null
  sourceReaderBackDetail: ParkedDetailSnapshot | null
  morphingItemHeight: number | null
  parkedDetailStack: ParkedDetailSnapshot[]
}

type ReaderSessionOptions<TSnapshot extends { savedAt?: number; routeFullPath?: string }> = {
  storageKey: string
  maxAgeMS: number
  saveDelayMS?: number
  createSnapshot: () => TSnapshot
  getCurrentRouteFullPath: () => string
  matchesCurrentRoute?: (snapshotRouteFullPath: string) => boolean
  restoreSnapshot: (snapshot: TSnapshot) => Promise<void> | void
  afterRestore?: () => void
}

export function useReaderSession<TSnapshot extends { savedAt?: number; routeFullPath?: string }>(
  options: ReaderSessionOptions<TSnapshot>,
) {
  const restoring = ref(false)
  let saveTimer = 0
  let sessionToken = 0

  function clearSaveTimer() {
    if (typeof window !== 'undefined' && saveTimer !== 0) {
      window.clearTimeout(saveTimer)
    }
    saveTimer = 0
  }

  function removeSavedSnapshot() {
    if (typeof window === 'undefined') {
      return
    }
    window.sessionStorage.removeItem(options.storageKey)
  }

  function readSavedSnapshot() {
    if (typeof window === 'undefined') {
      return null
    }

    const raw = window.sessionStorage.getItem(options.storageKey)
    if (!raw) {
      return null
    }

    let snapshot: TSnapshot
    try {
      snapshot = JSON.parse(raw) as TSnapshot
    } catch {
      removeSavedSnapshot()
      return null
    }

    if (!snapshot.savedAt || Date.now() - snapshot.savedAt > options.maxAgeMS) {
      removeSavedSnapshot()
      return null
    }

    return snapshot
  }

  function saveNow() {
    if (restoring.value || typeof window === 'undefined') {
      return
    }

    window.sessionStorage.setItem(options.storageKey, JSON.stringify(options.createSnapshot()))
  }

  function scheduleSave() {
    if (restoring.value || typeof window === 'undefined') {
      return
    }

    sessionToken += 1
    const token = sessionToken
    clearSaveTimer()
    saveTimer = window.setTimeout(() => {
      saveTimer = 0
      if (token !== sessionToken) {
        return
      }
      saveNow()
    }, options.saveDelayMS ?? 80)
  }

  async function restore() {
    const snapshot = readSavedSnapshot()
    if (!snapshot) {
      return
    }

    sessionToken += 1
    const token = sessionToken
    clearSaveTimer()
    restoring.value = true
    try {
      const matchesCurrentRoute = snapshot.routeFullPath
        ? options.matchesCurrentRoute?.(snapshot.routeFullPath) ??
          options.getCurrentRouteFullPath() === snapshot.routeFullPath
        : true
      if (!matchesCurrentRoute) {
        removeSavedSnapshot()
        return
      }

      await options.restoreSnapshot(snapshot)
    } finally {
      nextTick(() => {
        if (token !== sessionToken) {
          return
        }
        restoring.value = false
        scheduleSave()
        options.afterRestore?.()
      })
    }
  }

  function clearTimer() {
    sessionToken += 1
    clearSaveTimer()
    restoring.value = false
  }

  return {
    restoring,
    saveNow,
    scheduleSave,
    restore,
    clearTimer,
    removeSavedSnapshot,
  }
}
