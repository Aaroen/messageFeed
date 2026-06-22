import { useAppFeedViewSwipeInteractions } from '@/composables/useAppFeedViewSwipeInteractions'
import { useAppPointerGestureInteractions } from '@/composables/useAppPointerGestureInteractions'
import { useAppReaderBackSwipeInteractions } from '@/composables/useAppReaderBackSwipeInteractions'

type FeedViewSwipeOptions<TSurface extends string> = Parameters<
  typeof useAppFeedViewSwipeInteractions<TSurface>
>[0]

type ReaderBackSwipeOptions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
> = Parameters<typeof useAppReaderBackSwipeInteractions<TSwipeSurface, TFeedSurface>>[0]

type PointerGestureOptions = Parameters<typeof useAppPointerGestureInteractions>[0]

type TouchInjectedHandlers =
  | 'beginDetailGestureCandidate'
  | 'updateBackSwipe'
  | 'finishBackSwipe'
  | 'cancelBackSwipe'
  | 'showTopChromeForViewSwipe'
  | 'beginViewSwipeTransition'
  | 'syncViewSwipeTransition'
  | 'finishViewSwipe'

type FeedPointerInjectedHandlers =
  | 'showTopChromeForViewSwipe'
  | 'beginViewSwipeTransition'
  | 'syncViewSwipeTransition'
  | 'finishViewSwipe'

type AppGestureInteractionRuntimeOptions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
> = {
  feedView: FeedViewSwipeOptions<TSwipeSurface>
  readerBackSwipe: Omit<ReaderBackSwipeOptions<TSwipeSurface, TFeedSurface>, 'completion'> & {
    completion: Omit<
      ReaderBackSwipeOptions<TSwipeSurface, TFeedSurface>['completion'],
      'scheduleTransitionReset'
    >
    transitionResetDuration: number
  }
  pointer: {
    touch: Omit<PointerGestureOptions['touch'], TouchInjectedHandlers>
    navigationPointer: PointerGestureOptions['navigationPointer']
    feedPointer: Omit<PointerGestureOptions['feedPointer'], FeedPointerInjectedHandlers>
  }
}

export function useAppGestureInteractionRuntime<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
>(options: AppGestureInteractionRuntimeOptions<TSwipeSurface, TFeedSurface>) {
  const feedViewSwipe = useAppFeedViewSwipeInteractions<TSwipeSurface>(options.feedView)
  const readerBackSwipe = useAppReaderBackSwipeInteractions<TSwipeSurface, TFeedSurface>({
    pagePull: options.readerBackSwipe.pagePull,
    transition: options.readerBackSwipe.transition,
    drag: options.readerBackSwipe.drag,
    completion: {
      ...options.readerBackSwipe.completion,
      scheduleTransitionReset: () => {
        feedViewSwipe.scheduleSwipeTransitionReset(options.readerBackSwipe.transitionResetDuration)
      },
    },
  })
  const pointerGestures = useAppPointerGestureInteractions({
    touch: {
      ...options.pointer.touch,
      beginDetailGestureCandidate: readerBackSwipe.beginDetailGestureCandidate,
      updateBackSwipe: readerBackSwipe.updateBackSwipe,
      finishBackSwipe: readerBackSwipe.finishBackSwipe,
      cancelBackSwipe: readerBackSwipe.cancelBackSwipe,
      showTopChromeForViewSwipe: feedViewSwipe.showTopChromeForViewSwipe,
      beginViewSwipeTransition: feedViewSwipe.beginViewSwipeTransition,
      syncViewSwipeTransition: feedViewSwipe.syncViewSwipeTransition,
      finishViewSwipe: feedViewSwipe.finishViewSwipe,
    },
    navigationPointer: options.pointer.navigationPointer,
    feedPointer: {
      ...options.pointer.feedPointer,
      showTopChromeForViewSwipe: feedViewSwipe.showTopChromeForViewSwipe,
      beginViewSwipeTransition: feedViewSwipe.beginViewSwipeTransition,
      syncViewSwipeTransition: feedViewSwipe.syncViewSwipeTransition,
      finishViewSwipe: feedViewSwipe.finishViewSwipe,
    },
  })

  return {
    beginDetailGestureCandidate: readerBackSwipe.beginDetailGestureCandidate,
    updateBackSwipe: readerBackSwipe.updateBackSwipe,
    finishBackSwipe: readerBackSwipe.finishBackSwipe,
    cancelBackSwipe: readerBackSwipe.cancelBackSwipe,
    handleTouchStart: pointerGestures.handleTouchStart,
    handleTouchMove: pointerGestures.handleTouchMove,
    handleTouchEnd: pointerGestures.handleTouchEnd,
    handleTouchCancel: pointerGestures.handleTouchCancel,
    handleWindowPointerDown: pointerGestures.handleWindowPointerDown,
    handleWindowPointerMove: pointerGestures.handleWindowPointerMove,
    handleWindowPointerUp: pointerGestures.handleWindowPointerUp,
    handleWindowPointerCancel: pointerGestures.handleWindowPointerCancel,
    handleFeedPointerDown: pointerGestures.handleFeedPointerDown,
    handleFeedPointerMove: pointerGestures.handleFeedPointerMove,
    handleFeedPointerUp: pointerGestures.handleFeedPointerUp,
    handleFeedPointerCancel: pointerGestures.handleFeedPointerCancel,
  }
}
