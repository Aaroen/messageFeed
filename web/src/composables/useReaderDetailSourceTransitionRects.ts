import { nextTick } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { RectSnapshot } from '@/composables/useReaderSession'
import { snapshotElementRect } from '@/utils/domSnapshot'

type ReadableRef<T> = {
  readonly value: T
}

type ApplyDetailSourceTransitionRectsOptions = {
  itemRect: RectSnapshot | null
  sourceOriginRect: RectSnapshot | null
  sourceTargetRect: RectSnapshot | null
  lock?: boolean
}

type ApplyDetailSourceTransitionRectsResult = {
  locked: boolean
}

type ReaderDetailSourceTransitionRectsOptions = {
  sourceReaderContentRef: ReadableRef<HTMLElement | null>
  detailInlineSourceRef: ReadableRef<HTMLElement | null>
  detailItem: ReadableRef<FeedItem | null>
  detailFeedOriginLocked: ReadableRef<boolean>
  detailTransitionRectsLocked: ReadableRef<boolean>
  retryDelay: number
  findFeedItemElement: (itemID?: number) => Element | null
  applyDetailFeedOriginRectState: (itemRect: RectSnapshot, lock?: boolean) => void
  applyDetailSourceTransitionRectsState: (
    options: ApplyDetailSourceTransitionRectsOptions,
  ) => ApplyDetailSourceTransitionRectsResult
  applyVisibleSourceReturnTargetState: (
    itemRect: RectSnapshot | null,
    sourceOriginRect: RectSnapshot | null,
    sourceTargetRect: RectSnapshot | null,
  ) => boolean
}

export function useReaderDetailSourceTransitionRects(options: ReaderDetailSourceTransitionRectsOptions) {
  let captureFrame = 0
  let captureRetryTimer = 0
  let captureToken = 0

  function clearCaptureHandles() {
    if (typeof window !== 'undefined' && captureFrame !== 0) {
      window.cancelAnimationFrame(captureFrame)
    }
    if (typeof window !== 'undefined' && captureRetryTimer !== 0) {
      window.clearTimeout(captureRetryTimer)
    }
    captureFrame = 0
    captureRetryTimer = 0
  }

  function clearDetailSourceTransitionRectCapture() {
    captureToken += 1
    clearCaptureHandles()
  }

  function findSourceFeedItemElement(itemID?: number) {
    if (!itemID || !options.sourceReaderContentRef.value) {
      return null
    }
    return options.sourceReaderContentRef.value.querySelector(`[data-feed-item-id="${itemID}"]`)
  }

  function findSourceFeedItemSourceElement(itemID?: number) {
    const itemElement = findSourceFeedItemElement(itemID)
    return itemElement?.querySelector('.feed-item__source') ?? null
  }

  function sourceNameTargetFallback(itemRect: RectSnapshot | null) {
    if (itemRect) {
      const left = itemRect.left + 16
      const top = itemRect.top + 16
      return {
        left,
        top,
        width: Math.max(1, Math.min(itemRect.width - 32, 180)),
        height: 18,
      }
    }

    return null
  }

  function refreshDetailFeedOriginRect(lock = false) {
    if (options.detailFeedOriginLocked.value) {
      return
    }

    const itemRect = snapshotElementRect(options.findFeedItemElement(options.detailItem.value?.id))
    if (!itemRect) {
      return
    }

    options.applyDetailFeedOriginRectState(itemRect, lock)
  }

  function scheduleDetailSourceTransitionRectCapture(
    retry = 12,
    captureOptions: { force?: boolean; lock?: boolean } = {},
    token: number,
  ) {
    nextTick(() => {
      if (token !== captureToken) {
        return
      }

      captureFrame = window.requestAnimationFrame(() => {
        captureFrame = 0
        if (token !== captureToken) {
          return
        }

        if (options.detailTransitionRectsLocked.value && !captureOptions.force) {
          return
        }

        const itemRect = snapshotElementRect(findSourceFeedItemElement(options.detailItem.value?.id))
        const sourceOriginRect = snapshotElementRect(options.detailInlineSourceRef.value)
        const sourceTargetRect =
          snapshotElementRect(findSourceFeedItemSourceElement(options.detailItem.value?.id)) ??
          sourceNameTargetFallback(itemRect)

        const result = options.applyDetailSourceTransitionRectsState({
          itemRect,
          sourceOriginRect,
          sourceTargetRect,
          lock: captureOptions.lock,
        })
        if (result.locked) {
          return
        }

        if (retry > 0 && (!itemRect || !sourceOriginRect || !sourceTargetRect)) {
          captureRetryTimer = window.setTimeout(() => {
            captureRetryTimer = 0
            if (token !== captureToken) {
              return
            }
            scheduleDetailSourceTransitionRectCapture(retry - 1, captureOptions, token)
          }, options.retryDelay)
        }
      })
    })
  }

  function captureDetailSourceTransitionRects(
    retry = 12,
    captureOptions: { force?: boolean; lock?: boolean } = {},
  ) {
    captureToken += 1
    clearCaptureHandles()
    const token = captureToken
    if (options.detailTransitionRectsLocked.value && !captureOptions.force) {
      return
    }

    scheduleDetailSourceTransitionRectCapture(retry, captureOptions, token)
  }

  function captureVisibleSourceReturnTarget() {
    clearDetailSourceTransitionRectCapture()
    const itemRect = snapshotElementRect(findSourceFeedItemElement(options.detailItem.value?.id))
    const sourceTargetRect =
      snapshotElementRect(findSourceFeedItemSourceElement(options.detailItem.value?.id)) ??
      sourceNameTargetFallback(itemRect)
    const sourceOriginRect = snapshotElementRect(options.detailInlineSourceRef.value)

    return options.applyVisibleSourceReturnTargetState(itemRect, sourceOriginRect, sourceTargetRect)
  }

  return {
    refreshDetailFeedOriginRect,
    captureDetailSourceTransitionRects,
    captureVisibleSourceReturnTarget,
    clearDetailSourceTransitionRectCapture,
  }
}
