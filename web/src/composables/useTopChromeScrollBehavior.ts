import { feedContentTopOffset } from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type TopChromeScrollBehaviorOptions = {
  topChromeProgress: ReadableRef<number>
  feedPullActive: ReadableRef<boolean>
  sourcePullActive: ReadableRef<boolean>
  feedTopPulling: ReadableRef<boolean>
  feedChromeSettling: ReadableRef<boolean>
  feedContentCollapsed: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailScrollMax: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  setTopChromeVisible: (visible: boolean) => void
  hideTopChromeForScroll: () => void
  showTopChromeOverlay: () => void
  setTopChromeOverlayProgress: (progress: number) => void
}

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

function nonOverlappingFeedRevealProgress(payload: {
  progress: number
  scrollTop: number
  headerHeight: number
  contentCollapsed: boolean
}) {
  const topOffset = feedContentTopOffset(payload.headerHeight)
  if (payload.contentCollapsed) {
    return payload.scrollTop <= topOffset ? payload.progress : 0
  }

  const maxProgress = (payload.headerHeight + topOffset - payload.scrollTop) / payload.headerHeight
  return Math.min(payload.progress, clampProgress(maxProgress))
}

export function useTopChromeScrollBehavior(options: TopChromeScrollBehaviorOptions) {
  function restoreFeedTopChromeSpaceIfNeeded(current: number) {
    if (!options.isFeedRoute.value || options.detailReaderOpen.value || !options.feedContentCollapsed.value) {
      return false
    }

    if (options.topChromeProgress.value <= 0.04) {
      return false
    }

    const topOffset = feedContentTopOffset(options.feedHeaderHeight.value)
    if (current > topOffset) {
      return false
    }

    options.setTopChromeVisible(true)
    return true
  }

  function updateByScroll(current: number, previous: number) {
    if (options.feedPullActive.value || options.sourcePullActive.value || options.feedTopPulling.value) {
      return
    }

    if (restoreFeedTopChromeSpaceIfNeeded(current)) {
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
      options.hideTopChromeForScroll()
      return
    }

    if (delta < 0 && options.topChromeProgress.value < 0.99) {
      if (options.isFeedRoute.value && !options.detailReaderOpen.value) {
        const headerHeight = options.feedHeaderHeight.value
        const revealDistance = Math.max(headerHeight * 2, 1)
        const revealSettleDistance = feedContentTopOffset(headerHeight)
        const progress = nonOverlappingFeedRevealProgress({
          progress: clampProgress(1 - current / revealDistance),
          scrollTop: current,
          headerHeight,
          contentCollapsed: options.feedContentCollapsed.value,
        })
        if (
          (progress >= 0.95 || current <= revealSettleDistance) &&
          options.topChromeProgress.value < 0.99
        ) {
          if (options.feedContentCollapsed.value) {
            options.setTopChromeVisible(true)
            return
          }
          options.showTopChromeOverlay()
          return
        }

        if (progress <= 0.01) {
          options.setTopChromeOverlayProgress(0)
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
