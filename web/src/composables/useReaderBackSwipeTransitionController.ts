import type { SwipeDirection } from '@/composables/useSwipeTransition'

type ReadableRef<T> = {
  readonly value: T
}

type SwipeTransitionPayload<TSurface extends string> = {
  to?: TSurface | null
  direction?: SwipeDirection
  progress?: number
  isBlocked?: boolean
}

type SwipeTransitionBeginPayload<TSurface extends string> = SwipeTransitionPayload<TSurface> & {
  from: TSurface | null
  direction: SwipeDirection
}

type ReaderBackSwipeTransitionSurfaces<TFeedSurface extends string> = {
  activeFeedSurface: TFeedSurface
  pageReturnSurface: TFeedSurface
}

type ReaderBackSwipeTransitionControllerOptions<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
> = {
  activeFeedSurface: ReadableRef<TFeedSurface>
  pageReturnSurface: TFeedSurface
  fallbackStretch: ReadableRef<number>
  beginSwipeTransition: (payload: SwipeTransitionBeginPayload<TSwipeSurface>) => void
  updateSwipeTransition: (payload: SwipeTransitionPayload<TSwipeSurface>) => void
  transitionBeginPayload: (
    deltaX: number,
    surfaces: ReaderBackSwipeTransitionSurfaces<TFeedSurface>,
  ) => SwipeTransitionBeginPayload<TSwipeSurface>
  transitionUpdatePayload: (
    deltaX: number,
    fallbackStretch: number,
    surfaces: ReaderBackSwipeTransitionSurfaces<TFeedSurface>,
  ) => SwipeTransitionPayload<TSwipeSurface>
}

export function useReaderBackSwipeTransitionController<
  TSwipeSurface extends string,
  TFeedSurface extends TSwipeSurface,
>(options: ReaderBackSwipeTransitionControllerOptions<TSwipeSurface, TFeedSurface>) {
  function transitionSurfaces(): ReaderBackSwipeTransitionSurfaces<TFeedSurface> {
    return {
      activeFeedSurface: options.activeFeedSurface.value,
      pageReturnSurface: options.pageReturnSurface,
    }
  }

  function beginBackSwipeTransition(deltaX: number) {
    options.beginSwipeTransition(options.transitionBeginPayload(deltaX, transitionSurfaces()))
  }

  function syncBackSwipeTransition(deltaX: number) {
    options.updateSwipeTransition(
      options.transitionUpdatePayload(deltaX, options.fallbackStretch.value, transitionSurfaces()),
    )
  }

  return {
    beginBackSwipeTransition,
    syncBackSwipeTransition,
  }
}
