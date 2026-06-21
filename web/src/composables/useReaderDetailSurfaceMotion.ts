import { computed, type Ref } from 'vue'

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
  const readerStyle = computed(() => {
    const committedListReturn = options.committedListReturn()
    const stretch = options.stretch.value
    return {
      transform: `translate3d(0, 0, 0) scaleX(${(1 + Math.abs(stretch)).toFixed(4)})`,
      transition: options.dragging.value ? 'none' : 'transform var(--motion-quick) var(--ease-standard)',
      transformOrigin: stretchTransformOrigin(stretch, options.stretchAnchor.value),
      pointerEvents: committedListReturn ? ('none' as const) : ('auto' as const),
      '--detail-overlay-opacity':
        options.blockedBackSwipeActive.value || committedListReturn || options.returningToFeed.value
          ? '0'
          : options.surfaceProgress.value.toFixed(3),
    }
  })

  return {
    readerStyle,
  }
}
