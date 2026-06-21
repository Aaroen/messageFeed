import { useAppGestureStartGuards } from '@/composables/useAppGestureStartGuards'
import { useGestureDirection } from '@/composables/useGestureDirection'

type AppGesturePolicyOptions = {
  direction: Parameters<typeof useGestureDirection>[0]
  startGuards: Parameters<typeof useAppGestureStartGuards>[0]
}

export function useAppGesturePolicy(options: AppGesturePolicyOptions) {
  const direction = useGestureDirection(options.direction)
  const startGuards = useAppGestureStartGuards(options.startGuards)

  return {
    isHorizontalSwipe: direction.isHorizontalSwipe,
    isViewHorizontalSwipe: direction.isViewHorizontalSwipe,
    isNavigationDrag: direction.isNavigationDrag,
    isBackHorizontalSwipe: direction.isBackHorizontalSwipe,
    shouldCancelTopPull: direction.shouldCancelTopPull,
    shouldWaitForTopPull: direction.shouldWaitForTopPull,
    canStartViewSwipe: startGuards.canStartViewSwipe,
    canStartNavigationOpen: startGuards.canStartNavigationOpen,
  }
}
