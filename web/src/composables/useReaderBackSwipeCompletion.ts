type ReaderBackSwipeCompletionAction =
  | 'restore-item-expansion'
  | 'restore-detail-from-source-swipe'
  | 'restore-parked-source'
  | 'complete-detail-to-source'
  | 'collapse-detail'
  | 'restore-detail-from-parked-source'
  | 'return-page'
  | 'reset'

type ReaderBackSwipeCompletionActionHandlers = {
  restoreItemExpansion: () => void
  restoreDetailFromSourceSwipe: () => void
  restoreParkedSource: () => void
  completeDetailToSource: () => void
  collapseDetail: () => void
  restoreDetailFromParkedSource: () => void
  returnPage: () => void
  reset: () => void
}

type ReaderBackSwipeFinishResult = {
  committed: boolean
  progress: number
  isBlocked: boolean
  action: ReaderBackSwipeCompletionAction
}

type ReaderBackSwipeCancelResult = {
  progress: number
  isBlocked: boolean
  action: ReaderBackSwipeCompletionAction
}

type ReaderBackSwipeCompletionOptions = ReaderBackSwipeCompletionActionHandlers & {
  switchDistance: number
  getFallbackStretch: () => number
  finishResult: (
    deltaX: number,
    switchDistance: number,
    fallbackStretch: number,
  ) => ReaderBackSwipeFinishResult
  cancelResult: (fallbackStretch: number) => ReaderBackSwipeCancelResult
  settleTransition: (committed: boolean, payload: { progress?: number; isBlocked?: boolean }) => void
  scheduleTransitionReset: () => void
  suppressFollowingClick: () => void
  applyAction: (
    action: ReaderBackSwipeCompletionAction,
    handlers: ReaderBackSwipeCompletionActionHandlers,
  ) => void
}

export function useReaderBackSwipeCompletion(options: ReaderBackSwipeCompletionOptions) {
  const actionHandlers: ReaderBackSwipeCompletionActionHandlers = {
    restoreItemExpansion: options.restoreItemExpansion,
    restoreDetailFromSourceSwipe: options.restoreDetailFromSourceSwipe,
    restoreParkedSource: options.restoreParkedSource,
    completeDetailToSource: options.completeDetailToSource,
    collapseDetail: options.collapseDetail,
    restoreDetailFromParkedSource: options.restoreDetailFromParkedSource,
    returnPage: options.returnPage,
    reset: options.reset,
  }

  function finishBackSwipe(deltaX: number, _deltaY?: number) {
    const result = options.finishResult(
      deltaX,
      options.switchDistance,
      options.getFallbackStretch(),
    )

    options.settleTransition(result.committed, {
      progress: result.progress,
      isBlocked: result.isBlocked,
    })
    options.scheduleTransitionReset()

    if (result.committed) {
      options.suppressFollowingClick()
    }
    options.applyAction(result.action, actionHandlers)
  }

  function cancelBackSwipe() {
    const result = options.cancelResult(options.getFallbackStretch())

    options.settleTransition(false, {
      progress: result.progress,
      isBlocked: result.isBlocked,
    })
    options.scheduleTransitionReset()
    options.applyAction(result.action, actionHandlers)
  }

  return {
    finishBackSwipe,
    cancelBackSwipe,
  }
}
