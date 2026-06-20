import { computed, ref } from 'vue'

import type { FeedItem, Source, SourceCatalogEntry } from '@/api/feed'
import type {
  FeedSourceKind,
  ParkedDetailSnapshot,
  ReaderSessionSnapshot,
  ReaderSource,
  RectSnapshot,
} from '@/composables/useReaderSession'

type RestoreParkedDetailOptions = {
  onDetailScrollTop?: (scrollTop: number) => void
}

type ApplyReaderStackSessionOptions = {
  onSourceScrollTop?: (scrollTop: number) => void
  onDetailScrollTop?: (scrollTop: number) => void
  onReaderSourceRestored?: (source: ReaderSource) => void
}

type OpenSourceReaderStateOptions = {
  visible?: boolean
}

type OpenSourceReaderStateResult = {
  nextVisible: boolean
  sourceChanged: boolean
  resetScroll: boolean
  captureTransition: boolean
  loadSubscription: boolean
}

type BeginOpenItemReaderStateOptions = {
  openedFromSourceReader: boolean
  originRect: RectSnapshot | null
}

type CloseItemReaderStateResult = {
  shouldScheduleHiddenSourceCleanup: boolean
}

type BeginCollapseItemReaderStateResult = {
  shouldRefreshFeedOrigin: boolean
  shouldRestorePreviousParkedDetail: boolean
}

type ReaderStackSessionSnapshot = Pick<
  ReaderSessionSnapshot,
  | 'sourceReaderScrollTop'
  | 'detailScrollTop'
  | 'readerSource'
  | 'sourceReaderVisible'
  | 'detailItem'
  | 'detailSourceKind'
  | 'detailOpenedFromSourceReader'
  | 'detailListReturnCommitted'
  | 'detailSourceExitProgress'
  | 'sourceReaderReturnMode'
  | 'sourceReaderBackDetail'
  | 'morphingItemHeight'
  | 'parkedDetailStack'
>

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
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

  function cloneParkedDetailSnapshot(snapshot: ParkedDetailSnapshot) {
    return {
      ...snapshot,
      item: { ...snapshot.item },
      originRect: snapshot.originRect ? { ...snapshot.originRect } : null,
      sourceItemTargetRect: snapshot.sourceItemTargetRect ? { ...snapshot.sourceItemTargetRect } : null,
      sourceNameOriginRect: snapshot.sourceNameOriginRect ? { ...snapshot.sourceNameOriginRect } : null,
      sourceNameTargetRect: snapshot.sourceNameTargetRect ? { ...snapshot.sourceNameTargetRect } : null,
    }
  }

  function createReaderStackSessionSnapshot(): ReaderStackSessionSnapshot {
    return {
      sourceReaderScrollTop: sourceReaderScrollTop.value,
      detailScrollTop: detailScrollTop.value,
      readerSource: readerSource.value ? { ...readerSource.value } : null,
      sourceReaderVisible: sourceReaderVisible.value,
      detailItem: detailItem.value ? { ...detailItem.value } : null,
      detailSourceKind: detailSourceKind.value,
      detailOpenedFromSourceReader: detailOpenedFromSourceReader.value,
      detailListReturnCommitted: detailListReturnCommitted.value,
      detailSourceExitProgress: detailSourceExitProgress.value,
      sourceReaderReturnMode: sourceReaderReturnMode.value,
      sourceReaderBackDetail: sourceReaderBackDetail.value
        ? cloneParkedDetailSnapshot(sourceReaderBackDetail.value)
        : null,
      morphingItemHeight: morphingItemHeight.value,
      parkedDetailStack: parkedDetailStack.value.map(cloneParkedDetailSnapshot),
    }
  }

  function applyReaderStackSessionSnapshot(
    snapshot: ReaderSessionSnapshot,
    options: ApplyReaderStackSessionOptions = {},
  ) {
    sourceReaderScrollTop.value = snapshot.sourceReaderScrollTop || 0
    options.onSourceScrollTop?.(sourceReaderScrollTop.value)
    detailScrollTop.value = snapshot.detailScrollTop || 0
    options.onDetailScrollTop?.(detailScrollTop.value)
    readerSource.value = snapshot.readerSource ? { ...snapshot.readerSource } : null
    sourceReaderVisible.value = Boolean(snapshot.readerSource && snapshot.sourceReaderVisible)
    detailItem.value = snapshot.detailItem ? { ...snapshot.detailItem } : null
    detailSourceKind.value = snapshot.detailSourceKind || 'subscriptions'
    detailOpenedFromSourceReader.value = Boolean(snapshot.detailOpenedFromSourceReader)
    detailEntryProgress.value = 1
    detailEntrySettling.value = false
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = snapshot.detailListReturnCommitted
      ? 1
      : clampProgress(snapshot.detailSourceExitProgress || 0)
    sourceReaderReturnMode.value = snapshot.sourceReaderReturnMode === 'detail' ? 'detail' : null
    sourceReaderBackDetail.value = snapshot.sourceReaderBackDetail
      ? cloneParkedDetailSnapshot(snapshot.sourceReaderBackDetail)
      : null
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = Boolean(snapshot.detailListReturnCommitted)
    detailRestoringFromSourceReader.value = false
    detailError.value = ''
    detailLoading.value = false
    detailFrameContentHeight.value = 0
    morphingItemId.value = null
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = snapshot.morphingItemHeight ?? null
    parkedDetailStack.value = Array.isArray(snapshot.parkedDetailStack)
      ? snapshot.parkedDetailStack.map(cloneParkedDetailSnapshot)
      : []

    if (readerSource.value) {
      options.onReaderSourceRestored?.(readerSource.value)
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

  function openSourceReaderState(
    source: ReaderSource,
    options: OpenSourceReaderStateOptions = {},
  ): OpenSourceReaderStateResult {
    const nextVisible = options.visible ?? true
    const sameSource = readerSource.value?.id === source.id && readerSource.value.kind === source.kind

    if (sameSource) {
      if (nextVisible) {
        sourceReaderRefreshNonce.value += 1
        sourceReaderOffset.value = 0
        sourceReaderStretch.value = 0
        if (!detailReaderOpen.value) {
          sourceReaderReturnMode.value = null
          sourceReaderBackDetail.value = null
        }
      }
      sourceReaderVisible.value = nextVisible
      return {
        nextVisible,
        sourceChanged: false,
        resetScroll: false,
        captureTransition: nextVisible && detailReaderOpen.value,
        loadSubscription: nextVisible,
      }
    }

    readerSource.value = source
    if (nextVisible) {
      sourceReaderRefreshNonce.value += 1
      sourceReaderOffset.value = 0
      sourceReaderStretch.value = 0
      if (!detailReaderOpen.value) {
        sourceReaderReturnMode.value = null
        sourceReaderBackDetail.value = null
      }
    }
    sourceReaderVisible.value = nextVisible
    sourceReaderScrollTop.value = 0

    return {
      nextVisible,
      sourceChanged: true,
      resetScroll: true,
      captureTransition: nextVisible && detailReaderOpen.value,
      loadSubscription: true,
    }
  }

  function closeVisibleSourceReaderState() {
    readerBackDragging.value = false
    sourceReaderOffset.value = 0
    sourceReaderStretch.value = 0
    sourceReaderVisible.value = false
    sourceReaderReturnMode.value = null
    sourceReaderBackDetail.value = null
    parkedDetailStack.value = []
  }

  function clearSourceReaderState() {
    readerSource.value = null
    sourceReaderVisible.value = false
    sourceReaderReturnMode.value = null
    sourceReaderBackDetail.value = null
    sourceReaderOffset.value = 0
    sourceReaderStretch.value = 0
    parkedDetailStack.value = []
  }

  function beginOpenItemReaderState(
    item: FeedItem,
    sourceKind: FeedSourceKind,
    options: BeginOpenItemReaderStateOptions,
  ) {
    const { openedFromSourceReader, originRect } = options
    if (openedFromSourceReader && (hasParkedDetailSourceState() || sourceReaderReturnMode.value === 'detail')) {
      pushParkedDetailSnapshot()
    } else if (!openedFromSourceReader) {
      parkedDetailStack.value = []
    }
    detailError.value = ''
    detailLoading.value = true
    detailItem.value = item
    detailSourceKind.value = sourceKind
    detailOpenedFromSourceReader.value = openedFromSourceReader
    morphingItemId.value = item.id
    morphingHeightLockItemId.value = item.id
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
    detailSourceItemTargetRect.value = openedFromSourceReader ? originRect : null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
    detailFeedOriginLocked.value = false
    detailScrollTop.value = 0
    detailScrollHeight.value = 0
    detailScrollClientHeight.value = 0
    detailFrameContentHeight.value = 0
    detailProgressDragging.value = false
    sourceReaderVisible.value = openedFromSourceReader
  }

  function closeItemReaderState(): CloseItemReaderStateResult {
    const previousSourceReturnMode = sourceReaderReturnMode.value
    detailItem.value = null
    detailError.value = ''
    detailLoading.value = false
    detailHeaderPreviousTitle.value = ''
    detailHeaderSwapProgress.value = 1
    detailOpenedFromSourceReader.value = false
    detailRestoringFromSourceReader.value = false
    detailScrollTop.value = 0
    detailScrollHeight.value = 0
    detailScrollClientHeight.value = 0
    detailFrameContentHeight.value = 0
    detailProgressDragging.value = false
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    resetDetailTransition()

    if (sourceReaderVisible.value && previousSourceReturnMode === 'detail') {
      sourceReaderReturnMode.value = 'detail'
      return { shouldScheduleHiddenSourceCleanup: false }
    }

    if (!sourceReaderVisible.value) {
      sourceReaderReturnMode.value = null
      sourceReaderBackDetail.value = null
      parkedDetailStack.value = []
      return { shouldScheduleHiddenSourceCleanup: true }
    }

    return { shouldScheduleHiddenSourceCleanup: false }
  }

  function beginCollapseItemReaderState(): BeginCollapseItemReaderStateResult {
    const shouldRestorePreviousParkedDetail =
      detailOpenedFromSourceReader.value && parkedDetailStack.value.length > 0
    const shouldRefreshFeedOrigin = !detailOpenedFromSourceReader.value

    if (detailOpenedFromSourceReader.value && readerSource.value) {
      sourceReaderVisible.value = true
    }
    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    detailBackExitProgress.value = 1
    detailSourceExitProgress.value = 0
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = !detailOpenedFromSourceReader.value
    detailListReturnCommitted.value = true

    return {
      shouldRefreshFeedOrigin,
      shouldRestorePreviousParkedDetail,
    }
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
    createReaderStackSessionSnapshot,
    applyReaderStackSessionSnapshot,
    snapshotCurrentDetail,
    snapshotParkedDetail,
    pushParkedDetailSnapshot,
    restoreParkedDetailSnapshot,
    restorePreviousParkedDetail,
    resetDetailTransition,
    clearHiddenSourceReader,
    openSourceReaderState,
    closeVisibleSourceReaderState,
    clearSourceReaderState,
    beginOpenItemReaderState,
    closeItemReaderState,
    beginCollapseItemReaderState,
    detailBlocksGestures,
  }
}
