import { computed, type Ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

type StretchAnchor = 'left' | 'right' | null

type ReaderDetailSurfaceMotionOptions = {
  stretch: Ref<number>
  stretchAnchor: Ref<StretchAnchor>
  dragging: Ref<boolean>
  blockedBackSwipeActive: Ref<boolean>
  returningToFeed: Ref<boolean>
  surfaceProgress: Ref<number>
  committedListReturn: () => boolean
}

function stretchTransformOrigin(stretch: number, anchor: StretchAnchor) {
  if (stretch > 0 || anchor === 'left') {
    return 'left center'
  }
  if (stretch < 0 || anchor === 'right') {
    return 'right center'
  }
  return undefined
}

export function useReaderDetailSurfaceMotion(options: ReaderDetailSurfaceMotionOptions) {
  const surfaceState = computed(() => {
    const committedListReturn = options.committedListReturn()
    const returningToFeed = options.returningToFeed.value
    const stretch = Number.isFinite(options.stretch.value) ? options.stretch.value : 0
    const overlayOpacity =
      options.blockedBackSwipeActive.value || committedListReturn || returningToFeed
        ? 0
        : clampProgress(options.surfaceProgress.value)
    return {
      committedListReturn,
      returningToFeed,
      stretch,
      overlayOpacity,
    }
  })

  const readerStyle = computed(() => {
    const state = surfaceState.value
    return {
      transform: `translate3d(0, 0, 0) scaleX(${(1 + Math.abs(state.stretch)).toFixed(4)})`,
      transition: options.dragging.value ? 'none' : 'transform var(--motion-quick) var(--ease-standard)',
      transformOrigin: stretchTransformOrigin(state.stretch, options.stretchAnchor.value),
      pointerEvents:
        state.committedListReturn || state.returningToFeed ? ('none' as const) : ('auto' as const),
    }
  })

  const backdropStyle = computed(() => ({
    opacity: surfaceState.value.overlayOpacity.toFixed(3),
  }))

  return {
    readerStyle,
    backdropStyle,
  }
}
