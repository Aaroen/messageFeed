import { computed, ref } from 'vue'

import type { FeedItem, Source, SourceCatalogEntry } from '@/api/feed'
import type {
  FeedSourceKind,
  ParkedDetailSnapshot,
  ReaderSource,
  RectSnapshot,
} from '@/composables/useReaderSession'

type RestoreParkedDetailOptions = {
  onDetailScrollTop?: (scrollTop: number) => void
}

export function useReaderStackState() {
  const sourceReaderContentRef = ref<HTMLElement | null>(null)
  const detailContentRef = ref<HTMLElement | null>(null)
  const detailFrameRef = ref<HTMLIFrameElement | null>(null)
  const detailInlineSourceRef = ref<HTMLElement | null>(null)
  const sourceTitleTextRef = ref<HTMLElement | null>(null)
  const detailProgressTrackRef = ref<HTMLElement | null>(null)
  const detailProgressBarRef = ref<HTMLElement | null>(null)

  const sourceReaderScrollTop = ref(0)
  const detailReaderTouchOffset = ref(0)
  const detailReaderStretch = ref(0)
  const sourceReaderOffset = ref(0)
  const sourceReaderStretch = ref(0)
  const detailStretchAnchor = ref<'left' | 'right' | null>(null)
  const sourceStretchAnchor = ref<'left' | 'right' | null>(null)
  const readerBackDragging = ref(false)
  const readerMotionSettling = ref(false)

  const readerSource = ref<ReaderSource | null>(null)
  const sourceReaderRefreshNonce = ref(0)
  const sourceReaderVisible = ref(false)
  const detailItem = ref<FeedItem | null>(null)
  const detailLoading = ref(false)
  const detailError = ref('')
  const detailSourceKind = ref<FeedSourceKind>('subscriptions')
  const detailOriginRect = ref<RectSnapshot | null>(null)
  const detailSourceItemTargetRect = ref<RectSnapshot | null>(null)
  const detailSourceNameOriginRect = ref<RectSnapshot | null>(null)
  const detailSourceNameTargetRect = ref<RectSnapshot | null>(null)
  const morphingItemId = ref<number | null>(null)
  const morphingHeightLockItemId = ref<number | null>(null)
  const morphingItemHeight = ref<number | null>(null)
  const detailOpenedFromSourceReader = ref(false)
  const detailEntryProgress = ref(1)
  const detailEntrySettling = ref(false)
  const detailHeaderPreviousTitle = ref('')
  const detailHeaderSwapProgress = ref(1)
  const detailBackExitProgress = ref(0)
  const detailSourceExitProgress = ref(0)
  const detailReturningToFeed = ref(false)
  const detailListReturnCommitted = ref(false)
  const detailRestoringFromSourceReader = ref(false)
  const sourceReaderReturnMode = ref<'detail' | null>(null)
  const detailScrollTop = ref(0)
  const detailScrollHeight = ref(0)
  const detailScrollClientHeight = ref(0)
  const detailFrameContentHeight = ref(0)
  const detailProgressDragging = ref(false)
  const parkedDetailStack = ref<ParkedDetailSnapshot[]>([])
  const sourceReaderBackDetail = ref<ParkedDetailSnapshot | null>(null)
  const sourceCatalogEntry = ref<SourceCatalogEntry | null>(null)
  const sourceSubscription = ref<Source | null>(null)
  const sourceSubscriptionLoading = ref(false)
  const sourceNotice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
  const sourceTimelinePreloadEnabled = ref(false)
  const detailTransitionRectsLocked = ref(false)
  const detailFeedOriginLocked = ref(false)
  const sourceReturnTargetReady = ref(false)

  const sourceReaderMounted = computed(() => readerSource.value !== null)
  const sourceReaderOpen = computed(() => readerSource.value !== null && sourceReaderVisible.value)
  const detailReaderOpen = computed(
    () => detailItem.value !== null || detailLoading.value || detailError.value !== '',
  )

  function detailCommittedListReturn() {
    return detailReaderOpen.value && detailListReturnCommitted.value && !readerBackDragging.value
  }

  function hasDetailParkedBehindSource() {
    return (
      detailReaderOpen.value &&
      sourceReaderVisible.value &&
      detailListReturnCommitted.value &&
      !detailReturningToFeed.value &&
      detailSourceExitProgress.value >= 0.99
    )
  }

  function hasParkedDetailSourceState() {
    return (
      detailReaderOpen.value &&
      sourceReaderVisible.value &&
      detailListReturnCommitted.value &&
      !detailReturningToFeed.value
    )
  }

  function sourceReaderShouldReturnToDetail() {
    if (detailOpenedFromSourceReader.value && !detailCommittedListReturn()) {
      return false
    }

    const hasDetailReturnTarget =
      detailReaderOpen.value ||
      parkedDetailStack.value.length > 0 ||
      sourceReaderBackDetail.value !== null ||
      detailListReturnCommitted.value ||
      detailSourceExitProgress.value > 0.45 ||
      detailRestoringFromSourceReader.value
    const sourceLayerAvailable =
      readerSource.value !== null &&
      (sourceReaderVisible.value ||
        sourceReaderBackDetail.value !== null ||
        detailListReturnCommitted.value ||
        detailSourceExitProgress.value > 0.45)
    return (
      sourceReaderReturnMode.value === 'detail' &&
      sourceLayerAvailable &&
      !detailReturningToFeed.value &&
      hasDetailReturnTarget
    )
  }

  function snapshotCurrentDetail(): ParkedDetailSnapshot | null {
    if (!detailItem.value) {
      return null
    }

    return {
      item: { ...detailItem.value },
      sourceKind: detailSourceKind.value,
      originRect: detailOriginRect.value ? { ...detailOriginRect.value } : null,
      sourceItemTargetRect: detailSourceItemTargetRect.value ? { ...detailSourceItemTargetRect.value } : null,
      sourceNameOriginRect: detailSourceNameOriginRect.value ? { ...detailSourceNameOriginRect.value } : null,
      sourceNameTargetRect: detailSourceNameTargetRect.value ? { ...detailSourceNameTargetRect.value } : null,
      morphingItemHeight: morphingItemHeight.value,
      scrollTop: detailScrollTop.value,
    }
  }

  function snapshotParkedDetail(): ParkedDetailSnapshot | null {
    const canSnapshotForSourceReturn =
      sourceReaderReturnMode.value === 'detail' &&
      sourceReaderVisible.value &&
      !detailReturningToFeed.value
    if (!hasParkedDetailSourceState() && !canSnapshotForSourceReturn) {
      return null
    }

    return snapshotCurrentDetail()
  }

  function pushParkedDetailSnapshot() {
    const snapshot = snapshotParkedDetail()
    if (!snapshot) {
      return false
    }

    parkedDetailStack.value.push(snapshot)
    return true
  }

  function restoreParkedDetailSnapshot(
    snapshot: ParkedDetailSnapshot | null,
    options: RestoreParkedDetailOptions = {},
  ) {
    if (!snapshot) {
      return false
    }

    detailItem.value = snapshot.item
    detailError.value = ''
    detailLoading.value = false
    detailSourceKind.value = snapshot.sourceKind
    detailOpenedFromSourceReader.value = false
    detailOriginRect.value = snapshot.originRect
    detailSourceItemTargetRect.value = snapshot.sourceItemTargetRect
    detailSourceNameOriginRect.value = snapshot.sourceNameOriginRect
    detailSourceNameTargetRect.value = snapshot.sourceNameTargetRect
    detailScrollTop.value = snapshot.scrollTop
    options.onDetailScrollTop?.(snapshot.scrollTop)
    detailFrameContentHeight.value = 0
    morphingItemId.value = null
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = snapshot.morphingItemHeight
    detailEntryProgress.value = 1
    detailEntrySettling.value = false
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 1
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = true
    detailRestoringFromSourceReader.value = false
    sourceReaderReturnMode.value = 'detail'
    sourceReaderVisible.value = true
    return true
  }

  function restorePreviousParkedDetail(options: RestoreParkedDetailOptions = {}) {
    return restoreParkedDetailSnapshot(parkedDetailStack.value.pop() ?? null, options)
  }

  function resetDetailTransition() {
    detailEntryProgress.value = 1
    detailEntrySettling.value = false
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
    detailRestoringFromSourceReader.value = false
    sourceReaderReturnMode.value = null
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
    detailFeedOriginLocked.value = false
    sourceReturnTargetReady.value = false
  }

  function clearHiddenSourceReader() {
    if (sourceReaderVisible.value) {
      return false
    }

    sourceReaderOffset.value = 0
    sourceReaderStretch.value = 0
    readerSource.value = null
    sourceCatalogEntry.value = null
    sourceSubscription.value = null
    sourceNotice.value = null
    return true
  }

  function detailBlocksGestures() {
    return detailReaderOpen.value && !detailCommittedListReturn()
  }

  return {
    sourceReaderContentRef,
    detailContentRef,
    detailFrameRef,
    detailInlineSourceRef,
    sourceTitleTextRef,
    detailProgressTrackRef,
    detailProgressBarRef,
    sourceReaderScrollTop,
    detailReaderTouchOffset,
    detailReaderStretch,
    sourceReaderOffset,
    sourceReaderStretch,
    detailStretchAnchor,
    sourceStretchAnchor,
    readerBackDragging,
    readerMotionSettling,
    readerSource,
    sourceReaderRefreshNonce,
    sourceReaderVisible,
    detailItem,
    detailLoading,
    detailError,
    detailSourceKind,
    detailOriginRect,
    detailSourceItemTargetRect,
    detailSourceNameOriginRect,
    detailSourceNameTargetRect,
    morphingItemId,
    morphingHeightLockItemId,
    morphingItemHeight,
    detailOpenedFromSourceReader,
    detailEntryProgress,
    detailEntrySettling,
    detailHeaderPreviousTitle,
    detailHeaderSwapProgress,
    detailBackExitProgress,
    detailSourceExitProgress,
    detailReturningToFeed,
    detailListReturnCommitted,
    detailRestoringFromSourceReader,
    sourceReaderReturnMode,
    detailScrollTop,
    detailScrollHeight,
    detailScrollClientHeight,
    detailFrameContentHeight,
    detailProgressDragging,
    parkedDetailStack,
    sourceReaderBackDetail,
    sourceCatalogEntry,
    sourceSubscription,
    sourceSubscriptionLoading,
    sourceNotice,
    sourceTimelinePreloadEnabled,
    detailTransitionRectsLocked,
    detailFeedOriginLocked,
    sourceReturnTargetReady,
    sourceReaderMounted,
    sourceReaderOpen,
    detailReaderOpen,
    detailCommittedListReturn,
    hasDetailParkedBehindSource,
    hasParkedDetailSourceState,
    sourceReaderShouldReturnToDetail,
    snapshotCurrentDetail,
    snapshotParkedDetail,
    pushParkedDetailSnapshot,
    restoreParkedDetailSnapshot,
    restorePreviousParkedDetail,
    resetDetailTransition,
    clearHiddenSourceReader,
    detailBlocksGestures,
  }
}
