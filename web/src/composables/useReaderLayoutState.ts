import { computed } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'
import type { RectSnapshot } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type ReaderLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  windowHeight: ReadableRef<number>
  feedHeaderHeight: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
  feedContentCollapsed: ReadableRef<boolean>
}

export function useReaderLayoutState(options: ReaderLayoutStateOptions) {
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const sourceHeaderSpace = computed(() => {
    if (!options.feedContentCollapsed.value) {
      return options.feedHeaderHeight.value
    }

    return options.feedHeaderHeight.value * topChromeProgress.value
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
