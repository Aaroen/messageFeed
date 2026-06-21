import { computed, type Ref } from 'vue'

type ReaderDetailContentMotionOptions = {
  surfaceProgress: Ref<number>
  sourceExitProgress: Ref<number>
  frameMinHeight: Ref<number>
  frameContentHeight: Ref<number>
  dragging: Ref<boolean>
  committedListReturn: () => boolean
}

function clamp(value: number, min = 0, max = 1) {
  if (!Number.isFinite(value)) {
    return min
  }
  return Math.min(Math.max(value, min), max)
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

export function useReaderDetailContentMotion(options: ReaderDetailContentMotionOptions) {
  const contentStyle = computed(() => {
    const progress = options.surfaceProgress.value
    const sourceExitProgress = options.sourceExitProgress.value
    const committedListReturn = options.committedListReturn()
    const opacity = sourceExitProgress > 0 ? 1 : clamp((progress - 0.56) / 0.22)
    const bodyOpacity = sourceExitProgress > 0 ? clamp((0.72 - sourceExitProgress) / 0.32) : 1
    const inlineMetaOpacity = sourceExitProgress > 0 ? clamp((0.24 - sourceExitProgress) / 0.18) : 1
    return {
      opacity: committedListReturn ? '0' : opacity.toFixed(3),
      '--detail-body-opacity': bodyOpacity.toFixed(3),
      '--detail-inline-meta-opacity': inlineMetaOpacity.toFixed(3),
      '--detail-frame-min-height': cssPx(options.frameMinHeight.value),
      '--detail-frame-content-height': cssPx(
        Math.max(options.frameMinHeight.value, options.frameContentHeight.value),
      ),
      transform:
        sourceExitProgress > 0 ? cssTranslate3d(0, 0) : cssTranslate3d(0, (1 - progress) * 8),
      visibility: !committedListReturn && opacity > 0.01 ? ('visible' as const) : ('hidden' as const),
      transition: options.dragging.value || committedListReturn ? 'none' : undefined,
    }
  })

  return {
    contentStyle,
  }
}
