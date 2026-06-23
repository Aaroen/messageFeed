import { clampProgress } from '@/composables/feedChromeMetrics'
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

type FeedViewSwipeFinishResult = {
  committed: boolean
  settlePayload: {
    progress: number
    isBlocked: boolean
  }
  startedWithHiddenChrome: boolean
}

type FeedViewSwipeControllerOptions<TSurface extends string> = {
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
  motionNormalDuration: number
  resolveDelay: (duration: number) => number
  beginSwipeTransition: (payload: SwipeTransitionBeginPayload<TSurface>) => void
  updateSwipeTransition: (payload: SwipeTransitionPayload<TSurface>) => void
  settleSwipeTransition: (
    committed: boolean,
    payload: { progress?: number; isBlocked?: boolean },
  ) => void
  scheduleSwipeReset: (delayMS: number) => void
  swipeTransitionBeginPayload: (offset: number) => SwipeTransitionBeginPayload<TSurface>
  swipeTransitionUpdatePayload: (offset: number) => SwipeTransitionPayload<TSurface>
  finishSwipeResult: (nextPath: string | null) => FeedViewSwipeFinishResult
  settleFinishedSwipe: (delayMS: number) => void
  markStartedWithHiddenChrome: () => void
  beginTopChromeGestureReturn: (options: {
    settleDelayMS: number
    preserveContentCollapsed?: boolean
  }) => void
  setTopChromeVisible: (visible: boolean) => void
  hideTopChromeOverlay: () => void
  pushRoute: (path: string) => Promise<unknown> | void
  topChromeGestureSettleDelayMS: number
}

export function useFeedViewSwipeController<TSurface extends string>(
  options: FeedViewSwipeControllerOptions<TSurface>,
) {
  function scheduleSwipeTransitionReset(duration = options.motionNormalDuration) {
    options.scheduleSwipeReset(options.resolveDelay(duration))
  }

  function beginViewSwipeTransition(offset: number) {
    options.beginSwipeTransition(options.swipeTransitionBeginPayload(offset))
  }

  function syncViewSwipeTransition(offset: number) {
    options.updateSwipeTransition(options.swipeTransitionUpdatePayload(offset))
  }

  function finishViewSwipe(nextPath: string | null) {
    const result = options.finishSwipeResult(nextPath)
    options.settleSwipeTransition(result.committed, result.settlePayload)

    if (result.startedWithHiddenChrome) {
      if (options.feedContentCollapsed.value) {
        options.hideTopChromeOverlay()
      } else {
        options.setTopChromeVisible(true)
      }
    }

    if (nextPath) {
      void options.pushRoute(nextPath)
    }
    options.settleFinishedSwipe(options.resolveDelay(options.motionNormalDuration))
    scheduleSwipeTransitionReset(options.motionNormalDuration)
  }

  function showTopChromeForViewSwipe() {
    const shouldRevealChrome =
      clampProgress(options.topChromeProgress.value) < 0.99 && !options.feedContentCollapsed.value
    if (shouldRevealChrome) {
      options.markStartedWithHiddenChrome()
      options.beginTopChromeGestureReturn({
        settleDelayMS: options.topChromeGestureSettleDelayMS,
        preserveContentCollapsed: true,
      })
    }
  }

  return {
    scheduleSwipeTransitionReset,
    beginViewSwipeTransition,
    syncViewSwipeTransition,
    finishViewSwipe,
    showTopChromeForViewSwipe,
  }
}
