import { useAppTouchGestureHandlers } from '@/composables/useAppTouchGestureHandlers'
import { useFeedPointerSwipeHandlers } from '@/composables/useFeedPointerSwipeHandlers'
import { useNavigationPointerHandlers } from '@/composables/useNavigationPointerHandlers'

type AppPointerGestureInteractionsOptions = {
  touch: Parameters<typeof useAppTouchGestureHandlers>[0]
  navigationPointer: Parameters<typeof useNavigationPointerHandlers>[0]
  feedPointer: Parameters<typeof useFeedPointerSwipeHandlers>[0]
}

export function useAppPointerGestureInteractions(options: AppPointerGestureInteractionsOptions) {
  const touchHandlers = useAppTouchGestureHandlers(options.touch)
  const navigationPointerHandlers = useNavigationPointerHandlers(options.navigationPointer)
  const feedPointerHandlers = useFeedPointerSwipeHandlers(options.feedPointer)

  return {
    handleTouchStart: touchHandlers.handleTouchStart,
    handleTouchMove: touchHandlers.handleTouchMove,
    handleTouchEnd: touchHandlers.handleTouchEnd,
    handleTouchCancel: touchHandlers.handleTouchCancel,
    handleWindowPointerDown: navigationPointerHandlers.handleWindowPointerDown,
    handleWindowPointerMove: navigationPointerHandlers.handleWindowPointerMove,
    handleWindowPointerUp: navigationPointerHandlers.handleWindowPointerUp,
    handleWindowPointerCancel: navigationPointerHandlers.handleWindowPointerCancel,
    handleFeedPointerDown: feedPointerHandlers.handleFeedPointerDown,
    handleFeedPointerMove: feedPointerHandlers.handleFeedPointerMove,
    handleFeedPointerUp: feedPointerHandlers.handleFeedPointerUp,
    handleFeedPointerCancel: feedPointerHandlers.handleFeedPointerCancel,
  }
}
