import { computed } from 'vue'

import { clampProgress, sourceContentTopOffset, topScrollInset } from '@/composables/feedChromeMetrics'
import type { RectSnapshot } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  windowHeight: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  sourceReaderScrollTop: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
}

export function useReaderLayoutState(options: ReaderLayoutStateOptions) {
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const sourceTopOffset = sourceContentTopOffset()
  const sourceTopInset = computed(() => topScrollInset(options.sourceReaderScrollTop.value, sourceTopOffset))
  const sourceHeaderSpace = computed(() => {
    if (!options.feedContentCollapsed.value) {
      return options.feedHeaderHeight.value + sourceTopInset.value
    }

    const collapsedSpace = options.feedHeaderHeight.value * topChromeProgress.value
    if (topChromeProgress.value > 0.04 && options.sourceReaderScrollTop.value <= sourceTopOffset) {
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
