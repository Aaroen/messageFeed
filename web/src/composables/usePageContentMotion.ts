import { computed, readonly, ref } from 'vue'

type StretchAnchor = 'left' | 'right' | null
type ReadableRef<T> = {
  readonly value: T
}

type PageContentMotionOptions = {
  pullOffset: ReadableRef<number>
  settling: ReadableRef<boolean>
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
}

function cssTranslate3dWithPageShift(x: number, y: number) {
  return `translate3d(${cssPx(x)}, calc(var(--page-content-shift, 0px) + ${cssPx(y)}), 0)`
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

export function usePageContentMotion(options: PageContentMotionOptions) {
  const sideOffset = ref(0)
  const sideStretch = ref(0)
  const stretchAnchor = ref<StretchAnchor>(null)

  const contentStyle = computed(() => ({
    transform: `${cssTranslate3dWithPageShift(sideOffset.value, options.pullOffset.value)} scaleX(${(
      1 + Math.abs(sideStretch.value)
    ).toFixed(4)})`,
    transformOrigin: stretchTransformOrigin(sideStretch.value, stretchAnchor.value),
    transition: options.settling.value
      ? 'transform var(--motion-refresh-complete) var(--ease-emphasized)'
      : 'var(--page-content-shift-transition, none)',
  }))

  function setSideOffset(nextOffset: number) {
    sideOffset.value = Number.isFinite(nextOffset) ? nextOffset : 0
  }

  function setSideStretch(nextStretch: number) {
    sideStretch.value = Number.isFinite(nextStretch) ? nextStretch : 0
    if (sideStretch.value > 0) {
      stretchAnchor.value = 'left'
    } else if (sideStretch.value < 0) {
      stretchAnchor.value = 'right'
    }
  }

  function resetSideMotion() {
    sideOffset.value = 0
    sideStretch.value = 0
  }

  function clearStretchAnchorIfIdle(active: boolean) {
    if (!active && sideStretch.value === 0) {
      stretchAnchor.value = null
    }
  }

  return {
    sideStretch: readonly(sideStretch),
    contentStyle,
    setSideOffset,
    setSideStretch,
    resetSideMotion,
    clearStretchAnchorIfIdle,
  }
}
