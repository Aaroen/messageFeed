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

type RestoreSourceReaderBackTargetStateResult =
  | {
      action: 'restore-detail'
    }
  | {
      action: 'close-source'
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

type OpenItemReaderTransitionOptions = BeginOpenItemReaderStateOptions & {
  detailEntryDelay: number
  headerSwapDelay: number
  afterBegin?: () => void
  afterEntry?: () => void
}

type FinishOpenItemReaderLoadOptions = {
  item?: FeedItem
  errorMessage?: string
}

type BeginDetailEntryStateResult = {
  shouldAnimate: boolean
}

type BeginDetailHeaderTitleSwapStateResult = {
  shouldAnimate: boolean
}

type ApplyDetailSourceTransitionRectsStateOptions = {
  itemRect: RectSnapshot | null
  sourceOriginRect: RectSnapshot | null
  sourceTargetRect: RectSnapshot | null
  lock?: boolean
}

type ApplyDetailSourceTransitionRectsStateResult = {
  locked: boolean
}

type CloseItemReaderStateResult = {
  shouldScheduleHiddenSourceCleanup: boolean
}

type BeginCollapseItemReaderStateResult = {
  shouldRefreshFeedOrigin: boolean
  shouldRestorePreviousParkedDetail: boolean
}

type CollapseItemReaderTransitionOptions = {
  afterBegin?: (result: BeginCollapseItemReaderStateResult) => void
  afterFinish?: (result: BeginCollapseItemReaderStateResult) => void
}

type BeginRestoreItemReaderExpansionStateResult = {
  shouldHideSourceAfterRestore: boolean
}

type CompleteDetailToSourceReaderTransitionOptions = {
  afterBegin?: () => void
  afterFinish?: () => void
}

type RestoreDetailFromParkedSourceTransitionOptions = {
  beforeBegin?: () => void
  afterBegin?: () => void
  afterFinish?: () => void
}

type ReaderBackSwipeIntentState =
  | {
      intent: 'source-return'
    }
  | {
      intent: 'detail-back'
      returningToFeed: boolean
      revealSourceReader: boolean
      resetSourceExit?: boolean
    }
  | {
      intent: 'blocked'
      clearReturningToFeed?: boolean
    }

export type ReaderBackSwipeTarget = 'detail' | 'source' | 'page' | null
export type ReaderBackSwipeIntent = 'back' | 'source' | 'blocked' | null
type ActiveReaderBackSwipeTarget = Exclude<ReaderBackSwipeTarget, null>
type ActiveReaderBackSwipeIntent = Exclude<ReaderBackSwipeIntent, null>

type ReaderBackSwipeVisualState =
  | {
      target: 'detail-back'
      progress: number
    }
  | {
      target: 'source-return'
      returnProgress: number
    }
  | {
      target: 'source-blocked'
      stretch: number
    }
  | {
      target: 'detail-source'
      progress: number
    }
  | {
      target: 'detail-blocked'
      stretch: number
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

function updateStretchAnchor(anchorRef: { value: 'left' | 'right' | null }, stretch: number) {
  if (stretch > 0) {
    anchorRef.value = 'left'
  } else if (stretch < 0) {
    anchorRef.value = 'right'
  }
}

export function useReaderStackState() {
  let readerMotionTimer = 0
  let morphingHeightUnlockTimer = 0
  let hiddenSourceCleanupTimer = 0
  let detailHeaderSwapTimer = 0
  let detailEntryTimer = 0

  const sourceReaderContentRef = ref<HTMLElement | null>(null)
  const detailContentRef = ref<HTMLElement | null>(null)
  const detailFrameRef = ref<HTMLIFrameElement | null>(null)
  const detailInlineSourceRef = ref<HTMLElement | null>(null)

  const sourceReaderScrollTop = ref(0)
  const detailReaderTouchOffset = ref(0)
  const detailReaderStretch = ref(0)
  const sourceReaderOffset = ref(0)
  const sourceReaderStretch = ref(0)
  const detailStretchAnchor = ref<'left' | 'right' | null>(null)
  const sourceStretchAnchor = ref<'left' | 'right' | null>(null)
  const readerBackDragging = ref(false)
  const backSwipeTarget = ref<ReaderBackSwipeTarget>(null)
  const backSwipeIntent = ref<ReaderBackSwipeIntent>(null)
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
  const parkedDetailStackDepth = computed(() => parkedDetailStack.value.length)
  const sourceReaderBackDetailItemId = computed(() => sourceReaderBackDetail.value?.item.id ?? 0)
  const sourceCatalogEntry = ref<SourceCatalogEntry | null>(null)
  const sourceSubscription = ref<Source | null>(null)
  const sourceSubscriptionLoading = ref(false)
  const sourceNotice = ref<{ type: 'success' | 'warning'; message: string } | null>(null)
  const sourceTimelinePreloadEnabled = ref(false)
  const detailTransitionRectsLocked = ref(false)
  const detailFeedOriginLocked = ref(false)
  const sourceReturnTargetReady = ref(false)
  const sourceReaderBlockedBackSwipeActive = computed(
    () => readerBackDragging.value && backSwipeTarget.value === 'source' && backSwipeIntent.value === 'blocked',
  )
  const sourceReaderReturnTargetPending = computed(
    () => backSwipeTarget.value === 'source' && !sourceReturnTargetReady.value,
  )
  const readerBackSwipeCanCommitRight = computed(() => backSwipeTarget.value === 'detail')

  const sourceReaderMounted = computed(() => readerSource.value !== null)
  const sourceReaderOpen = computed(() => readerSource.value !== null && sourceReaderVisible.value)
  const detailReaderOpen = computed(
    () => detailItem.value !== null || detailLoading.value || detailError.value !== '',
  )
  const sourceReaderUnderDetail = computed(() => detailReaderOpen.value && sourceReaderVisible.value)
  const sourceReaderRevealProgress = computed(() =>
    clampProgress(Math.max(detailSourceExitProgress.value, detailOpenedFromSourceReader.value ? detailBackExitProgress.value : 0)),
  )
  const sourceNameMorphProgress = sourceReaderRevealProgress
  const detailSurfaceProgress = computed(() =>
    clampProgress(detailEntryProgress.value * (1 - Math.max(detailBackExitProgress.value, detailSourceExitProgress.value))),
  )
  const detailScrollMax = computed(() => Math.max(0, detailScrollHeight.value - detailScrollClientHeight.value))
  const detailReadingProgress = computed(() =>
    detailScrollMax.value > 0 ? clampProgress(detailScrollTop.value / detailScrollMax.value) : 0,
  )
  const detailParkedBehindSource = computed(() => hasDetailParkedBehindSource() && !readerBackDragging.value)
  const detailChromeVisible = computed(
    () =>
      detailReaderOpen.value &&
      !detailParkedBehindSource.value &&
      (!detailReturningToFeed.value || readerBackDragging.value),
  )
  const detailProgressVisible = computed(
    () =>
      detailReaderOpen.value &&
      !detailCommittedListReturn() &&
      detailSurfaceProgress.value > 0.86 &&
      detailScrollMax.value > 8,
  )
  const feedItemPreviewProgress = computed(() => {
    if (
      sourceReaderVisible.value &&
      detailReaderOpen.value &&
      !detailParkedBehindSource.value &&
      (detailSourceExitProgress.value > 0 ||
        detailRestoringFromSourceReader.value ||
        (detailOpenedFromSourceReader.value && detailBackExitProgress.value > 0))
    ) {
      return clampProgress(Math.max(detailSourceExitProgress.value, detailBackExitProgress.value))
    }

    if (detailParkedBehindSource.value) {
      return 1
    }

    return clampProgress(Math.max(detailBackExitProgress.value, detailListReturnCommitted.value ? 1 : 0))
  })
  const sourceNameTransitionActive = computed(
    () =>
      Boolean(detailItem.value) &&
      sourceReaderVisible.value &&
      !sourceReaderBlockedBackSwipeActive.value &&
      !detailReturningToFeed.value &&
      !detailCommittedListReturn() &&
      (readerBackDragging.value ||
        detailEntrySettling.value ||
        detailRestoringFromSourceReader.value ||
        detailSourceExitProgress.value > 0.001 ||
        (detailOpenedFromSourceReader.value && detailBackExitProgress.value > 0.001)),
  )
  const sourceTitleProgress = computed(() =>
    detailReaderOpen.value && sourceReaderVisible.value && !detailCommittedListReturn()
      ? sourceNameMorphProgress.value
      : 1,
  )
  const sourceTitleRevealProgress = computed(() =>
    clampProgress((sourceTitleProgress.value - 0.64) / 0.24),
  )
  const sourceTitleRevealReady = computed(
    () =>
      Boolean(readerSource.value) &&
      sourceNameTransitionActive.value &&
      sourceTitleRevealProgress.value > 0.001 &&
      !detailRestoringFromSourceReader.value,
  )
  const sourceNameMorphActive = computed(
    () =>
      sourceNameTransitionActive.value &&
      sourceNameMorphProgress.value > 0.001 &&
      sourceNameMorphProgress.value < 0.985 &&
      Boolean(detailSourceNameOriginRect.value && detailSourceNameTargetRect.value) &&
      (readerBackDragging.value ||
        detailRestoringFromSourceReader.value ||
        detailSourceExitProgress.value > 0.001),
  )
  const sourceNameMorphVisible = computed(
    () =>
      sourceNameTransitionActive.value &&
      sourceNameMorphProgress.value > 0.001 &&
      sourceNameMorphProgress.value < 0.995 &&
      Boolean(detailSourceNameOriginRect.value && detailSourceNameTargetRect.value),
  )
  const detailMorphSummaryVisible = computed(() => detailSurfaceProgress.value < 0.54)
  const detailMorphTextVisible = computed(() => {
    if (!detailItem.value || detailCommittedListReturn()) {
      return false
    }

    return (
      detailEntrySettling.value ||
      readerBackDragging.value ||
      detailReturningToFeed.value ||
      detailRestoringFromSourceReader.value ||
      detailBackExitProgress.value > 0.001 ||
      detailSourceExitProgress.value > 0.001
    )
  })
  const detailHeaderTitleSwapping = computed(() =>
    Boolean(detailHeaderPreviousTitle.value) && detailHeaderSwapProgress.value < 0.999,
  )
  const detailSourceListTitleProgress = computed(() =>
    sourceReaderVisible.value && !detailReturningToFeed.value
      ? clampProgress((1 - sourceNameMorphProgress.value) / 0.52)
      : 0,
  )
  const detailHeaderFeedTitleProgress = computed(() =>
    clampProgress((detailSurfaceProgress.value - 0.58) / 0.22),
  )
  const sourceNameMorphLabelOpacity = computed(() => {
    const progress = sourceNameMorphProgress.value
    return sourceNameMorphActive.value ? clampProgress((0.2 - progress) / 0.2) : 1 - progress
  })
  const sourceNameMorphLabelBlur = computed(() => {
    const progress = sourceNameMorphProgress.value
    return sourceNameMorphActive.value ? clampProgress(progress / 0.2) * 2.2 : progress * 1.8
  })
  const detailFeedHeaderReturnProgress = computed(() => {
    if (!detailReaderOpen.value || detailOpenedFromSourceReader.value) {
      return 0
    }
    if (
      sourceReaderVisible.value &&
      !detailReturningToFeed.value &&
      (detailSourceExitProgress.value > 0.001 ||
        detailRestoringFromSourceReader.value ||
        detailListReturnCommitted.value)
    ) {
      return 0
    }
    return clampProgress(Math.max(detailBackExitProgress.value, detailListReturnCommitted.value ? 1 : 0))
  })

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

  function sourceReaderCanReturnToDetail() {
    return sourceReaderShouldReturnToDetail() || hasParkedDetailSourceState() || detailRestoringFromSourceReader.value
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

  function restorePreviousParkedDetailIfReaderClosed(options: RestoreParkedDetailOptions = {}) {
    return !detailReaderOpen.value && parkedDetailStack.value.length > 0 && restorePreviousParkedDetail(options)
  }

  function restoreSourceReaderBackTargetState(
    options: RestoreParkedDetailOptions = {},
  ): RestoreSourceReaderBackTargetStateResult {
    if (detailReaderOpen.value) {
      return { action: 'restore-detail' }
    }

    if (parkedDetailStack.value.length > 0 && restorePreviousParkedDetail(options)) {
      return { action: 'restore-detail' }
    }

    if (sourceReaderBackDetail.value && restoreParkedDetailSnapshot(sourceReaderBackDetail.value, options)) {
      return { action: 'restore-detail' }
    }

    clearSourceReaderReturnModeState()
    return { action: 'close-source' }
  }

  function prepareSourceReaderReturnDragState(options: RestoreParkedDetailOptions = {}) {
    if (detailReaderOpen.value) {
      return true
    }

    const parkedSnapshot = parkedDetailStack.value[parkedDetailStack.value.length - 1] ?? null
    const snapshot = sourceReaderBackDetail.value ?? parkedSnapshot
    if (!snapshot) {
      return false
    }

    return restoreParkedDetailSnapshot(snapshot, options)
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

  function clearHiddenSourceCleanupTimer() {
    window.clearTimeout(hiddenSourceCleanupTimer)
  }

  function scheduleHiddenSourceReaderCleanupWithDelay(delay = 180) {
    clearHiddenSourceCleanupTimer()
    hiddenSourceCleanupTimer = window.setTimeout(() => {
      clearHiddenSourceReader()
    }, delay)
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

  function beginDetailEntryState(originRect: RectSnapshot | null): BeginDetailEntryStateResult {
    detailOriginRect.value = originRect
    morphingItemHeight.value = originRect?.height ?? null

    if (!detailOriginRect.value) {
      detailEntryProgress.value = 1
      detailEntrySettling.value = false
      return { shouldAnimate: false }
    }

    detailEntryProgress.value = 0
    detailEntrySettling.value = true
    return { shouldAnimate: true }
  }

  function commitDetailEntryState() {
    detailEntryProgress.value = 1
  }

  function finishDetailEntryState() {
    detailEntrySettling.value = false
  }

  function clearDetailEntryTimer() {
    window.clearTimeout(detailEntryTimer)
  }

  function setDetailEntryTimer(handler: () => void, delay?: number) {
    detailEntryTimer = window.setTimeout(handler, delay)
  }

  function startDetailEntryWithDelay(originRect: RectSnapshot | null, delay: number) {
    clearDetailEntryTimer()
    const result = beginDetailEntryState(originRect)
    if (!result.shouldAnimate) {
      return
    }

    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        commitDetailEntryState()
        setDetailEntryTimer(() => {
          finishDetailEntryState()
        }, delay)
      })
    })
  }

  function completeOpenItemReaderLoadState(item?: FeedItem) {
    if (item) {
      detailItem.value = item
    }
    detailLoading.value = false
  }

  function failOpenItemReaderLoadState(message: string) {
    detailError.value = message
    detailLoading.value = false
  }

  function finishOpenItemReaderLoad(options: FinishOpenItemReaderLoadOptions = {}) {
    if (options.errorMessage) {
      failOpenItemReaderLoadState(options.errorMessage)
      return
    }
    completeOpenItemReaderLoadState(options.item)
  }

  function beginDetailHeaderTitleSwapState(nextItem: FeedItem): BeginDetailHeaderTitleSwapStateResult {
    if (!detailItem.value || detailItem.value.id === nextItem.id) {
      detailHeaderPreviousTitle.value = ''
      detailHeaderSwapProgress.value = 1
      return { shouldAnimate: false }
    }

    detailHeaderPreviousTitle.value = detailItem.value.title
    detailHeaderSwapProgress.value = 0
    return { shouldAnimate: true }
  }

  function commitDetailHeaderTitleSwapState() {
    detailHeaderSwapProgress.value = 1
  }

  function finishDetailHeaderTitleSwapState() {
    detailHeaderPreviousTitle.value = ''
  }

  function clearDetailHeaderSwapTimer() {
    window.clearTimeout(detailHeaderSwapTimer)
  }

  function startDetailHeaderTitleSwapWithDelay(nextItem: FeedItem, delay = 320) {
    const result = beginDetailHeaderTitleSwapState(nextItem)
    clearDetailHeaderSwapTimer()
    if (!result.shouldAnimate) {
      return
    }

    requestAnimationFrame(() => {
      commitDetailHeaderTitleSwapState()
    })
    detailHeaderSwapTimer = window.setTimeout(() => {
      finishDetailHeaderTitleSwapState()
    }, delay)
  }

  function openItemReaderWithTransition(
    item: FeedItem,
    sourceKind: FeedSourceKind,
    options: OpenItemReaderTransitionOptions,
  ) {
    clearMorphingHeightUnlockTimer()
    clearHiddenSourceCleanupTimer()
    startDetailHeaderTitleSwapWithDelay(item, options.headerSwapDelay)
    beginOpenItemReaderState(item, sourceKind, {
      openedFromSourceReader: options.openedFromSourceReader,
      originRect: options.originRect,
    })
    options.afterBegin?.()
    startDetailEntryWithDelay(options.originRect, options.detailEntryDelay)
    options.afterEntry?.()
  }

  function applyDetailFeedOriginRectState(itemRect: RectSnapshot, lock = false) {
    detailOriginRect.value = itemRect
    morphingItemHeight.value = itemRect.height
    if (lock) {
      detailFeedOriginLocked.value = true
    }
  }

  function applyDetailSourceTransitionRectsState(
    options: ApplyDetailSourceTransitionRectsStateOptions,
  ): ApplyDetailSourceTransitionRectsStateResult {
    const { itemRect, sourceOriginRect, sourceTargetRect, lock } = options

    if (itemRect) {
      detailSourceItemTargetRect.value = itemRect
      morphingItemHeight.value = itemRect.height
    }
    if (sourceOriginRect) {
      detailSourceNameOriginRect.value = sourceOriginRect
    }
    if (sourceTargetRect) {
      detailSourceNameTargetRect.value = sourceTargetRect
    }

    const hasSourceOrigin = Boolean(sourceOriginRect || detailSourceNameOriginRect.value)
    const shouldLock = Boolean(lock && itemRect && sourceTargetRect && hasSourceOrigin)
    if (shouldLock) {
      detailTransitionRectsLocked.value = true
    }
    return { locked: shouldLock }
  }

  function applyVisibleSourceReturnTargetState(
    itemRect: RectSnapshot | null,
    sourceOriginRect: RectSnapshot | null,
    sourceTargetRect: RectSnapshot | null,
  ) {
    if (!itemRect) {
      sourceReturnTargetReady.value = false
      return false
    }

    detailSourceItemTargetRect.value = itemRect
    morphingItemHeight.value = itemRect.height
    if (sourceTargetRect) {
      detailSourceNameTargetRect.value = sourceTargetRect
    }
    if (sourceOriginRect) {
      detailSourceNameOriginRect.value = sourceOriginRect
    }
    sourceReturnTargetReady.value = true
    return true
  }

  function beginRestoreMorphingItemContentState() {
    const lockedItemId = morphingItemId.value ?? morphingHeightLockItemId.value
    morphingItemId.value = null
    morphingHeightLockItemId.value = lockedItemId
  }

  function finishRestoreMorphingItemContentState() {
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = null
  }

  function clearMorphingHeightUnlockTimer() {
    window.clearTimeout(morphingHeightUnlockTimer)
  }

  function restoreMorphingItemContentWithDelay(unlockDelay = 180) {
    beginRestoreMorphingItemContentState()
    clearMorphingHeightUnlockTimer()
    morphingHeightUnlockTimer = window.setTimeout(() => {
      finishRestoreMorphingItemContentState()
    }, unlockDelay)
  }

  function revealSourceReaderUnderDetailState() {
    sourceReaderVisible.value = true
  }

  function beginReaderMotionSettlingState() {
    readerBackDragging.value = false
    readerMotionSettling.value = true
  }

  function finishReaderMotionSettlingState() {
    readerMotionSettling.value = false
  }

  function clearReaderMotionTimer() {
    window.clearTimeout(readerMotionTimer)
  }

  function settleReaderMotionWithDelay(delay = 260, done?: () => void) {
    beginReaderMotionSettlingState()
    clearReaderMotionTimer()
    readerMotionTimer = window.setTimeout(() => {
      finishReaderMotionSettlingState()
      done?.()
    }, delay)
  }

  function clearReaderStackTimers() {
    clearReaderMotionTimer()
    clearDetailEntryTimer()
    clearDetailHeaderSwapTimer()
    clearMorphingHeightUnlockTimer()
    clearHiddenSourceCleanupTimer()
  }

  function updateSourceReaderScrollTopState(scrollTop: number) {
    sourceReaderScrollTop.value = Math.max(0, scrollTop)
  }

  function updateDetailScrollMetricsState(scrollTop: number, scrollHeight: number, clientHeight: number) {
    detailScrollTop.value = Math.max(0, scrollTop)
    detailScrollHeight.value = Math.max(0, scrollHeight)
    detailScrollClientHeight.value = Math.max(0, clientHeight)
  }

  function updateDetailScrollTopState(scrollTop: number) {
    detailScrollTop.value = Math.max(0, scrollTop)
  }

  function updateDetailFrameContentHeightState(scrollHeight: number) {
    detailFrameContentHeight.value = Math.max(0, scrollHeight)
  }

  function setDetailProgressDraggingState(dragging: boolean) {
    detailProgressDragging.value = dragging
  }

  function clearSourceReturnTargetReadyState() {
    sourceReturnTargetReady.value = false
  }

  function clearSourceReaderReturnModeState() {
    sourceReaderReturnMode.value = null
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

  function closeItemReaderWithTransition() {
    clearDetailHeaderSwapTimer()
    restoreMorphingItemContentWithDelay()
    return closeItemReaderState()
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

  function collapseItemReaderWithDelay(
    delay: number,
    options: CollapseItemReaderTransitionOptions = {},
  ) {
    const result = beginCollapseItemReaderState()
    options.afterBegin?.(result)
    clearDetailEntryTimer()
    setDetailEntryTimer(() => {
      options.afterFinish?.(result)
    }, delay)
  }

  function beginRestoreItemReaderExpansionState(): BeginRestoreItemReaderExpansionStateResult {
    const shouldHideSourceAfterRestore =
      detailOpenedFromSourceReader.value && sourceReaderVisible.value
    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
    detailFeedOriginLocked.value = false
    return { shouldHideSourceAfterRestore }
  }

  function finishRestoreItemReaderExpansionState(shouldHideSourceAfterRestore: boolean) {
    detailEntrySettling.value = false
    if (shouldHideSourceAfterRestore) {
      sourceReaderVisible.value = false
    }
  }

  function restoreItemReaderExpansionWithDelay(delay: number) {
    const result = beginRestoreItemReaderExpansionState()
    clearDetailEntryTimer()
    setDetailEntryTimer(() => {
      finishRestoreItemReaderExpansionState(result.shouldHideSourceAfterRestore)
    }, delay)
  }

  function beginRestoreDetailFromSourceSwipeState() {
    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailSourceExitProgress.value = 0
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
  }

  function finishRestoreDetailFromSourceSwipeState() {
    detailEntrySettling.value = false
    sourceReaderVisible.value = false
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
  }

  function restoreDetailFromSourceSwipeWithDelay(delay: number) {
    beginRestoreDetailFromSourceSwipeState()
    clearDetailEntryTimer()
    setDetailEntryTimer(() => {
      finishRestoreDetailFromSourceSwipeState()
    }, delay)
  }

  function beginCompleteDetailToSourceReaderState() {
    if (!readerSource.value && detailItem.value?.source_id) {
      readerSource.value = {
        id: detailItem.value.source_id,
        name: detailItem.value.source_name || '未知来源',
        kind: detailSourceKind.value,
      }
    }

    const startProgress = detailSourceExitProgress.value > 0.001 ? detailSourceExitProgress.value : 0
    if (!sourceReaderBackDetail.value) {
      sourceReaderBackDetail.value = snapshotCurrentDetail()
    }
    sourceReaderReturnMode.value = 'detail'
    sourceReaderVisible.value = true
    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = startProgress
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
  }

  function commitCompleteDetailToSourceReaderState() {
    detailSourceExitProgress.value = 1
  }

  function finishCompleteDetailToSourceReaderState() {
    detailEntrySettling.value = false
    detailListReturnCommitted.value = true
    detailSourceExitProgress.value = 1
  }

  function completeDetailToSourceReaderWithDelay(
    delay: number,
    options: CompleteDetailToSourceReaderTransitionOptions = {},
  ) {
    beginCompleteDetailToSourceReaderState()
    options.afterBegin?.()
    clearDetailEntryTimer()
    requestAnimationFrame(() => {
      requestAnimationFrame(() => {
        commitCompleteDetailToSourceReaderState()
      })
    })
    setDetailEntryTimer(() => {
      finishCompleteDetailToSourceReaderState()
      options.afterFinish?.()
    }, delay)
  }

  function beginRestoreParkedSourceReaderState() {
    if (!detailReaderOpen.value || !sourceReaderVisible.value) {
      return false
    }

    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailRestoringFromSourceReader.value = true
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = Math.max(detailSourceExitProgress.value, 0.001)
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
    return true
  }

  function commitRestoreParkedSourceReaderState() {
    detailSourceExitProgress.value = 1
  }

  function finishRestoreParkedSourceReaderState() {
    detailEntrySettling.value = false
    detailRestoringFromSourceReader.value = false
    detailSourceExitProgress.value = 1
    detailListReturnCommitted.value = true
  }

  function restoreParkedSourceReaderWithDelay(delay: number) {
    if (!beginRestoreParkedSourceReaderState()) {
      return false
    }

    clearDetailEntryTimer()
    requestAnimationFrame(() => {
      commitRestoreParkedSourceReaderState()
    })
    setDetailEntryTimer(() => {
      finishRestoreParkedSourceReaderState()
    }, delay)
    return true
  }

  function beginRestoreDetailFromParkedSourceState() {
    if (!detailReaderOpen.value) {
      return false
    }

    if (detailItem.value?.id) {
      morphingItemId.value = detailItem.value.id
      morphingHeightLockItemId.value = detailItem.value.id
      morphingItemHeight.value = detailSourceItemTargetRect.value?.height ?? morphingItemHeight.value
    }

    const startProgress = detailSourceExitProgress.value > 0.001 ? detailSourceExitProgress.value : 1
    readerBackDragging.value = false
    detailEntrySettling.value = true
    detailRestoringFromSourceReader.value = true
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = startProgress
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = false
    return true
  }

  function commitRestoreDetailFromParkedSourceState() {
    detailSourceExitProgress.value = 0
  }

  function finishRestoreDetailFromParkedSourceState() {
    detailEntrySettling.value = false
    detailRestoringFromSourceReader.value = false
    sourceReaderReturnMode.value = null
    sourceReaderBackDetail.value = null
    parkedDetailStack.value = []
    sourceReaderVisible.value = false
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
  }

  function restoreDetailFromParkedSourceWithDelay(
    delay: number,
    options: RestoreDetailFromParkedSourceTransitionOptions = {},
  ) {
    clearDetailEntryTimer()
    options.beforeBegin?.()
    if (!beginRestoreDetailFromParkedSourceState()) {
      return false
    }
    options.afterBegin?.()

    requestAnimationFrame(() => {
      commitRestoreDetailFromParkedSourceState()
    })

    setDetailEntryTimer(() => {
      finishRestoreDetailFromParkedSourceState()
      options.afterFinish?.()
    }, delay)
    return true
  }

  function resetReaderBackSwipeState() {
    const keepDetailParkedBehindSource = hasParkedDetailSourceState()
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = keepDetailParkedBehindSource ? 1 : 0
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = false
    detailReaderStretch.value = 0
    sourceReaderStretch.value = 0
    sourceReaderOffset.value = 0
    readerBackDragging.value = false
    sourceReturnTargetReady.value = false
  }

  function resetReaderBackSwipeTargetState() {
    backSwipeTarget.value = null
    backSwipeIntent.value = null
  }

  function setReaderBackSwipeTargetState(target: ReaderBackSwipeTarget) {
    backSwipeTarget.value = target
  }

  function setReaderBackSwipeIntentState(intent: ReaderBackSwipeIntent) {
    backSwipeIntent.value = intent
  }

  function getReaderBackSwipeState() {
    return {
      target: backSwipeTarget.value,
      intent: backSwipeIntent.value,
    }
  }

  function readerBackSwipeMatches(
    target: ActiveReaderBackSwipeTarget,
    intent?: ActiveReaderBackSwipeIntent,
  ) {
    return backSwipeTarget.value === target && (intent === undefined || backSwipeIntent.value === intent)
  }

  function readerBackSwipeTransitionProgress(fallbackStretch = 0) {
    if (readerBackSwipeMatches('detail', 'source')) {
      return detailSourceExitProgress.value
    }
    if (readerBackSwipeMatches('source', 'back')) {
      return 1 - detailSourceExitProgress.value
    }
    if (readerBackSwipeMatches('detail', 'back')) {
      return detailBackExitProgress.value
    }
    return clampProgress(Math.abs(detailReaderStretch.value || sourceReaderStretch.value || fallbackStretch) / 0.07)
  }

  function beginReaderBackSwipeTrackingState() {
    detailEntrySettling.value = false
    sourceReturnTargetReady.value = false
  }

  function prepareReaderBackSwipeIntentState(state: ReaderBackSwipeIntentState) {
    if (state.intent === 'source-return') {
      detailRestoringFromSourceReader.value = true
      detailReturningToFeed.value = false
      return
    }

    if (state.intent === 'detail-back') {
      if (state.resetSourceExit) {
        detailSourceExitProgress.value = 0
      }
      detailReturningToFeed.value = state.returningToFeed
      if (state.revealSourceReader) {
        sourceReaderVisible.value = true
      }
      return
    }

    if (state.intent === 'blocked' && state.clearReturningToFeed) {
      detailReturningToFeed.value = false
    }
  }

  function startReaderBackSwipeDragState() {
    readerBackDragging.value = true
  }

  function resetReaderBackSwipeStretchState() {
    detailReaderStretch.value = 0
    sourceReaderStretch.value = 0
  }

  function applyReaderBackSwipeVisualState(state: ReaderBackSwipeVisualState) {
    if (state.target === 'detail-back') {
      detailReaderTouchOffset.value = 0
      detailBackExitProgress.value = clampProgress(state.progress)
      return
    }

    if (state.target === 'source-return') {
      detailRestoringFromSourceReader.value = true
      detailReaderTouchOffset.value = 0
      detailBackExitProgress.value = 0
      sourceReaderOffset.value = 0
      detailSourceExitProgress.value = 1 - clampProgress(state.returnProgress)
      return
    }

    if (state.target === 'source-blocked') {
      detailSourceExitProgress.value = hasParkedDetailSourceState() ? 1 : 0
      sourceReaderStretch.value = state.stretch
      sourceReaderOffset.value = 0
      updateStretchAnchor(sourceStretchAnchor, state.stretch)
      return
    }

    if (state.target === 'detail-source') {
      detailReaderTouchOffset.value = 0
      detailBackExitProgress.value = 0
      detailSourceExitProgress.value = clampProgress(state.progress)
      return
    }

    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = 0
    detailReaderStretch.value = state.stretch
    updateStretchAnchor(detailStretchAnchor, state.stretch)
  }

  function detailBlocksGestures() {
    return detailReaderOpen.value && !detailCommittedListReturn()
  }

  return {
    sourceReaderContentRef,
    detailContentRef,
    detailFrameRef,
    detailInlineSourceRef,
    sourceReaderScrollTop,
    detailReaderTouchOffset,
    detailReaderStretch,
    sourceReaderOffset,
    sourceReaderStretch,
    detailStretchAnchor,
    sourceStretchAnchor,
    readerBackDragging,
    sourceReaderBlockedBackSwipeActive,
    sourceReaderReturnTargetPending,
    readerBackSwipeCanCommitRight,
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
    parkedDetailStackDepth,
    sourceReaderBackDetailItemId,
    sourceCatalogEntry,
    sourceSubscription,
    sourceSubscriptionLoading,
    sourceNotice,
    sourceTimelinePreloadEnabled,
    detailTransitionRectsLocked,
    detailFeedOriginLocked,
    sourceReaderMounted,
    sourceReaderOpen,
    detailReaderOpen,
    sourceReaderUnderDetail,
    sourceReaderRevealProgress,
    sourceNameMorphProgress,
    detailSurfaceProgress,
    detailScrollMax,
    detailReadingProgress,
    detailProgressVisible,
    feedItemPreviewProgress,
    sourceNameTransitionActive,
    sourceTitleProgress,
    sourceTitleRevealProgress,
    sourceTitleRevealReady,
    sourceNameMorphActive,
    sourceNameMorphVisible,
    detailMorphSummaryVisible,
    detailMorphTextVisible,
    detailHeaderTitleSwapping,
    detailSourceListTitleProgress,
    detailHeaderFeedTitleProgress,
    sourceNameMorphLabelOpacity,
    sourceNameMorphLabelBlur,
    detailFeedHeaderReturnProgress,
    detailParkedBehindSource,
    detailChromeVisible,
    detailCommittedListReturn,
    hasDetailParkedBehindSource,
    hasParkedDetailSourceState,
    sourceReaderShouldReturnToDetail,
    sourceReaderCanReturnToDetail,
    createReaderStackSessionSnapshot,
    applyReaderStackSessionSnapshot,
    snapshotCurrentDetail,
    snapshotParkedDetail,
    pushParkedDetailSnapshot,
    restoreParkedDetailSnapshot,
    restorePreviousParkedDetail,
    restorePreviousParkedDetailIfReaderClosed,
    restoreSourceReaderBackTargetState,
    prepareSourceReaderReturnDragState,
    clearHiddenSourceReader,
    clearHiddenSourceCleanupTimer,
    scheduleHiddenSourceReaderCleanupWithDelay,
    openSourceReaderState,
    closeVisibleSourceReaderState,
    clearSourceReaderState,
    clearDetailEntryTimer,
    finishOpenItemReaderLoad,
    openItemReaderWithTransition,
    applyDetailFeedOriginRectState,
    applyDetailSourceTransitionRectsState,
    applyVisibleSourceReturnTargetState,
    beginRestoreMorphingItemContentState,
    finishRestoreMorphingItemContentState,
    clearMorphingHeightUnlockTimer,
    restoreMorphingItemContentWithDelay,
    revealSourceReaderUnderDetailState,
    beginReaderMotionSettlingState,
    finishReaderMotionSettlingState,
    settleReaderMotionWithDelay,
    clearReaderStackTimers,
    updateSourceReaderScrollTopState,
    updateDetailScrollMetricsState,
    updateDetailScrollTopState,
    updateDetailFrameContentHeightState,
    setDetailProgressDraggingState,
    clearSourceReturnTargetReadyState,
    closeItemReaderWithTransition,
    collapseItemReaderWithDelay,
    restoreItemReaderExpansionWithDelay,
    restoreDetailFromSourceSwipeWithDelay,
    completeDetailToSourceReaderWithDelay,
    restoreParkedSourceReaderWithDelay,
    restoreDetailFromParkedSourceWithDelay,
    resetReaderBackSwipeState,
    resetReaderBackSwipeTargetState,
    setReaderBackSwipeTargetState,
    setReaderBackSwipeIntentState,
    getReaderBackSwipeState,
    readerBackSwipeMatches,
    readerBackSwipeTransitionProgress,
    beginReaderBackSwipeTrackingState,
    prepareReaderBackSwipeIntentState,
    startReaderBackSwipeDragState,
    resetReaderBackSwipeStretchState,
    applyReaderBackSwipeVisualState,
    detailBlocksGestures,
  }
}
