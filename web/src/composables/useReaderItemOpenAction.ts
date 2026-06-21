import { nextTick } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind, ReaderSource, RectSnapshot } from '@/composables/useReaderSession'
import { snapshotRect } from '@/utils/domSnapshot'

type ReadableRef<T> = {
  readonly value: T
}

type OpenItemReaderTransitionOptions = {
  openedFromSourceReader: boolean
  originRect: RectSnapshot | null
  headerSwapDelay: number
  detailEntryDelay: number
  afterBegin?: () => void
  afterEntry?: () => void
}

type ReaderItemOpenActionOptions = {
  sourceReaderOpen: ReadableRef<boolean>
  readerSource: ReadableRef<ReaderSource | null>
  sourceTimelinePreloadEnabled: ReadableRef<boolean>
  headerSwapDuration: number
  detailEntryDuration: number
  resolveDelay: (duration: number) => number
  openItemReaderWithTransition: (
    item: FeedItem,
    sourceKind: FeedSourceKind,
    options: OpenItemReaderTransitionOptions,
  ) => void
  openSourceReader: (source: ReaderSource, options?: { visible?: boolean }) => void
  loadFeedItem: (itemID: number) => Promise<FeedItem>
  finishOpenItemReaderLoad: (options?: { item?: FeedItem; errorMessage?: string }) => void
  setChromeStableVisible: () => void
  finishFeedTopPull: () => void
  rememberDetailScrollTop: (scrollTop: number) => void
  captureDetailSourceTransitionRects: (retry?: number, options?: { force?: boolean; lock?: boolean }) => void
  scrollDetailContentElementTo: (scrollTop: number) => void
  scheduleReaderSessionSave: () => void
}

export function useReaderItemOpenAction(options: ReaderItemOpenActionOptions) {
  async function openItemReader(item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) {
    const openedFromSourceReader =
      options.sourceReaderOpen.value &&
      options.readerSource.value?.id === item.source_id &&
      options.readerSource.value.kind === sourceKind

    options.openItemReaderWithTransition(item, sourceKind, {
      openedFromSourceReader,
      originRect: snapshotRect(originRect),
      headerSwapDelay: options.resolveDelay(options.headerSwapDuration),
      detailEntryDelay: options.resolveDelay(options.detailEntryDuration),
      afterBegin: () => {
        options.setChromeStableVisible()
        options.finishFeedTopPull()
        options.rememberDetailScrollTop(0)
      },
      afterEntry: () => {
        if (openedFromSourceReader) {
          options.captureDetailSourceTransitionRects(12, { lock: true })
        }
      },
    })

    if (!openedFromSourceReader && options.sourceTimelinePreloadEnabled.value && item.source_id) {
      options.openSourceReader(
        {
          id: item.source_id,
          name: item.source_name || '未知来源',
          kind: sourceKind,
        },
        { visible: false },
      )
    }

    try {
      let loadedItem: FeedItem | undefined
      if (sourceKind === 'subscriptions' && item.id > 0) {
        loadedItem = await options.loadFeedItem(item.id)
      }
      options.finishOpenItemReaderLoad({ item: loadedItem })
    } catch {
      options.finishOpenItemReaderLoad({ errorMessage: '无法加载完整条目，已显示当前列表内容。' })
    } finally {
      nextTick(() => {
        options.scrollDetailContentElementTo(0)
        options.scheduleReaderSessionSave()
      })
    }
  }

  return {
    openItemReader,
  }
}
