import { useAppReaderCloseInteractions } from '@/composables/useAppReaderCloseInteractions'
import { useAppReaderOpenInteractions } from '@/composables/useAppReaderOpenInteractions'
import { useAppReaderSourceCloseInteractions } from '@/composables/useAppReaderSourceCloseInteractions'
import { useAppReaderStackActions } from '@/composables/useAppReaderStackActions'
import { useAppReaderTransitionRects } from '@/composables/useAppReaderTransitionRects'
import { useReaderRouteQueryRestore } from '@/composables/useReaderRouteQueryRestore'

type ReaderOpenOptions = Parameters<typeof useAppReaderOpenInteractions>[0]
type ReaderSourceCloseOptions = Parameters<typeof useAppReaderSourceCloseInteractions>[0]
type ReaderCloseOptions = Parameters<typeof useAppReaderCloseInteractions>[0]
type ReaderRouteQueryRestoreOptions = Omit<
  Parameters<typeof useReaderRouteQueryRestore>[0],
  'openSourceReader' | 'openItemReader'
>

type AppReaderNavigationRuntimeOptions = {
  transitionRects: Parameters<typeof useAppReaderTransitionRects>[0]
  stackActions: Parameters<typeof useAppReaderStackActions>[0]
  open: {
    sourceOpen: Omit<ReaderOpenOptions['sourceOpen'], 'captureDetailSourceTransitionRects'>
    sourceReveal: Omit<ReaderOpenOptions['sourceReveal'], 'captureDetailSourceTransitionRects'>
    itemOpen: Omit<ReaderOpenOptions['itemOpen'], 'captureDetailSourceTransitionRects'>
  }
  sourceClose: {
    parkedRestore: Omit<
      ReaderSourceCloseOptions['parkedRestore'],
      'captureVisibleSourceReturnTarget' | 'restoreMorphingItemContent' | 'scheduleHiddenSourceReaderCleanup'
    >
    sourceClose: Omit<ReaderSourceCloseOptions['sourceClose'], 'scheduleHiddenSourceReaderCleanup'>
  }
  close: {
    backSwipeReset: ReaderCloseOptions['backSwipeReset']
    itemClose: Omit<
      ReaderCloseOptions['itemClose'],
      'refreshDetailFeedOriginRect' | 'restorePreviousParkedDetail' | 'scheduleHiddenSourceReaderCleanup'
    >
    restoreActions: Omit<
      ReaderCloseOptions['restoreActions'],
      'captureDetailSourceTransitionRects' | 'restoreMorphingItemContent'
    >
  }
}

export function useAppReaderNavigationRuntime(options: AppReaderNavigationRuntimeOptions) {
  const transitionRects = useAppReaderTransitionRects(options.transitionRects)
  const stackActions = useAppReaderStackActions(options.stackActions)
  const openInteractions = useAppReaderOpenInteractions({
    sourceOpen: {
      ...options.open.sourceOpen,
      captureDetailSourceTransitionRects: transitionRects.captureDetailSourceTransitionRects,
    },
    sourceReveal: {
      ...options.open.sourceReveal,
      captureDetailSourceTransitionRects: transitionRects.captureDetailSourceTransitionRects,
    },
    itemOpen: {
      ...options.open.itemOpen,
      captureDetailSourceTransitionRects: transitionRects.captureDetailSourceTransitionRects,
    },
  })
  const sourceCloseInteractions = useAppReaderSourceCloseInteractions({
    parkedRestore: {
      ...options.sourceClose.parkedRestore,
      captureVisibleSourceReturnTarget: transitionRects.captureVisibleSourceReturnTarget,
      restoreMorphingItemContent: stackActions.restoreMorphingItemContent,
      scheduleHiddenSourceReaderCleanup: stackActions.scheduleHiddenSourceReaderCleanup,
    },
    sourceClose: {
      ...options.sourceClose.sourceClose,
      scheduleHiddenSourceReaderCleanup: stackActions.scheduleHiddenSourceReaderCleanup,
    },
  })
  const closeInteractions = useAppReaderCloseInteractions({
    backSwipeReset: options.close.backSwipeReset,
    itemClose: {
      ...options.close.itemClose,
      refreshDetailFeedOriginRect: transitionRects.refreshDetailFeedOriginRect,
      restorePreviousParkedDetail: stackActions.restorePreviousParkedDetail,
      scheduleHiddenSourceReaderCleanup: stackActions.scheduleHiddenSourceReaderCleanup,
    },
    restoreActions: {
      ...options.close.restoreActions,
      captureDetailSourceTransitionRects: transitionRects.captureDetailSourceTransitionRects,
      restoreMorphingItemContent: stackActions.restoreMorphingItemContent,
    },
  })

  function installRouteQueryRestore(routeOptions: ReaderRouteQueryRestoreOptions) {
    return useReaderRouteQueryRestore({
      ...routeOptions,
      openSourceReader: openInteractions.openSourceReader,
      openItemReader: openInteractions.openItemReader,
    })
  }

  return {
    openSourceReader: openInteractions.openSourceReader,
    showSourceReaderUnderDetail: openInteractions.showSourceReaderUnderDetail,
    openItemReader: openInteractions.openItemReader,
    restoreDetailFromParkedSource: sourceCloseInteractions.restoreDetailFromParkedSource,
    restoreSourceReaderBackTarget: sourceCloseInteractions.restoreSourceReaderBackTarget,
    closeSourceReader: sourceCloseInteractions.closeSourceReader,
    finishCommittedListReturnForGesture: closeInteractions.finishCommittedListReturnForGesture,
    closeItemReader: closeInteractions.closeItemReader,
    collapseItemReader: closeInteractions.collapseItemReader,
    resetBackSwipeOffset: closeInteractions.resetBackSwipeOffset,
    clearBackSwipeStretchAnchorTimer: closeInteractions.clearBackSwipeStretchAnchorTimer,
    restoreParkedSourceReader: closeInteractions.restoreParkedSourceReader,
    restoreItemReaderExpansion: closeInteractions.restoreItemReaderExpansion,
    restoreDetailFromSourceSwipe: closeInteractions.restoreDetailFromSourceSwipe,
    completeDetailToSourceReader: closeInteractions.completeDetailToSourceReader,
    refreshDetailFeedOriginRect: transitionRects.refreshDetailFeedOriginRect,
    captureDetailSourceTransitionRects: transitionRects.captureDetailSourceTransitionRects,
    captureVisibleSourceReturnTarget: transitionRects.captureVisibleSourceReturnTarget,
    clearDetailSourceTransitionRectCapture: transitionRects.clearDetailSourceTransitionRectCapture,
    installRouteQueryRestore,
  }
}
