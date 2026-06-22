import { useAppGesturePolicy } from '@/composables/useAppGesturePolicy'
import { useAppSwipeNavigationState } from '@/composables/useAppSwipeNavigationState'

type ReadableRef<T> = {
  readonly value: T
}

type AppSwipeGestureRuntimeOptions = {
  getActiveKey: () => string | symbol | null | undefined
  windowWidth: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
  sourceReaderOpen: ReadableRef<boolean>
  isSubscriptionsRoute: () => boolean
  detailBlocksGestures: () => boolean
  feedPullBusy: () => boolean
}

export function useAppSwipeGestureRuntime<TSurface extends string>(
  options: AppSwipeGestureRuntimeOptions,
) {
  const swipeNavigationState = useAppSwipeNavigationState<TSurface>({
    feedPager: {
      getActiveKey: options.getActiveKey,
      getWindowWidth: () => options.windowWidth.value,
      isFeedRoute: () => options.isFeedRoute.value,
      isDetailReaderOpen: () => options.detailReaderOpen.value,
    },
  })
  const gesturePolicy = useAppGesturePolicy({
    direction: {
      viewDragThreshold: swipeNavigationState.feedPagerDragThreshold,
    },
    startGuards: {
      isFeedRoute: options.isFeedRoute,
      navigationVisible: options.navigationVisible,
      sourceReaderOpen: options.sourceReaderOpen,
      isSubscriptionsRoute: options.isSubscriptionsRoute,
      detailBlocksGestures: options.detailBlocksGestures,
      feedPullBusy: options.feedPullBusy,
    },
  })

  return {
    ...swipeNavigationState,
    ...gesturePolicy,
  }
}
