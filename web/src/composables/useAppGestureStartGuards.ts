type ReadableRef<T> = {
  readonly value: T
}

type AppGestureStartGuardsOptions = {
  isFeedRoute: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
  sourceReaderOpen: ReadableRef<boolean>
  isSubscriptionsRoute: () => boolean
  detailBlocksGestures: () => boolean
  feedPullBusy: ReadableRef<boolean>
}

export function useAppGestureStartGuards(options: AppGestureStartGuardsOptions) {
  function canStartViewSwipe(_clientX: number) {
    if (
      !options.isFeedRoute.value ||
      options.navigationVisible.value ||
      options.sourceReaderOpen.value ||
      options.feedPullBusy.value ||
      options.detailBlocksGestures()
    ) {
      return false
    }

    return true
  }

  function canStartNavigationOpen(_clientX: number) {
    return (
      options.isSubscriptionsRoute() &&
      !options.navigationVisible.value &&
      !options.sourceReaderOpen.value &&
      !options.feedPullBusy.value &&
      !options.detailBlocksGestures()
    )
  }

  return {
    canStartViewSwipe,
    canStartNavigationOpen,
  }
}
