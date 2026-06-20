import { computed, ref } from 'vue'

import type { FeedItem, Source, SourceCatalogEntry } from '@/api/feed'
import type {
  FeedSourceKind,
  ParkedDetailSnapshot,
  ReaderSource,
  RectSnapshot,
} from '@/composables/useReaderSession'

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
  }
}
