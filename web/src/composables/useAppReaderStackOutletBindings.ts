import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppReaderMorphVisibilityState } from '@/composables/useAppReaderMorphVisibilityState'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type SourceNotice = {
  type: 'success' | 'warning'
  message: string
}

type AppReaderStackOutletBindingOptions = {
  sourceReaderMounted: ReadableRef<boolean>
  sourceReaderUnderDetail: ReadableRef<boolean>
  sourceReaderStyle: ReadableRef<StyleValue>
  readerSource: ReadableRef<ReaderSource | null>
  sourceToggleActive: ReadableRef<boolean>
  detailItem: ReadableRef<FeedItem | null>
  readerMorph: AppReaderMorphVisibilityState
  detailReaderOpen: ReadableRef<boolean>
  readerMotionSettling: ReadableRef<boolean>
  detailReturningToFeed: ReadableRef<boolean>
  detailReaderStyle: ReadableRef<StyleValue>
  sourceNotice: ReadableRef<SourceNotice | null>
  topChromePhase: ReadableRef<ChromePhase>
  topChromeProgress: ReadableRef<number>
  sourceHeaderStyle: ReadableRef<StyleValue>
  sourceTitleTextStyle: ReadableRef<StyleValue>
  sourceTitleLayerStyle: ReadableRef<StyleValue>
  sourceMainLayerStyle: ReadableRef<StyleValue>
  sourcePullStatusStyle: ReadableRef<StyleValue>
  sourcePullIconStyle: ReadableRef<StyleValue>
  sourcePullActive: ReadableRef<boolean>
  sourcePullRefreshing: () => boolean
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  sourceToggleLabel: ReadableRef<string>
  sourceToggleDisabled: ReadableRef<boolean>
  sourceContentStyle: ReadableRef<StyleValue>
  sourceReaderScrollTop: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  morphingItemId: ReadableRef<number | null>
  morphingHeightLockItemId: ReadableRef<number | null>
  morphingItemHeight: ReadableRef<number | null>
  feedItemPreviewProgress: ReadableRef<number>
  sourceReaderVisible: ReadableRef<boolean>
  detailEntrySettling: ReadableRef<boolean>
  feedChromeSettling: ReadableRef<boolean>
  detailTransitionSurfaceStyle: ReadableRef<StyleValue>
  detailContentStyle: ReadableRef<StyleValue>
  detailLoading: ReadableRef<boolean>
  detailError: ReadableRef<string>
  detailSrcdoc: ReadableRef<string>
  detailInlineSourceStyle: ReadableRef<StyleValue>
  detailProgressVisible: ReadableRef<boolean>
  detailProgressDragging: ReadableRef<boolean>
  detailReadingProgress: ReadableRef<number>
  detailProgressStyle: ReadableRef<StyleValue>
  detailProgressFillStyle: ReadableRef<StyleValue>
  detailProgressThumbStyle: ReadableRef<StyleValue>
  setSourceReaderContentElement: (element: HTMLElement | null) => void
  handleSourceReaderScroll: (event: Event) => void
  openNavigation: () => void
  toggleSourceReaderSubscription: () => void
  handleFeedTopPullStart: (startedWithVisibleChrome: boolean) => void
  handleFeedTopPullMove: (distance: number) => void
  handleFeedTopPullEnd: (shouldRefresh: boolean) => void
  openItemReader: (item: FeedItem, sourceKind: FeedSourceKind, originRect?: DOMRect) => void
  setDetailContentElement: (element: HTMLElement | null) => void
  handleDetailContentScroll: (event: Event) => void
  setDetailInlineSourceElement: (element: HTMLElement | null) => void
  setDetailFrameElement: (element: HTMLIFrameElement | null) => void
  handleDetailFrameLoad: () => void
  handleDetailProgressDragStart: () => void
  handleDetailProgressDragEnd: () => void
  handleDetailProgressChange: (progress: number) => void
}

export function useAppReaderStackOutletBindings(options: AppReaderStackOutletBindingOptions) {
  const props = computed(() => {
    const readerSource = options.readerSource.value
    const detailItem = options.detailItem.value
    const sourceName = readerSource?.name || ''
    const readerMorph = options.readerMorph

    return {
      sourceMounted: options.sourceReaderMounted.value && Boolean(readerSource),
      sourceUnderDetail: options.sourceReaderUnderDetail.value,
      sourceStyle: options.sourceReaderStyle.value,
      sourceTitleRevealMounted: readerMorph.sourceTitleRevealMounted.value,
      sourceTitleRevealVisible: readerMorph.sourceTitleRevealVisible.value,
      sourceTitleRevealStyle: readerMorph.sourceTitleRevealStyle.value,
      sourceTitle: readerMorph.sourceTitle.value,
      sourceMeta: readerMorph.sourceMeta.value,
      sourceNameMorphMounted: readerMorph.sourceNameMorphMounted.value,
      sourceNameMorphVisible: readerMorph.sourceNameMorphVisible.value,
      sourceNameMorphStyle: readerMorph.sourceNameMorphStyle.value,
      sourceNameMorphText: readerMorph.sourceNameMorphText.value,
      detailOpen: options.detailReaderOpen.value,
      detailMotionSettling: options.readerMotionSettling.value,
      detailReturningFeed: options.detailReturningToFeed.value,
      detailStyle: options.detailReaderStyle.value,
      sourceNotice: options.sourceNotice.value,
      topChromePhase: options.topChromePhase.value,
      topChromeProgress: options.topChromeProgress.value,
      sourceHeaderStyle: options.sourceHeaderStyle.value,
      sourceName,
      sourceTitleTextStyle: options.sourceTitleTextStyle.value,
      sourceTitleLayerStyle: options.sourceTitleLayerStyle.value,
      sourceMainLayerStyle: options.sourceMainLayerStyle.value,
      sourcePullStatusStyle: options.sourcePullStatusStyle.value,
      sourcePullIconStyle: options.sourcePullIconStyle.value,
      sourcePullActive: options.sourcePullActive.value,
      sourcePullRefreshing: options.sourcePullRefreshing(),
      pullStatusText: options.pullStatusText.value,
      pullStatusMeta: options.pullStatusMeta.value,
      sourceToggleActive: options.sourceToggleActive.value,
      sourceToggleLabel: options.sourceToggleLabel.value,
      sourceToggleDisabled: options.sourceToggleDisabled.value,
      sourceContentStyle: options.sourceContentStyle.value,
      readerSource,
      sourceScrollTop: options.sourceReaderScrollTop.value,
      feedHeaderHeight: options.feedHeaderHeight.value,
      morphingItemId: options.morphingItemId.value,
      morphingHeightLockItemId: options.morphingHeightLockItemId.value,
      morphingItemHeight: options.morphingItemHeight.value,
      feedItemPreviewProgress: options.feedItemPreviewProgress.value,
      sourceBackgroundRefresh: !options.sourceReaderVisible.value,
      detailEntrySettling: options.detailEntrySettling.value,
      detailChromeSettling: options.feedChromeSettling.value,
      detailTransitionStyle: options.detailTransitionSurfaceStyle.value,
      detailItem,
      detailMorphVisible: readerMorph.detailMorphVisible.value,
      detailMorphTextStyle: readerMorph.detailMorphTextStyle.value,
      detailMorphSourceLabelStyle: readerMorph.detailMorphSourceLabelStyle.value,
      detailDisplayDate: readerMorph.detailDisplayDate.value,
      detailMorphSummaryVisible: readerMorph.detailMorphSummaryVisible.value,
      detailPreviewSummary: readerMorph.detailPreviewSummary.value,
      detailContentStyle: options.detailContentStyle.value,
      detailLoading: options.detailLoading.value,
      detailError: options.detailError.value,
      detailSrcdoc: options.detailSrcdoc.value,
      detailInlineSourceStyle: options.detailInlineSourceStyle.value,
      detailProgressVisible: options.detailProgressVisible.value,
      detailProgressDragging: options.detailProgressDragging.value,
      detailReadingProgress: options.detailReadingProgress.value,
      detailProgressStyle: options.detailProgressStyle.value,
      detailProgressFillStyle: options.detailProgressFillStyle.value,
      detailProgressThumbStyle: options.detailProgressThumbStyle.value,
    }
  })

  const listeners = {
    'source-content-ref': options.setSourceReaderContentElement,
    'source-content-scroll': options.handleSourceReaderScroll,
    'open-navigation': options.openNavigation,
    'toggle-source-subscription': options.toggleSourceReaderSubscription,
    'top-pull-start': options.handleFeedTopPullStart,
    'top-pull-move': options.handleFeedTopPullMove,
    'top-pull-end': options.handleFeedTopPullEnd,
    'open-item': options.openItemReader,
    'detail-content-ref': options.setDetailContentElement,
    'detail-content-scroll': options.handleDetailContentScroll,
    'detail-inline-source-ref': options.setDetailInlineSourceElement,
    'detail-frame-ref': options.setDetailFrameElement,
    'detail-frame-load': options.handleDetailFrameLoad,
    'detail-progress-drag-start': options.handleDetailProgressDragStart,
    'detail-progress-drag-end': options.handleDetailProgressDragEnd,
    'detail-progress-change': options.handleDetailProgressChange,
  }

  return {
    props,
    listeners,
  }
}
