import { computed, ref, type Ref } from 'vue'

type StretchAnchor = 'left' | 'right' | null

type PageContentMotionOptions = {
  pullOffset: Ref<number>
}

function cssPx(value: number) {
  return `${(Number.isFinite(value) ? value : 0).toFixed(2)}px`
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

export function usePageContentMotion(options: PageContentMotionOptions) {
  const sideOffset = ref(0)
  const sideStretch = ref(0)
  const stretchAnchor = ref<StretchAnchor>(null)

  const contentStyle = computed(() => ({
    transform: `${cssTranslate3d(sideOffset.value, options.pullOffset.value)} scaleX(${(
      1 + Math.abs(sideStretch.value)
    ).toFixed(4)})`,
    transformOrigin: stretchTransformOrigin(sideStretch.value, stretchAnchor.value),
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
    sideOffset,
    sideStretch,
    stretchAnchor,
    contentStyle,
    setSideOffset,
    setSideStretch,
    resetSideMotion,
    clearStretchAnchorIfIdle,
  }
}
