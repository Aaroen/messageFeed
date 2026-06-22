import { computed, ref } from 'vue'

import type { FeedItem, Source, SourceCatalogEntry } from '@/api/feed'
import type {
  FeedSourceKind,
  ParkedDetailSnapshot,
  ReaderSessionSnapshot,
  ReaderSource,
  RectSnapshot,
} from '@/composables/useReaderSession'
import { useMotionTimings } from '@/composables/useMotionTimings'
import { useDetailHeaderTitleSwap } from '@/composables/useDetailHeaderTitleSwap'

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
type ReaderBackSwipeSurface = 'reader:detail' | 'reader:source' | 'page:management'
type ReaderBackSwipeFinishAction =
  | 'restore-item-expansion'
  | 'restore-detail-from-source-swipe'
  | 'restore-parked-source'
  | 'complete-detail-to-source'
  | 'collapse-detail'
  | 'restore-detail-from-parked-source'
  | 'return-page'
  | 'reset'
type ReaderBackSwipeCancelAction =
  | 'restore-item-expansion'
  | 'restore-detail-from-source-swipe'
  | 'restore-parked-source'
  | 'reset'
type ReaderBackSwipeCancelResult = {
  progress: number
  isBlocked: boolean
  action: ReaderBackSwipeCancelAction
}
type ReaderBackSwipeFinishResult = {
  committed: boolean
  progress: number
  isBlocked: boolean
  action: ReaderBackSwipeFinishAction
}
type ReaderBackSwipeAction = ReaderBackSwipeFinishAction | ReaderBackSwipeCancelAction
type ReaderBackSwipeActionHandlers = {
  restoreItemExpansion: () => void
  restoreDetailFromSourceSwipe: () => void
  restoreParkedSource: () => void
  completeDetailToSource: () => void
  collapseDetail: () => void
  restoreDetailFromParkedSource: () => void
  returnPage: () => void
  reset: () => void
}
type ReaderBackSwipeVisualAction =
  | {
      type: 'reader'
      state: ReaderBackSwipeVisualState
    }
  | {
      type: 'page'
      stretch: number
    }
  | {
      type: 'none'
    }
type ReaderBackSwipeIntentAction =
  | {
      type: 'source-return'
    }
  | {
      type: 'right-swipe'
      intent: 'back' | 'blocked'
      returningToFeed: boolean
      revealSourceReader: boolean
    }
  | {
      type: 'detail-source'
    }
  | {
      type: 'blocked'
    }
type ReaderBackSwipeIntentSideEffectState = {
  returningToFeed: boolean
  revealSourceReader: boolean
}
type ApplyReaderBackSwipeIntentStateOptions = {
  resetSourceExit?: boolean
  prepareBlocked?: boolean
  beforeSourceReturnIntent?: () => void
  afterSourceReturnIntent?: () => void
  beforeDetailBackPrepare?: (state: ReaderBackSwipeIntentSideEffectState) => void
  afterDetailBackPrepare?: (state: ReaderBackSwipeIntentSideEffectState) => void
  afterDetailSourceIntent?: () => void
}
type ApplyReaderBackSwipeVisualActionOptions = {
  resetPageStretch?: () => void
  resetPageOffset?: () => void
  applyPageStretch?: (stretch: number) => void
}
type UpdateReaderBackSwipeDragStateOptions = {
  intent?: ApplyReaderBackSwipeIntentStateOptions
  visual?: ApplyReaderBackSwipeVisualActionOptions
}
type ReaderBackSwipeDragMetrics = {
  currentX: number
  startX: number
  width: number
}
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
  const motionTimings = useMotionTimings()
  const hiddenSourceCleanupDelay = motionTimings.hiddenSourceCleanupDelay
  const detailHeaderSwapDelay = motionTimings.detailHeaderSwapDelay
  const morphingItemContentUnlockDelay = motionTimings.morphingItemContentUnlockDelay
  const readerMotionSettleDelay = motionTimings.readerMotionSettleDelay

  let readerMotionTimer = 0
  let morphingHeightUnlockTimer = 0
  let hiddenSourceCleanupTimer = 0
  let readerMotionTimerToken = 0
  let morphingHeightUnlockTimerToken = 0
  let hiddenSourceCleanupTimerToken = 0
  let detailEntryTimer = 0
  let detailEntryFrame = 0
  let detailEntrySecondFrame = 0
  let detailEntryTimerToken = 0
  let detailEntryFrameToken = 0
  const detailHeaderTitleSwap = useDetailHeaderTitleSwap()

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
  const readerBackSwipeCandidateTracking = ref(false)
  const readerBackSwipeGestureTracking = ref(false)
  const readerBackDragging = ref(false)
  const backSwipeTarget = ref<ReaderBackSwipeTarget>(null)
  const backSwipeIntent = ref<ReaderBackSwipeIntent>(null)

  const readerSource = ref<ReaderSource | null>(null)
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
  const detailHeaderPreviousTitle = detailHeaderTitleSwap.previousTitle
  const detailHeaderSwapProgress = detailHeaderTitleSwap.progress
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
  const sourceNotice = ref<{ type: 'running' | 'success' | 'warning'; message: string } | null>(null)
  const sourceTimelinePreloadEnabled = ref(false)
  const detailTransitionRectsLocked = ref(false)
  const detailFeedOriginLocked = ref(false)
  const sourceReturnTargetReady = ref(false)
  const readerBackSwipeCandidateActive = computed(() => readerBackSwipeCandidateTracking.value)
  const readerBackSwipeTrackingActive = computed(() => readerBackSwipeGestureTracking.value)
  const sourceReaderBlockedBackSwipeActive = computed(
    () => readerBackDragging.value && backSwipeTarget.value === 'source' && backSwipeIntent.value === 'blocked',
  )
  const sourceReaderReturnTargetPending = computed(
    () => backSwipeTarget.value === 'source' && !sourceReturnTargetReady.value,
  )
  const readerBackSwipeCanCommitRight = computed(
    () => backSwipeTarget.value === 'detail' || backSwipeTarget.value === 'page',
  )

  const sourceReaderMounted = computed(() => readerSource.value !== null)
  const sourceReaderOpen = computed(() => readerSource.value !== null && sourceReaderVisible.value)
  const detailReaderOpen = computed(
    () => detailItem.value !== null || detailLoading.value || detailError.value !== '',
  )
  const sourceReaderUnderDetail = computed(
    () => detailReaderOpen.value && sourceReaderVisible.value && !hasDetailParkedBehindSource(),
  )
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
  const detailParkedBehindSource = computed(
    () => hasDetailParkedBehindSource() && (!readerBackDragging.value || sourceReaderBlockedBackSwipeActive.value),
  )
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
    if (!detailItem.value || detailCommittedListReturn() || sourceReaderBlockedBackSwipeActive.value) {
      return false
    }

    const entryMorphVisible = detailEntrySettling.value && detailSurfaceProgress.value < 0.985
    return (
      entryMorphVisible ||
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

  function sourceReaderCanRestoreReturnOnCancel() {
    return hasParkedDetailSourceState() || detailRestoringFromSourceReader.value
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

  function durableParkedDetailSnapshot(snapshot: ParkedDetailSnapshot): ParkedDetailSnapshot {
    return {
      item: { ...snapshot.item },
      sourceKind: snapshot.sourceKind,
      originRect: null,
      sourceItemTargetRect: null,
      sourceNameOriginRect: null,
      sourceNameTargetRect: null,
      morphingItemHeight: null,
      scrollTop: snapshot.scrollTop,
    }
  }

  function durableParkedDetailSnapshotFromItem(
    item: FeedItem | null,
    sourceKind: FeedSourceKind,
    scrollTop: number,
  ) {
    if (!item) {
      return null
    }

    return durableParkedDetailSnapshot({
      item,
      sourceKind,
      originRect: null,
      sourceItemTargetRect: null,
      sourceNameOriginRect: null,
      sourceNameTargetRect: null,
      morphingItemHeight: null,
      scrollTop,
    })
  }

  function readerSourceFromDetailItem(item: FeedItem | null, kind: FeedSourceKind): ReaderSource | null {
    if (!item?.source_id) {
      return null
    }

    return {
      id: item.source_id,
      name: item.source_name || '未知来源',
      kind,
    }
  }

  function createReaderStackSessionSnapshot(): ReaderStackSessionSnapshot {
    const currentSourceBackDetail =
      sourceReaderBackDetail.value ??
      snapshotCurrentDetail() ??
      durableParkedDetailSnapshotFromItem(detailItem.value, detailSourceKind.value, detailScrollTop.value)
    const committedListReturn = Boolean(detailListReturnCommitted.value && currentSourceBackDetail)
    const transientSourceReturn = !committedListReturn && sourceReaderReturnMode.value === 'detail'
    const durableSourceReaderVisible = Boolean(
      readerSource.value &&
        (committedListReturn ||
          (sourceReaderVisible.value && (!transientSourceReturn || detailOpenedFromSourceReader.value))),
    )
    const durableSourceBackDetail =
      committedListReturn && currentSourceBackDetail
        ? durableParkedDetailSnapshot(currentSourceBackDetail)
        : null

    return {
      sourceReaderScrollTop: sourceReaderScrollTop.value,
      detailScrollTop: detailScrollTop.value,
      readerSource: readerSource.value ? { ...readerSource.value } : null,
      sourceReaderVisible: durableSourceReaderVisible,
      detailItem: detailItem.value ? { ...detailItem.value } : null,
      detailSourceKind: detailSourceKind.value,
      detailOpenedFromSourceReader: detailOpenedFromSourceReader.value,
      detailListReturnCommitted: committedListReturn,
      detailSourceExitProgress: committedListReturn ? 1 : 0,
      sourceReaderReturnMode: committedListReturn ? 'detail' : null,
      sourceReaderBackDetail: durableSourceBackDetail,
      morphingItemHeight: null,
      parkedDetailStack: parkedDetailStack.value.map(durableParkedDetailSnapshot),
    }
  }

  function applyReaderStackSessionSnapshot(
    snapshot: ReaderSessionSnapshot,
    options: ApplyReaderStackSessionOptions = {},
  ) {
    resetDetailHeaderTitleSwapState()
    const restoredDetailItem = snapshot.detailItem ? { ...snapshot.detailItem } : null
    const restoredDetailSourceKind = snapshot.detailSourceKind || 'subscriptions'
    const restoredReaderSource =
      (snapshot.readerSource ? { ...snapshot.readerSource } : null) ??
      readerSourceFromDetailItem(restoredDetailItem, restoredDetailSourceKind)
    const restoredSourceBackDetailSnapshot =
      snapshot.sourceReaderBackDetail ??
      durableParkedDetailSnapshotFromItem(
        restoredDetailItem,
        restoredDetailSourceKind,
        snapshot.detailScrollTop || 0,
      )
    const restoredListReturnCommitted = Boolean(
      restoredReaderSource && snapshot.detailListReturnCommitted && restoredSourceBackDetailSnapshot,
    )
    const restoredTransientSourceReturn =
      !restoredListReturnCommitted && snapshot.sourceReaderReturnMode === 'detail'
    const restoredSourceReaderVisible = Boolean(
      restoredReaderSource &&
        (restoredListReturnCommitted ||
          (snapshot.sourceReaderVisible &&
            (!restoredTransientSourceReturn || snapshot.detailOpenedFromSourceReader))),
    )
    const restoredSourceBackDetail =
      restoredListReturnCommitted && restoredSourceBackDetailSnapshot
        ? durableParkedDetailSnapshot(restoredSourceBackDetailSnapshot)
        : null

    sourceReaderScrollTop.value = snapshot.sourceReaderScrollTop || 0
    options.onSourceScrollTop?.(sourceReaderScrollTop.value)
    detailScrollTop.value = snapshot.detailScrollTop || 0
    options.onDetailScrollTop?.(detailScrollTop.value)
    readerSource.value = restoredReaderSource
    sourceReaderVisible.value = restoredSourceReaderVisible
    detailItem.value = restoredDetailItem
    detailSourceKind.value = restoredDetailSourceKind
    detailOpenedFromSourceReader.value = Boolean(snapshot.detailOpenedFromSourceReader)
    detailEntryProgress.value = 1
    detailEntrySettling.value = false
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = restoredListReturnCommitted ? 1 : 0
    sourceReaderReturnMode.value = restoredListReturnCommitted ? 'detail' : null
    sourceReaderBackDetail.value = restoredSourceBackDetail
    detailReturningToFeed.value = false
    detailListReturnCommitted.value = restoredListReturnCommitted
    detailRestoringFromSourceReader.value = false
    detailError.value = ''
    detailLoading.value = false
    detailFrameContentHeight.value = 0
    morphingItemId.value = null
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = null
    detailOriginRect.value = null
    clearDetailSourceTransitionTargetState()
    detailFeedOriginLocked.value = false
    parkedDetailStack.value = Array.isArray(snapshot.parkedDetailStack)
      ? snapshot.parkedDetailStack.map(durableParkedDetailSnapshot)
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

    parkedDetailStack.value.push(cloneParkedDetailSnapshot(snapshot))
    return true
  }

  function restoreParkedDetailSnapshot(
    snapshot: ParkedDetailSnapshot | null,
    options: RestoreParkedDetailOptions = {},
  ) {
    if (!snapshot) {
      return false
    }

    const restoredSnapshot = cloneParkedDetailSnapshot(snapshot)
    resetDetailHeaderTitleSwapState()
    detailItem.value = restoredSnapshot.item
    detailError.value = ''
    detailLoading.value = false
    detailSourceKind.value = restoredSnapshot.sourceKind
    detailOpenedFromSourceReader.value = false
    detailOriginRect.value = restoredSnapshot.originRect
    detailSourceItemTargetRect.value = restoredSnapshot.sourceItemTargetRect
    detailSourceNameOriginRect.value = restoredSnapshot.sourceNameOriginRect
    detailSourceNameTargetRect.value = restoredSnapshot.sourceNameTargetRect
    detailScrollTop.value = restoredSnapshot.scrollTop
    options.onDetailScrollTop?.(restoredSnapshot.scrollTop)
    detailFrameContentHeight.value = 0
    morphingItemId.value = null
    morphingHeightLockItemId.value = null
    morphingItemHeight.value = restoredSnapshot.morphingItemHeight
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
    detailTransitionRectsLocked.value = false
    sourceReturnTargetReady.value = false
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

  function clearDetailSourceTransitionTargetState() {
    detailSourceItemTargetRect.value = null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
    sourceReturnTargetReady.value = false
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
    clearDetailSourceTransitionTargetState()
    detailFeedOriginLocked.value = false
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
    clearDetailSourceTransitionTargetState()
    return true
  }

  function clearHiddenSourceCleanupTimer() {
    hiddenSourceCleanupTimerToken += 1
    if (typeof window !== 'undefined' && hiddenSourceCleanupTimer !== 0) {
      window.clearTimeout(hiddenSourceCleanupTimer)
    }
    hiddenSourceCleanupTimer = 0
  }

  function scheduleHiddenSourceReaderCleanupWithDelay(delay = hiddenSourceCleanupDelay) {
    clearHiddenSourceCleanupTimer()
    const token = hiddenSourceCleanupTimerToken
    hiddenSourceCleanupTimer = window.setTimeout(() => {
      if (token !== hiddenSourceCleanupTimerToken) {
        return
      }
      hiddenSourceCleanupTimer = 0
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
      const resolvedVisible = nextVisible || sourceReaderVisible.value
      if (nextVisible) {
        sourceReaderOffset.value = 0
        sourceReaderStretch.value = 0
        if (!detailReaderOpen.value) {
          sourceReaderReturnMode.value = null
          sourceReaderBackDetail.value = null
        }
      }
      sourceReaderVisible.value = resolvedVisible
      return {
        nextVisible: resolvedVisible,
        sourceChanged: false,
        resetScroll: false,
        captureTransition: nextVisible && detailReaderOpen.value,
        loadSubscription: true,
      }
    }

    readerSource.value = source
    if (nextVisible) {
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
    clearDetailSourceTransitionTargetState()
  }

  function clearSourceReaderState() {
    readerSource.value = null
    sourceReaderVisible.value = false
    sourceReaderReturnMode.value = null
    sourceReaderBackDetail.value = null
    sourceReaderOffset.value = 0
    sourceReaderStretch.value = 0
    parkedDetailStack.value = []
    clearDetailSourceTransitionTargetState()
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
    detailRestoringFromSourceReader.value = false
    detailSourceItemTargetRect.value = openedFromSourceReader ? originRect : null
    detailSourceNameOriginRect.value = null
    detailSourceNameTargetRect.value = null
    detailTransitionRectsLocked.value = false
    detailFeedOriginLocked.value = false
    sourceReturnTargetReady.value = false
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
    detailEntryTimerToken += 1
    if (typeof window !== 'undefined' && detailEntryTimer !== 0) {
      window.clearTimeout(detailEntryTimer)
    }
    detailEntryTimer = 0
  }

  function clearDetailEntryFrames() {
    detailEntryFrameToken += 1
    if (typeof window !== 'undefined' && detailEntryFrame !== 0) {
      window.cancelAnimationFrame(detailEntryFrame)
    }
    if (typeof window !== 'undefined' && detailEntrySecondFrame !== 0) {
      window.cancelAnimationFrame(detailEntrySecondFrame)
    }
    detailEntryFrame = 0
    detailEntrySecondFrame = 0
  }

  function clearDetailEntryAsync() {
    clearDetailEntryTimer()
    clearDetailEntryFrames()
  }

  function setDetailEntryTimer(handler: () => void, delay?: number) {
    clearDetailEntryTimer()
    const token = detailEntryTimerToken
    detailEntryTimer = window.setTimeout(() => {
      if (token !== detailEntryTimerToken) {
        return
      }
      detailEntryTimer = 0
      handler()
    }, delay)
  }

  function scheduleDetailEntryFrame(handler: () => void) {
    clearDetailEntryFrames()
    const token = detailEntryFrameToken
    detailEntryFrame = window.requestAnimationFrame(() => {
      if (token !== detailEntryFrameToken) {
        return
      }
      detailEntryFrame = 0
      handler()
    })
  }

  function scheduleDetailEntryDoubleFrame(handler: () => void) {
    clearDetailEntryFrames()
    const token = detailEntryFrameToken
    detailEntryFrame = window.requestAnimationFrame(() => {
      if (token !== detailEntryFrameToken) {
        return
      }
      detailEntryFrame = 0
      detailEntrySecondFrame = window.requestAnimationFrame(() => {
        if (token !== detailEntryFrameToken) {
          return
        }
        detailEntrySecondFrame = 0
        handler()
      })
    })
  }

  function startDetailEntryWithDelay(originRect: RectSnapshot | null, delay: number) {
    clearDetailEntryAsync()
    const result = beginDetailEntryState(originRect)
    if (!result.shouldAnimate) {
      return
    }

    scheduleDetailEntryDoubleFrame(() => {
      commitDetailEntryState()
      setDetailEntryTimer(() => {
        finishDetailEntryState()
      }, delay)
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

  function resetDetailHeaderTitleSwapState() {
    detailHeaderTitleSwap.reset()
  }

  function clearDetailHeaderSwapTimer() {
    detailHeaderTitleSwap.clearTimer()
  }

  function startDetailHeaderTitleSwapWithDelay(nextItem: FeedItem, delay = detailHeaderSwapDelay) {
    detailHeaderTitleSwap.start(nextItem, detailItem.value, delay)
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
      clearDetailSourceTransitionTargetState()
      return false
    }

    detailSourceItemTargetRect.value = itemRect
    morphingItemHeight.value = itemRect.height
    detailSourceNameOriginRect.value = sourceOriginRect
    detailSourceNameTargetRect.value = sourceTargetRect
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
    morphingHeightUnlockTimerToken += 1
    if (typeof window !== 'undefined' && morphingHeightUnlockTimer !== 0) {
      window.clearTimeout(morphingHeightUnlockTimer)
    }
    morphingHeightUnlockTimer = 0
  }

  function restoreMorphingItemContentWithDelay(unlockDelay = morphingItemContentUnlockDelay) {
    beginRestoreMorphingItemContentState()
    clearMorphingHeightUnlockTimer()
    const token = morphingHeightUnlockTimerToken
    morphingHeightUnlockTimer = window.setTimeout(() => {
      if (token !== morphingHeightUnlockTimerToken) {
        return
      }
      morphingHeightUnlockTimer = 0
      finishRestoreMorphingItemContentState()
    }, unlockDelay)
  }

  function revealSourceReaderUnderDetailState() {
    sourceReaderVisible.value = true
  }

  function beginReaderMotionSettlingState() {
    readerBackDragging.value = false
  }

  function clearReaderMotionTimer() {
    readerMotionTimerToken += 1
    if (typeof window !== 'undefined' && readerMotionTimer !== 0) {
      window.clearTimeout(readerMotionTimer)
    }
    readerMotionTimer = 0
  }

  function settleReaderMotionWithDelay(delay = readerMotionSettleDelay, done?: () => void) {
    beginReaderMotionSettlingState()
    clearReaderMotionTimer()
    const token = readerMotionTimerToken
    readerMotionTimer = window.setTimeout(() => {
      if (token !== readerMotionTimerToken) {
        return
      }
      readerMotionTimer = 0
      done?.()
    }, delay)
  }

  function clearReaderStackTimers() {
    clearReaderMotionTimer()
    clearDetailEntryAsync()
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
    resetDetailHeaderTitleSwapState()
    detailOpenedFromSourceReader.value = false
    detailRestoringFromSourceReader.value = false
    detailScrollTop.value = 0
    detailScrollHeight.value = 0
    detailScrollClientHeight.value = 0
    detailFrameContentHeight.value = 0
    detailProgressDragging.value = false
    detailReaderTouchOffset.value = 0
    detailReaderStretch.value = 0
    clearDetailEntryAsync()
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
    clearDetailEntryAsync()
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
    clearDetailEntryAsync()
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
    clearDetailSourceTransitionTargetState()
  }

  function restoreDetailFromSourceSwipeWithDelay(delay: number) {
    beginRestoreDetailFromSourceSwipeState()
    clearDetailEntryAsync()
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
      const currentDetailSnapshot = snapshotCurrentDetail()
      sourceReaderBackDetail.value = currentDetailSnapshot
        ? cloneParkedDetailSnapshot(currentDetailSnapshot)
        : null
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
    clearDetailSourceTransitionTargetState()
  }

  function completeDetailToSourceReaderWithDelay(
    delay: number,
    options: CompleteDetailToSourceReaderTransitionOptions = {},
  ) {
    beginCompleteDetailToSourceReaderState()
    options.afterBegin?.()
    clearDetailEntryAsync()
    scheduleDetailEntryDoubleFrame(() => {
      commitCompleteDetailToSourceReaderState()
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
    clearDetailSourceTransitionTargetState()
  }

  function restoreParkedSourceReaderWithDelay(delay: number) {
    if (!beginRestoreParkedSourceReaderState()) {
      return false
    }

    clearDetailEntryAsync()
    scheduleDetailEntryFrame(() => {
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
    clearDetailSourceTransitionTargetState()
  }

  function restoreDetailFromParkedSourceWithDelay(
    delay: number,
    options: RestoreDetailFromParkedSourceTransitionOptions = {},
  ) {
    clearDetailEntryAsync()
    options.beforeBegin?.()
    if (!beginRestoreDetailFromParkedSourceState()) {
      return false
    }
    options.afterBegin?.()

    scheduleDetailEntryFrame(() => {
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
    readerBackSwipeCandidateTracking.value = false
    readerBackSwipeGestureTracking.value = false
    backSwipeTarget.value = null
    backSwipeIntent.value = null
    detailReaderTouchOffset.value = 0
    detailBackExitProgress.value = 0
    detailSourceExitProgress.value = keepDetailParkedBehindSource ? 1 : 0
    detailRestoringFromSourceReader.value = false
    detailReturningToFeed.value = false
    detailReaderStretch.value = 0
    sourceReaderStretch.value = 0
    sourceReaderOffset.value = 0
    readerBackDragging.value = false
    clearDetailSourceTransitionTargetState()
  }

  function resetReaderBackSwipeDragState() {
    resetReaderBackSwipeState()
  }

  function resetReaderBackSwipeTargetState() {
    backSwipeTarget.value = null
    backSwipeIntent.value = null
  }

  function resetReaderBackSwipeCandidateState() {
    readerBackSwipeCandidateTracking.value = false
    readerBackSwipeGestureTracking.value = false
    resetReaderBackSwipeTargetState()
    clearSourceReturnTargetReadyState()
  }

  function setReaderBackSwipeTargetState(target: ReaderBackSwipeTarget) {
    backSwipeTarget.value = target
  }

  function beginReaderBackSwipeCandidateState(target: Exclude<ReaderBackSwipeTarget, null>) {
    readerBackSwipeCandidateTracking.value = true
    readerBackSwipeGestureTracking.value = false
    setReaderBackSwipeTargetState(target)
    backSwipeIntent.value = null
  }

  function setReaderBackSwipeIntentState(intent: ReaderBackSwipeIntent) {
    backSwipeIntent.value = intent
  }

  function readerBackSwipeMatches(
    target: ActiveReaderBackSwipeTarget,
    intent?: ActiveReaderBackSwipeIntent,
  ) {
    return backSwipeTarget.value === target && (intent === undefined || backSwipeIntent.value === intent)
  }

  function readerBackSwipeReturningToFeed() {
    return readerBackSwipeMatches('detail') && !detailOpenedFromSourceReader.value
  }

  function readerBackSwipeRevealsSourceReader() {
    return readerBackSwipeMatches('detail') && detailOpenedFromSourceReader.value && readerSource.value !== null
  }

  function readerBackSwipeCanOpenSourceFromDetail() {
    return readerBackSwipeMatches('detail') && Boolean(detailItem.value?.source_id) && !detailOpenedFromSourceReader.value
  }

  function readerBackSwipeCanReturnSourceToDetail(deltaX: number) {
    return readerBackSwipeMatches('source') && deltaX > 0 && sourceReaderCanReturnToDetail()
  }

  function readerBackSwipeIntentAction(deltaX: number): ReaderBackSwipeIntentAction {
    if (readerBackSwipeCanReturnSourceToDetail(deltaX)) {
      return { type: 'source-return' }
    }
    if (deltaX > 0) {
      return {
        type: 'right-swipe',
        intent: readerBackSwipeCanCommitRight.value ? 'back' : 'blocked',
        returningToFeed: readerBackSwipeReturningToFeed(),
        revealSourceReader: readerBackSwipeRevealsSourceReader(),
      }
    }
    if (readerBackSwipeCanOpenSourceFromDetail()) {
      return { type: 'detail-source' }
    }
    return { type: 'blocked' }
  }

  function applyReaderBackSwipeIntentState(
    deltaX: number,
    options: ApplyReaderBackSwipeIntentStateOptions = {},
  ) {
    const action = readerBackSwipeIntentAction(deltaX)
    if (action.type === 'source-return') {
      options.beforeSourceReturnIntent?.()
      setReaderBackSwipeIntentState('back')
      options.afterSourceReturnIntent?.()
      prepareReaderBackSwipeIntentState({ intent: 'source-return' })
      return action
    }

    if (action.type === 'right-swipe') {
      if (action.intent === 'blocked') {
        setReaderBackSwipeIntentState('blocked')
        if (options.prepareBlocked) {
          prepareReaderBackSwipeIntentState({ intent: 'blocked', clearReturningToFeed: true })
        }
        return action
      }

      const effectState = {
        returningToFeed: action.returningToFeed,
        revealSourceReader: action.revealSourceReader,
      }
      setReaderBackSwipeIntentState(action.intent)
      options.beforeDetailBackPrepare?.(effectState)
      prepareReaderBackSwipeIntentState({
        intent: 'detail-back',
        returningToFeed: action.returningToFeed,
        revealSourceReader: action.revealSourceReader,
        ...(options.resetSourceExit ? { resetSourceExit: true } : {}),
      })
      options.afterDetailBackPrepare?.(effectState)
      return action
    }

    if (action.type === 'detail-source') {
      setReaderBackSwipeIntentState('source')
      options.afterDetailSourceIntent?.()
      return action
    }

    setReaderBackSwipeIntentState('blocked')
    if (options.prepareBlocked) {
      prepareReaderBackSwipeIntentState({ intent: 'blocked', clearReturningToFeed: true })
    }
    return action
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

  function readerBackSwipeVisualAction(offset: number, stretch: number, width: number): ReaderBackSwipeVisualAction {
    const intent = backSwipeIntent.value
    const target = backSwipeTarget.value
    const progressBase = Math.max(220, width * 0.52)
    if (intent === 'back' && target === 'detail') {
      return {
        type: 'reader',
        state: {
          target: 'detail-back',
          progress: clampProgress(Math.max(0, offset) / progressBase),
        },
      }
    }
    if (intent === 'back' && target === 'source') {
      if (offset > 0 && sourceReaderCanReturnToDetail()) {
        return {
          type: 'reader',
          state: {
            target: 'source-return',
            returnProgress: clampProgress(Math.max(0, offset) / progressBase),
          },
        }
      }
      return {
        type: 'reader',
        state: { target: 'source-blocked', stretch },
      }
    }
    if (intent === 'back' && target === 'page') {
      return { type: 'page', stretch }
    }
    if (intent === 'source' && target === 'detail') {
      return {
        type: 'reader',
        state: {
          target: 'detail-source',
          progress: clampProgress(Math.max(0, -offset) / progressBase),
        },
      }
    }
    if (intent === 'blocked' && target === 'detail') {
      return {
        type: 'reader',
        state: { target: 'detail-blocked', stretch },
      }
    }
    if (intent === 'blocked' && target === 'source') {
      return {
        type: 'reader',
        state: { target: 'source-blocked', stretch },
      }
    }
    if (intent === 'blocked' && target === 'page') {
      return { type: 'page', stretch }
    }
    return { type: 'none' }
  }

  function readerBackSwipeShouldCommit(deltaX: number, switchDistance: number) {
    const target = backSwipeTarget.value
    const intent = backSwipeIntent.value
    if (intent === 'back' && target === 'detail') {
      return deltaX > 0 && (detailBackExitProgress.value >= 0.42 || deltaX >= switchDistance)
    }
    if (intent === 'back' && target === 'source') {
      return deltaX > 0 && (detailSourceExitProgress.value <= 0.58 || deltaX >= switchDistance)
    }
    if (intent === 'source' && target === 'detail') {
      return deltaX < 0 && (detailSourceExitProgress.value >= 0.42 || Math.abs(deltaX) >= switchDistance)
    }
    if (intent === 'back') {
      return deltaX > 0 && Math.abs(deltaX) >= switchDistance
    }
    return false
  }

  function readerBackSwipeIsBlocked() {
    return backSwipeIntent.value === 'blocked'
  }

  function readerBackSwipeFinishAction(committed: boolean): ReaderBackSwipeFinishAction {
    const target = backSwipeTarget.value
    const intent = backSwipeIntent.value

    if (!committed) {
      if (intent === 'back' && target === 'detail') {
        return 'restore-item-expansion'
      }
      if (intent === 'source' && target === 'detail') {
        return 'restore-detail-from-source-swipe'
      }
      if (intent === 'back' && target === 'source' && sourceReaderCanReturnToDetail()) {
        return 'restore-parked-source'
      }
      return 'reset'
    }

    if (intent === 'source' && target === 'detail') {
      return 'complete-detail-to-source'
    }
    if (intent === 'back' && target === 'detail') {
      return 'collapse-detail'
    }
    if (intent === 'back' && target === 'source') {
      return sourceReaderCanReturnToDetail() ? 'restore-detail-from-parked-source' : 'reset'
    }
    if (intent === 'back' && target === 'page') {
      return 'return-page'
    }
    return 'reset'
  }

  function readerBackSwipeFinishResult(
    deltaX: number,
    switchDistance: number,
    fallbackStretch = 0,
  ): ReaderBackSwipeFinishResult {
    const committed = readerBackSwipeShouldCommit(deltaX, switchDistance)
    return {
      committed,
      progress: committed ? 1 : readerBackSwipeTransitionProgress(fallbackStretch),
      isBlocked: readerBackSwipeIsBlocked(),
      action: readerBackSwipeFinishAction(committed),
    }
  }

  function readerBackSwipeCancelAction(): ReaderBackSwipeCancelAction {
    const target = backSwipeTarget.value
    const intent = backSwipeIntent.value
    if (intent === 'back' && target === 'detail') {
      return 'restore-item-expansion'
    }
    if (intent === 'source' && target === 'detail') {
      return 'restore-detail-from-source-swipe'
    }
    if (intent === 'back' && target === 'source' && sourceReaderCanRestoreReturnOnCancel()) {
      return 'restore-parked-source'
    }
    return 'reset'
  }

  function readerBackSwipeCancelResult(fallbackStretch = 0): ReaderBackSwipeCancelResult {
    return {
      progress: readerBackSwipeTransitionProgress(fallbackStretch),
      isBlocked: readerBackSwipeIsBlocked(),
      action: readerBackSwipeCancelAction(),
    }
  }

  function applyReaderBackSwipeAction(
    action: ReaderBackSwipeAction,
    handlers: ReaderBackSwipeActionHandlers,
  ) {
    if (action === 'restore-item-expansion') {
      handlers.restoreItemExpansion()
      return
    }
    if (action === 'restore-detail-from-source-swipe') {
      handlers.restoreDetailFromSourceSwipe()
      return
    }
    if (action === 'restore-parked-source') {
      handlers.restoreParkedSource()
      return
    }
    if (action === 'complete-detail-to-source') {
      handlers.completeDetailToSource()
      return
    }
    if (action === 'collapse-detail') {
      handlers.collapseDetail()
      return
    }
    if (action === 'restore-detail-from-parked-source') {
      handlers.restoreDetailFromParkedSource()
      return
    }
    if (action === 'return-page') {
      handlers.reset()
      handlers.returnPage()
      return
    }
    handlers.reset()
  }

  function readerBackSwipeTransitionSurfaces<TSurface extends string>(surfaces: {
    activeFeedSurface: TSurface
    pageReturnSurface: TSurface
  }) {
    const target = backSwipeTarget.value
    const intent = backSwipeIntent.value
    const from: ReaderBackSwipeSurface =
      target === 'source' ? 'reader:source' : target === 'page' ? 'page:management' : 'reader:detail'
    let to: ReaderBackSwipeSurface | TSurface | null = null

    if (intent !== 'blocked' && target) {
      if (intent === 'source') {
        to = 'reader:source'
      } else if (target === 'source') {
        to = 'reader:detail'
      } else if (target === 'page') {
        to = surfaces.pageReturnSurface
      } else {
        to = detailOpenedFromSourceReader.value ? 'reader:source' : surfaces.activeFeedSurface
      }
    }

    return {
      from,
      to,
      isBlocked: intent === 'blocked',
    }
  }

  function readerBackSwipeTransitionBeginPayload<TSurface extends string>(
    deltaX: number,
    surfaces: {
      activeFeedSurface: TSurface
      pageReturnSurface: TSurface
    },
  ) {
    const transitionSurfaces = readerBackSwipeTransitionSurfaces(surfaces)
    return {
      from: transitionSurfaces.from,
      to: transitionSurfaces.to,
      direction: deltaX < 0 ? ('left' as const) : ('right' as const),
      isBlocked: transitionSurfaces.isBlocked,
    }
  }

  function readerBackSwipeTransitionUpdatePayload<TSurface extends string>(
    deltaX: number,
    fallbackStretch: number,
    surfaces: {
      activeFeedSurface: TSurface
      pageReturnSurface: TSurface
    },
  ) {
    const transitionSurfaces = readerBackSwipeTransitionSurfaces(surfaces)
    return {
      to: transitionSurfaces.to,
      direction: deltaX < 0 ? ('left' as const) : ('right' as const),
      progress: readerBackSwipeTransitionProgress(fallbackStretch),
      isBlocked: transitionSurfaces.isBlocked,
    }
  }

  function beginReaderBackSwipeTrackingState() {
    readerBackSwipeCandidateTracking.value = false
    readerBackSwipeGestureTracking.value = true
    detailEntrySettling.value = false
    clearDetailSourceTransitionTargetState()
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

  function beginReaderBackSwipeDragState(
    deltaX: number,
    options: ApplyReaderBackSwipeIntentStateOptions = {},
  ) {
    beginReaderBackSwipeTrackingState()
    const action = applyReaderBackSwipeIntentState(deltaX, options)
    startReaderBackSwipeDragState()
    return action
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

  function applyReaderBackSwipeVisualActionState(
    offset: number,
    stretch: number,
    width: number,
    options: ApplyReaderBackSwipeVisualActionOptions = {},
  ) {
    const action = readerBackSwipeVisualAction(offset, stretch, width)
    resetReaderBackSwipeStretchState()
    options.resetPageStretch?.()

    if (action.type === 'reader') {
      applyReaderBackSwipeVisualState(action.state)
    } else if (action.type === 'page') {
      options.resetPageOffset?.()
      options.applyPageStretch?.(action.stretch)
    }

    return action
  }

  function readerBackSwipeVisualOffset(deltaX: number, width: number) {
    const limit = Math.round(Math.max(1, width) * 0.72)
    return Math.max(-limit, Math.min(limit, deltaX))
  }

  function readerBackSwipeBlockedStretch(deltaX: number, metrics: ReaderBackSwipeDragMetrics) {
    const width = Math.max(1, metrics.width)
    const edgeStopZone = Math.min(54, width * 0.12)
    const availableDistance =
      deltaX < 0
        ? Math.max(1, metrics.startX - edgeStopZone)
        : Math.max(1, width - metrics.startX - edgeStopZone)
    const travelledToEdge =
      deltaX < 0
        ? Math.max(0, metrics.startX - metrics.currentX)
        : Math.max(0, metrics.currentX - metrics.startX)
    const edgeProgress = clampProgress(travelledToEdge / availableDistance)
    const distanceProgress = Math.log1p(edgeProgress * 14) / Math.log1p(14)
    const stretch = 0.07 * distanceProgress
    return deltaX < 0 ? -stretch : stretch
  }

  function updateReaderBackSwipeDragState(
    deltaX: number,
    metrics: ReaderBackSwipeDragMetrics,
    options: UpdateReaderBackSwipeDragStateOptions = {},
  ) {
    const offset = readerBackSwipeVisualOffset(deltaX, metrics.width)
    const stretch = readerBackSwipeBlockedStretch(deltaX, metrics)
    const intentAction = applyReaderBackSwipeIntentState(deltaX, options.intent)
    const visualAction = applyReaderBackSwipeVisualActionState(
      offset,
      stretch,
      metrics.width,
      options.visual,
    )
    return {
      intentAction,
      visualAction,
    }
  }

  function detailBlocksGestures() {
    return detailReaderOpen.value && !detailCommittedListReturn()
  }

  function setSourceReaderContentElement(element: HTMLElement | null) {
    sourceReaderContentRef.value = element
  }

  function setDetailContentElement(element: HTMLElement | null) {
    detailContentRef.value = element
  }

  function setDetailInlineSourceElement(element: HTMLElement | null) {
    detailInlineSourceRef.value = element
  }

  function setDetailFrameElement(element: HTMLIFrameElement | null) {
    detailFrameRef.value = element
  }

  function scrollSourceReaderContentTo(scrollTop: number) {
    if (!sourceReaderContentRef.value) {
      return false
    }
    sourceReaderContentRef.value.scrollTop = scrollTop
    return true
  }

  function scrollDetailContentTo(scrollTop: number) {
    if (!detailContentRef.value) {
      return false
    }
    detailContentRef.value.scrollTop = scrollTop
    return true
  }

  function setSourceTimelinePreloadEnabledState(enabled: boolean) {
    sourceTimelinePreloadEnabled.value = enabled
  }

  function clearReaderStretchAnchorsIfIdle() {
    if (!readerBackDragging.value && detailReaderStretch.value === 0) {
      detailStretchAnchor.value = null
    }
    if (!readerBackDragging.value && sourceReaderStretch.value === 0) {
      sourceStretchAnchor.value = null
    }
  }

  function setSourceCatalogEntryState(entry: SourceCatalogEntry | null) {
    sourceCatalogEntry.value = entry
  }

  function setSourceSubscriptionState(source: Source | null) {
    sourceSubscription.value = source
  }

  function setSourceSubscriptionLoadingState(loading: boolean) {
    sourceSubscriptionLoading.value = loading
  }

  function setSourceNoticeState(notice: { type: 'running' | 'success' | 'warning'; message: string } | null) {
    sourceNotice.value = notice
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
    readerBackSwipeCandidateActive,
    readerBackSwipeTrackingActive,
    readerBackDragging,
    sourceReaderBlockedBackSwipeActive,
    sourceReaderReturnTargetPending,
    readerSource,
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
    sourceReaderCanRestoreReturnOnCancel,
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
    settleReaderMotionWithDelay,
    clearReaderStackTimers,
    updateSourceReaderScrollTopState,
    updateDetailScrollMetricsState,
    updateDetailScrollTopState,
    updateDetailFrameContentHeightState,
    setDetailProgressDraggingState,
    closeItemReaderWithTransition,
    collapseItemReaderWithDelay,
    restoreItemReaderExpansionWithDelay,
    restoreDetailFromSourceSwipeWithDelay,
    completeDetailToSourceReaderWithDelay,
    restoreParkedSourceReaderWithDelay,
    restoreDetailFromParkedSourceWithDelay,
    resetReaderBackSwipeDragState,
    resetReaderBackSwipeCandidateState,
    beginReaderBackSwipeCandidateState,
    updateReaderBackSwipeDragState,
    readerBackSwipeFinishResult,
    readerBackSwipeCancelResult,
    applyReaderBackSwipeAction,
    readerBackSwipeTransitionBeginPayload,
    readerBackSwipeTransitionUpdatePayload,
    beginReaderBackSwipeDragState,
    detailBlocksGestures,
    setSourceReaderContentElement,
    setDetailContentElement,
    setDetailInlineSourceElement,
    setDetailFrameElement,
    scrollSourceReaderContentTo,
    scrollDetailContentTo,
    setSourceTimelinePreloadEnabledState,
    clearReaderStretchAnchorsIfIdle,
    setSourceCatalogEntryState,
    setSourceSubscriptionState,
    setSourceSubscriptionLoadingState,
    setSourceNoticeState,
  }
}
