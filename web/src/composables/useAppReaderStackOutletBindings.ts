import { computed } from 'vue'
import type { StyleValue } from 'vue'

import type { FeedItem } from '@/api/feed'
import type { AppReaderMorphVisibilityState } from '@/composables/useAppReaderMorphVisibilityState'
import type { AppReaderMotionState } from '@/composables/useAppReaderMotionState'
import type { ChromePhase } from '@/composables/useChromeState'
import type { FeedSourceKind, ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type SourceNotice = {
  type: 'running' | 'success' | 'warning'
  message: string
}

type AppReaderStackOutletBindingOptions = {
  sourceReaderMounted: ReadableRef<boolean>
  sourceReaderUnderDetail: ReadableRef<boolean>
  readerMotion: AppReaderMotionState
  readerSource: ReadableRef<ReaderSource | null>
  sourceToggleActive: ReadableRef<boolean>
  detailItem: ReadableRef<FeedItem | null>
  readerMorph: AppReaderMorphVisibilityState
  detailReaderOpen: ReadableRef<boolean>
  detailParkedBehindSource: ReadableRef<boolean>
  sourceNotice: ReadableRef<SourceNotice | null>
  topChromePhase: ReadableRef<ChromePhase>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  sourceHeaderStyle: ReadableRef<StyleValue>
  sourceMainLayerStyle: ReadableRef<StyleValue>
  sourcePullStatusStyle: ReadableRef<StyleValue>
  sourcePullIconStyle: ReadableRef<StyleValue>
  sourcePullActive: ReadableRef<boolean>
  sourcePullRefreshing: () => boolean
  pullStatusText: ReadableRef<string>
  pullStatusMeta: ReadableRef<string>
  sourceToggleLabel: ReadableRef<string>
  sourceToggleDisabled: ReadableRef<boolean>
  sourceReaderScrollTop: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  morphingItemId: ReadableRef<number | null>
  morphingHeightLockItemId: ReadableRef<number | null>
  morphingItemHeight: ReadableRef<number | null>
  feedItemPreviewProgress: ReadableRef<number>
  sourceReaderVisible: ReadableRef<boolean>
  detailLoading: ReadableRef<boolean>
  detailError: ReadableRef<string>
  detailProgressVisible: ReadableRef<boolean>
  detailReadingProgress: ReadableRef<number>
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
    const readerMotion = options.readerMotion
    const sourceInteractive = options.sourceReaderVisible.value && !options.sourceReaderUnderDetail.value
    const sourceHeaderStyle = options.sourceHeaderStyle.value as Record<string, unknown> | undefined
    const sourceChromeInteractive = sourceHeaderStyle?.pointerEvents === 'auto'

    return {
      sourceMounted: options.sourceReaderMounted.value && Boolean(readerSource),
      sourceInteractive,
      sourceUnderDetail: options.sourceReaderUnderDetail.value,
      sourceStyle: readerMotion.sourceReaderStyle.value,
      sourceTitleRevealMounted: readerMorph.sourceTitleRevealMounted.value,
      sourceTitleRevealStyle: readerMorph.sourceTitleRevealStyle.value,
      sourceTitle: readerMorph.sourceTitle.value,
      sourceMeta: readerMorph.sourceMeta.value,
      sourceNameMorphMounted: readerMorph.sourceNameMorphMounted.value,
      sourceNameMorphStyle: readerMorph.sourceNameMorphStyle.value,
      sourceNameMorphText: readerMorph.sourceNameMorphText.value,
      detailOpen: options.detailReaderOpen.value,
      detailInteractive: options.detailReaderOpen.value && !options.detailParkedBehindSource.value,
      detailStyle: readerMotion.detailReaderStyle.value,
      detailBackdropStyle: readerMotion.detailBackdropStyle.value,
      sourceNotice: sourceInteractive ? options.sourceNotice.value : null,
      topChromePhase: options.topChromePhase.value,
      topChromeProgress: options.topChromeProgress.value,
      topChromeContentCollapsed: options.feedContentCollapsed.value,
      sourceHeaderStyle: options.sourceHeaderStyle.value,
      sourceChromeInteractive,
      sourceName,
      sourceTitleTextStyle: readerMotion.sourceTitleTextStyle.value,
      sourceTitleLayerStyle: readerMotion.sourceTitleLayerStyle.value,
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
      sourceContentStyle: readerMotion.sourceContentStyle.value,
      readerSource,
      sourceScrollTop: options.sourceReaderScrollTop.value,
      feedHeaderHeight: options.feedHeaderHeight.value,
      morphingItemId: options.morphingItemId.value,
      morphingHeightLockItemId: options.morphingHeightLockItemId.value,
      morphingItemHeight: options.morphingItemHeight.value,
      feedItemPreviewProgress: options.feedItemPreviewProgress.value,
      sourceBackgroundRefresh: !sourceInteractive,
      detailTransitionStyle: readerMotion.detailTransitionSurfaceStyle.value,
      detailItem,
      detailMorphVisible: readerMorph.detailMorphVisible.value,
      detailMorphTextStyle: readerMorph.detailMorphTextStyle.value,
      detailMorphMetaStyle: readerMorph.detailMorphMetaStyle.value,
      detailMorphTitleStyle: readerMorph.detailMorphTitleStyle.value,
      detailMorphSummaryStyle: readerMorph.detailMorphSummaryStyle.value,
      detailMorphSourceLabelStyle: readerMorph.detailMorphSourceLabelStyle.value,
      detailDisplayDate: readerMorph.detailDisplayDate.value,
      detailMorphSummaryVisible: readerMorph.detailMorphSummaryVisible.value,
      detailPreviewSummary: readerMorph.detailPreviewSummary.value,
      detailContentStyle: readerMotion.detailContentStyle.value,
      detailInlineMetaStyle: readerMotion.detailInlineMetaStyle.value,
      detailFrameStyle: readerMotion.detailFrameStyle.value,
      detailActionsStyle: readerMotion.detailActionsStyle.value,
      detailLoading: options.detailLoading.value,
      detailError: options.detailError.value,
      detailSrcdoc: readerMotion.detailSrcdoc.value,
      detailInlineSourceStyle: readerMotion.detailInlineSourceStyle.value,
      detailProgressVisible: options.detailProgressVisible.value,
      detailReadingProgress: options.detailReadingProgress.value,
      detailProgressStyle: readerMotion.detailProgressStyle.value,
      detailProgressFillStyle: readerMotion.detailProgressFillStyle.value,
      detailProgressThumbStyle: readerMotion.detailProgressThumbStyle.value,
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
