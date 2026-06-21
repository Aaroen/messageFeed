import { useFeedPagerTransition } from '@/composables/useFeedPagerTransition'
import { useSwipeTransition } from '@/composables/useSwipeTransition'

type AppSwipeNavigationStateOptions = {
  feedPager: Parameters<typeof useFeedPagerTransition>[0]
}

export function useAppSwipeNavigationState<TSurface extends string>(
  options: AppSwipeNavigationStateOptions,
) {
  const swipeTransition = useSwipeTransition<TSurface>()
  const feedPagerTransition = useFeedPagerTransition(options.feedPager)

  return {
    swipePhase: swipeTransition.phase,
    swipeDirection: swipeTransition.direction,
    swipeProgress: swipeTransition.progress,
    swipeIsBlocked: swipeTransition.isBlocked,
    beginSwipeTransition: swipeTransition.begin,
    updateSwipeTransition: swipeTransition.update,
    settleSwipeTransition: swipeTransition.settle,
    scheduleSwipeReset: swipeTransition.scheduleReset,
    clearSwipeTransitionTimer: swipeTransition.clearTimer,
    feedPagerTransition,
    feedPagerDragThreshold: feedPagerTransition.dragThreshold,
    viewDragOffset: feedPagerTransition.dragOffset,
    viewSettling: feedPagerTransition.settling,
    viewSwipeCandidateActive: feedPagerTransition.viewSwipeCandidateActive,
    viewSwipeActive: feedPagerTransition.viewSwipeActive,
    activeFeedIndex: feedPagerTransition.activeIndex,
    activeFeedSurface: feedPagerTransition.activeSurface,
    feedTrackStyle: feedPagerTransition.trackStyle,
    viewSwipeTargetKey: feedPagerTransition.targetKey,
    viewSwipeTargetVisible: feedPagerTransition.targetVisible,
    viewSwipeTargetProgress: feedPagerTransition.targetProgress,
    resetFeedViewSwipeTracking: feedPagerTransition.resetViewSwipeTracking,
    clearFeedViewStartedWithHiddenChrome: feedPagerTransition.clearStartedWithHiddenChrome,
    resetFeedViewDragOffset: feedPagerTransition.resetDragOffset,
    clearFeedPagerTimers: feedPagerTransition.clearTimers,
  }
}
