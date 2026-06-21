type ReadableRef<T> = {
  readonly value: T
}

type TopChromeScrollBehaviorOptions = {
  topChromeProgress: ReadableRef<number>
  feedPullActive: ReadableRef<boolean>
  sourcePullActive: ReadableRef<boolean>
  feedTopPulling: ReadableRef<boolean>
  feedChromeSettling: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailScrollMax: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  setTopChromeVisible: (visible: boolean) => void
}

export function useTopChromeScrollBehavior(options: TopChromeScrollBehaviorOptions) {
  function updateByScroll(current: number, previous: number) {
    if (
      current <= 1 &&
      options.topChromeProgress.value < 0.99 &&
      !options.feedPullActive.value &&
      !options.feedTopPulling.value
    ) {
      options.setTopChromeVisible(true)
      return
    }

    if (
      options.feedPullActive.value ||
      options.sourcePullActive.value ||
      options.feedTopPulling.value ||
      options.feedChromeSettling.value
    ) {
      return
    }

    const delta = current - previous
    if (Math.abs(delta) < 3 || current < 0) {
      return
    }

    if (options.detailReaderOpen.value) {
      const max = options.detailScrollMax.value
      const bottomStabilityZone = 28
      const nearBottom =
        max > 0 &&
        current >= max - bottomStabilityZone &&
        previous >= max - bottomStabilityZone
      if (nearBottom) {
        return
      }
    }

    const hideThreshold = options.detailReaderOpen.value
      ? options.feedHeaderHeight.value
      : options.isFeedRoute.value
        ? 8
        : options.feedHeaderHeight.value
    if (delta > 0 && current >= hideThreshold && options.topChromeProgress.value > 0.01) {
      options.setTopChromeVisible(false)
      return
    }

    if (delta < 0 && options.topChromeProgress.value < 0.99) {
      options.setTopChromeVisible(true)
    }
  }

  return {
    updateByScroll,
  }
}
