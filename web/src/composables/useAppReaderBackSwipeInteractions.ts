import type { AppPagePullState } from '@/composables/useAppPagePullState'
import { useReaderBackSwipeCompletion } from '@/composables/useReaderBackSwipeCompletion'
import { useReaderBackSwipeDragHandlers } from '@/composables/useReaderBackSwipeDragHandlers'
import { useReaderBackSwipeTransitionController } from '@/composables/useReaderBackSwipeTransitionController'

type ReaderBackSwipeTransitionOptions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
> = Parameters<typeof useReaderBackSwipeTransitionController<TSwipeSurface, TFeedSurface>>[0]

type ReaderBackSwipeDragOptions = Parameters<typeof useReaderBackSwipeDragHandlers>[0]

type ReaderBackSwipeCompletionOptions = Parameters<typeof useReaderBackSwipeCompletion>[0]

type AppReaderBackSwipeInteractionsOptions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
> = {
  pagePull: AppPagePullState
  transition: ReaderBackSwipeTransitionOptions<TSwipeSurface, TFeedSurface>
  drag: Omit<
    ReaderBackSwipeDragOptions,
    'beginBackSwipeTransition' | 'syncBackSwipeTransition' | 'setPageSideStretch' | 'setPageSideOffset'
  >
  completion: ReaderBackSwipeCompletionOptions
}

export function useAppReaderBackSwipeInteractions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
>(options: AppReaderBackSwipeInteractionsOptions<TSwipeSurface, TFeedSurface>) {
  const transitionController = useReaderBackSwipeTransitionController(options.transition)
  const dragHandlers = useReaderBackSwipeDragHandlers({
    ...options.drag,
    beginBackSwipeTransition: transitionController.beginBackSwipeTransition,
    syncBackSwipeTransition: transitionController.syncBackSwipeTransition,
    setPageSideStretch: options.pagePull.setSideStretch,
    setPageSideOffset: options.pagePull.setSideOffset,
  })
  const completion = useReaderBackSwipeCompletion(options.completion)

  return {
    beginDetailGestureCandidate: dragHandlers.beginDetailGestureCandidate,
    updateBackSwipe: dragHandlers.updateBackSwipe,
    finishBackSwipe: completion.finishBackSwipe,
    cancelBackSwipe: completion.cancelBackSwipe,
  }
}
