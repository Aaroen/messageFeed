import { computed, type Ref } from 'vue'

type StretchAnchor = 'left' | 'right' | null

type ReaderSourceSurfaceMotionOptions = {
  feedHeaderHeight: Ref<number>
  headerSpace: Ref<number>
  darkTheme: Ref<boolean>
  visible: Ref<boolean>
  underDetail: Ref<boolean>
  revealProgress: Ref<number>
  offset: Ref<number>
  stretch: Ref<number>
  stretchAnchor: Ref<StretchAnchor>
  dragging: Ref<boolean>
  blocksGestures: () => boolean
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
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

export function useReaderSourceSurfaceMotion(options: ReaderSourceSurfaceMotionOptions) {
  const surfaceStyle = computed(() => {
    const overlayBaseOpacity = options.darkTheme.value ? 0.48 : 0.34
    const sourceStretch = options.stretch.value
    return {
      '--feed-header-height': `${options.feedHeaderHeight.value}px`,
      '--source-header-space': cssPx(options.headerSpace.value),
      zIndex: options.underDetail.value ? 96 : options.visible.value ? 110 : 90,
      opacity: !options.visible.value
        ? '0'
        : options.underDetail.value
          ? (overlayBaseOpacity + options.revealProgress.value * (1 - overlayBaseOpacity)).toFixed(3)
          : '1',
      pointerEvents:
        !options.visible.value || options.blocksGestures() ? ('none' as const) : ('auto' as const),
      transform: `${cssTranslate3d(options.offset.value, 0)} scaleX(${(1 + Math.abs(sourceStretch)).toFixed(4)})`,
      transformOrigin: stretchTransformOrigin(sourceStretch, options.stretchAnchor.value),
      transition: options.dragging.value
        ? 'none'
        : 'opacity var(--motion-normal) var(--ease-standard), transform var(--motion-normal) var(--ease-standard)',
    }
  })

  return {
    surfaceStyle,
  }
}
