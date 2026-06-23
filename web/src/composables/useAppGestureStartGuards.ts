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

function navigationEdgeWidth() {
  const viewportWidth = typeof window === 'undefined' ? 390 : window.innerWidth
  return Math.min(Math.max(viewportWidth * 0.08, 44), 56)
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

  function canStartNavigationOpen(clientX: number) {
    return (
      options.isSubscriptionsRoute() &&
      clientX <= navigationEdgeWidth() &&
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
