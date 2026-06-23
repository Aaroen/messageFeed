import { computed } from 'vue'

import {
  chromePhaseConsumesCollapsedLayout,
  chromeNeedsVisibleTopClearance,
  clampProgress,
  sourceContentTopOffset,
  sourceVisibleContentTopOffset,
  topScrollInset,
} from '@/composables/feedChromeMetrics'
import type { ChromePhase } from '@/composables/useChromeState'
import type { RectSnapshot } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  windowHeight: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  sourceReaderScrollTop: ReadableRef<number>
  topChromePhase: ReadableRef<ChromePhase>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
}

export function useReaderLayoutState(options: ReaderLayoutStateOptions) {
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const collapsedLayoutProgress = computed(() =>
    chromePhaseConsumesCollapsedLayout(options.topChromePhase.value) ? topChromeProgress.value : 0,
  )
  const sourceTopOffset = sourceContentTopOffset()
  const sourceVisibleTopOffset = computed(() => sourceVisibleContentTopOffset(options.feedHeaderHeight.value))
  const sourceVisibleTopClearance = computed(() =>
    Math.max(0, sourceVisibleTopOffset.value - sourceTopOffset),
  )
  const sourceTopInset = computed(() => topScrollInset(options.sourceReaderScrollTop.value, sourceTopOffset))
  const visibleChromeNeedsSourceTopClearance = computed(() => {
    if (!options.feedContentCollapsed.value) {
      return false
    }

    if (options.sourceReaderScrollTop.value > sourceTopOffset || topChromeProgress.value <= 0.04) {
      return false
    }

    return chromeNeedsVisibleTopClearance(options.topChromePhase.value, topChromeProgress.value)
  })
  const sourceHeaderSpace = computed(() => {
    const visibleSourceHeaderSpace =
      options.feedHeaderHeight.value + sourceTopInset.value + sourceVisibleTopClearance.value

    if (!options.feedContentCollapsed.value) {
      return visibleSourceHeaderSpace
    }

    if (visibleChromeNeedsSourceTopClearance.value) {
      return visibleSourceHeaderSpace
    }

    const collapsedSpace = options.feedHeaderHeight.value * collapsedLayoutProgress.value
    if (collapsedLayoutProgress.value > 0.04 && options.sourceReaderScrollTop.value <= sourceTopOffset) {
      return collapsedSpace + sourceTopInset.value
    }

    return collapsedSpace
  })
  const detailSourceFallbackTargetRect = computed<RectSnapshot>(() => {
    const side = options.windowWidth.value <= 720 ? 24 : 46
    const top = options.feedHeaderHeight.value + 24
    return {
      left: side,
      top,
      width: Math.max(1, options.windowWidth.value - side * 2),
      height: 146,
    }
  })
  const detailSurfaceMargin = computed(() => (options.windowWidth.value <= 720 ? 10 : 14))
  const detailExpandedTop = computed(
    () => (options.feedHeaderHeight.value + detailSurfaceMargin.value) * topChromeProgress.value,
  )
  const detailFrameMinHeight = computed(() =>
    Math.max(300, options.windowHeight.value - detailExpandedTop.value - detailSurfaceMargin.value - 96),
  )

  return {
    sourceHeaderSpace,
    detailSourceFallbackTargetRect,
    detailSurfaceMargin,
    detailExpandedTop,
    detailFrameMinHeight,
  }
}
