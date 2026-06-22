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
  showTopChromeOverlay: () => void
  setTopChromeOverlayProgress: (progress: number) => void
}

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

export function useTopChromeScrollBehavior(options: TopChromeScrollBehaviorOptions) {
  function updateByScroll(current: number, previous: number) {
    if (options.feedPullActive.value || options.sourcePullActive.value || options.feedTopPulling.value) {
      return
    }

    const delta = current - previous
    if (Math.abs(delta) < 3 || current < 0) {
      return
    }

    const canInterruptChromeSettling =
      delta < 0 && options.isFeedRoute.value && !options.detailReaderOpen.value
    if (options.feedChromeSettling.value && !canInterruptChromeSettling) {
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
      if (options.isFeedRoute.value && !options.detailReaderOpen.value) {
        const revealDistance = Math.max(options.feedHeaderHeight.value * 2, 1)
        const progress = clampProgress(1 - current / revealDistance)
        if (progress >= 0.99 && options.topChromeProgress.value < 0.99) {
          options.showTopChromeOverlay()
          return
        }
        options.setTopChromeOverlayProgress(progress)
        return
      }
      options.showTopChromeOverlay()
    }
  }

  return {
    updateByScroll,
  }
}
