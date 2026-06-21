import type { FeedItem } from '@/api/feed'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderSourceRevealActionOptions = {
  detailItem: ReadableRef<FeedItem | null>
  detailSourceKind: ReadableRef<FeedSourceKind>
  readerSource: ReadableRef<ReaderSource | null>
  openSourceReader: (source: ReaderSource, options?: { visible?: boolean }) => void
  setTopChromeVisible: (visible: boolean) => void
  revealSourceReaderUnderDetailState: () => void
  captureDetailSourceTransitionRects: (retry?: number, options?: { force?: boolean; lock?: boolean }) => void
}

export function useReaderSourceRevealAction(options: ReaderSourceRevealActionOptions) {
  function showSourceReaderUnderDetail() {
    const item = options.detailItem.value
    if (!item?.source_id) {
      return
    }

    const source = {
      id: item.source_id,
      name: item.source_name || '未知来源',
      kind: options.detailSourceKind.value,
    }

    if (options.readerSource.value?.id !== source.id || options.readerSource.value.kind !== source.kind) {
      options.openSourceReader(source, { visible: false })
    }

    options.setTopChromeVisible(true)
    options.revealSourceReaderUnderDetailState()
    options.captureDetailSourceTransitionRects(12, { lock: true })
  }

  return {
    showSourceReaderUnderDetail,
  }
}
