import { computed } from 'vue'
import type { Ref, StyleValue } from 'vue'

import { useAppReaderDetailHeaderState } from '@/composables/useAppReaderDetailHeaderState'
import { useAppReaderMorphVisibilityState } from '@/composables/useAppReaderMorphVisibilityState'
import { useAppReaderMotionState } from '@/composables/useAppReaderMotionState'
import type { AppReaderStackRuntime } from '@/composables/useAppReaderStackRuntime'

type AppReaderPresentationRuntimeOptions = {
  readerStack: AppReaderStackRuntime
  viewport: {
    windowWidth: Ref<number>
    windowHeight: Ref<number>
  }
  chrome: {
    feedHeaderHeight: Ref<number>
    topChromeProgress: Ref<number>
    feedContentCollapsed: Ref<boolean>
    feedChromeSettling: Ref<boolean>
    detailHeaderVisible: Ref<boolean>
    detailHeaderLayerStyle: Ref<StyleValue>
  }
  theme: {
    darkTheme: Ref<boolean>
  }
  source: {
    foregroundPullActive: Ref<boolean>
  }
  timings: {
    resolveDelay: (duration: number) => number
    detailFrameMetricsInitialDelay: number
    detailFrameMetricsSettledDelay: number
  }
}

export function useAppReaderPresentationRuntime(options: AppReaderPresentationRuntimeOptions) {
  const readerStack = options.readerStack
  const readerMotion = useAppReaderMotionState({
    layout: {
      windowWidth: options.viewport.windowWidth,
      windowHeight: options.viewport.windowHeight,
      feedHeaderHeight: options.chrome.feedHeaderHeight,
      topChromeProgress: options.chrome.topChromeProgress,
      feedContentCollapsed: options.chrome.feedContentCollapsed,
    },
    sourceSurface: {
      feedHeaderHeight: options.chrome.feedHeaderHeight,
      darkTheme: options.theme.darkTheme,
      visible: readerStack.sourceReaderVisible,
      underDetail: readerStack.sourceReaderUnderDetail,
      revealProgress: readerStack.sourceReaderRevealProgress,
      offset: readerStack.sourceReaderOffset,
      stretch: readerStack.sourceReaderStretch,
      stretchAnchor: readerStack.sourceStretchAnchor,
      dragging: readerStack.readerBackDragging,
      blocksGestures: readerStack.detailBlocksGestures,
    },
    sourceContent: {
      headerHeight: options.chrome.feedHeaderHeight,
      darkTheme: options.theme.darkTheme,
      underDetail: readerStack.sourceReaderUnderDetail,
      revealProgress: readerStack.sourceReaderRevealProgress,
      chromeSettling: computed(
        () => options.chrome.feedChromeSettling.value && !readerStack.readerBackDragging.value,
      ),
      isVisible: () =>
        readerStack.sourceReaderVisible.value && !readerStack.sourceReaderUnderDetail.value,
      resolveDelay: options.timings.resolveDelay,
    },
    detailSurface: {
      stretch: readerStack.detailReaderStretch,
      stretchAnchor: readerStack.detailStretchAnchor,
      dragging: readerStack.readerBackDragging,
      blockedBackSwipeActive: readerStack.sourceReaderBlockedBackSwipeActive,
      returningToFeed: readerStack.detailReturningToFeed,
      surfaceProgress: readerStack.detailSurfaceProgress,
      committedListReturn: readerStack.detailCommittedListReturn,
    },
    detailContent: {
      surfaceProgress: readerStack.detailSurfaceProgress,
      sourceExitProgress: readerStack.detailSourceExitProgress,
      frameContentHeight: readerStack.detailFrameContentHeight,
      dragging: readerStack.readerBackDragging,
      committedListReturn: readerStack.detailCommittedListReturn,
    },
    detailProgress: {
      visible: readerStack.detailProgressVisible,
      dragging: readerStack.detailProgressDragging,
      readerBackDragging: readerStack.readerBackDragging,
      readingProgress: readerStack.detailReadingProgress,
    },
    detailText: {
      surfaceProgress: readerStack.detailSurfaceProgress,
      sourceListTitleProgress: readerStack.detailSourceListTitleProgress,
      headerFeedTitleProgress: readerStack.detailHeaderFeedTitleProgress,
      feedHeaderReturnProgress: readerStack.detailFeedHeaderReturnProgress,
      headerTitleSwapping: readerStack.detailHeaderTitleSwapping,
      headerSwapProgress: readerStack.detailHeaderSwapProgress,
      sourceLabelOpacity: readerStack.sourceNameMorphLabelOpacity,
      sourceLabelBlur: readerStack.sourceNameMorphLabelBlur,
      readerBackDragging: readerStack.readerBackDragging,
      committedListReturn: readerStack.detailCommittedListReturn,
    },
    sourceTitle: {
      revealReady: readerStack.sourceTitleRevealReady,
      pullActive: options.source.foregroundPullActive,
      titleProgress: readerStack.sourceTitleProgress,
      revealProgress: readerStack.sourceTitleRevealProgress,
      nameOriginRect: readerStack.detailSourceNameOriginRect,
      nameTargetRect: readerStack.detailSourceNameTargetRect,
      nameMorphProgress: readerStack.sourceNameMorphProgress,
      windowWidth: options.viewport.windowWidth,
      headerHeight: options.chrome.feedHeaderHeight,
      readerBackDragging: readerStack.readerBackDragging,
    },
    detailTransition: {
      originRect: readerStack.detailOriginRect,
      sourceItemTargetRect: readerStack.detailSourceItemTargetRect,
      restoringFromSourceReader: readerStack.detailRestoringFromSourceReader,
      sourceExitProgress: readerStack.detailSourceExitProgress,
      backExitProgress: readerStack.detailBackExitProgress,
      surfaceProgress: readerStack.detailSurfaceProgress,
      windowWidth: options.viewport.windowWidth,
      windowHeight: options.viewport.windowHeight,
      darkTheme: options.theme.darkTheme,
      readerBackDragging: readerStack.readerBackDragging,
      sourceReturnTargetPending: readerStack.sourceReaderReturnTargetPending,
      blockedBackSwipeActive: readerStack.sourceReaderBlockedBackSwipeActive,
      returningToFeed: readerStack.detailReturningToFeed,
      entrySettling: readerStack.detailEntrySettling,
      chromeSettling: options.chrome.feedChromeSettling,
      committedListReturn: readerStack.detailCommittedListReturn,
    },
    detailFrame: {
      item: readerStack.detailItem,
      metricsInitialDelay: options.timings.detailFrameMetricsInitialDelay,
      metricsSettledDelay: options.timings.detailFrameMetricsSettledDelay,
    },
  })

  const readerMorph = useAppReaderMorphVisibilityState({
    readerSource: readerStack.readerSource,
    sourceToggleActive: readerStack.sourceToggleActive,
    readerMotion,
    detailItem: readerStack.detailItem,
    sourceNameMorphVisible: readerStack.sourceNameMorphVisible,
    detailMorphTextVisible: readerStack.detailMorphTextVisible,
    detailMorphSummaryVisible: readerStack.detailMorphSummaryVisible,
  })

  const readerDetailHeader = useAppReaderDetailHeaderState({
    chromeVisible: readerStack.detailChromeVisible,
    readerOpen: readerStack.detailReaderOpen,
    visible: options.chrome.detailHeaderVisible,
    layerStyle: options.chrome.detailHeaderLayerStyle,
    item: readerStack.detailItem,
    readerMotion,
    previousTitle: readerStack.detailHeaderPreviousTitle,
  })

  return {
    readerMotion,
    readerMorph,
    readerDetailHeader,
  }
}
