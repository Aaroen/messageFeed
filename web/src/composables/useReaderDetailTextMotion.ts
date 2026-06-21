import { computed, type Ref } from 'vue'

type ReaderDetailTextMotionOptions = {
  surfaceProgress: Ref<number>
  sourceListTitleProgress: Ref<number>
  headerFeedTitleProgress: Ref<number>
  feedHeaderReturnProgress: Ref<number>
  headerTitleSwapping: Ref<boolean>
  headerSwapProgress: Ref<number>
  sourceLabelOpacity: Ref<number>
  sourceLabelBlur: Ref<number>
  readerBackDragging: Ref<boolean>
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

const morphTextTransition =
  'opacity var(--motion-fast) var(--ease-linear), transform var(--motion-quick) var(--ease-standard)'

export function useReaderDetailTextMotion(options: ReaderDetailTextMotionOptions) {
  const morphTextStyle = computed(() => {
    const progress = options.surfaceProgress.value
    const committedListReturn = options.committedListReturn()
    const summaryOpacity = clamp((0.56 - progress) / 0.18)
    const textOpacity = clamp((0.74 - progress) / 0.18)
    return {
      opacity: committedListReturn ? '0' : '1',
      '--morph-title-size': `${(18 + progress * 10).toFixed(2)}px`,
      '--morph-text-opacity': textOpacity.toFixed(3),
      '--morph-summary-opacity': summaryOpacity.toFixed(3),
      '--morph-source-pointer-events': textOpacity > 0.12 ? 'auto' : 'none',
      transform: cssTranslate3d(0, progress * -4),
      transition: options.readerBackDragging.value || committedListReturn ? 'none' : morphTextTransition,
    }
  })

  const headerTitleStyle = computed(() => {
    const sourceListTitleProgress = options.sourceListTitleProgress.value
    const opacity =
      sourceListTitleProgress > 0
        ? sourceListTitleProgress
        : options.headerFeedTitleProgress.value * (1 - options.feedHeaderReturnProgress.value)
    return {
      opacity: opacity.toFixed(3),
      transform: cssTranslate3d(0, (1 - opacity) * 8),
      filter: `blur(${((1 - opacity) * 3.2).toFixed(2)}px)`,
      transition: options.readerBackDragging.value ? 'none' : undefined,
    }
  })

  const headerCurrentTextStyle = computed(() => {
    const progress = options.headerTitleSwapping.value ? options.headerSwapProgress.value : 1
    return {
      opacity: progress.toFixed(3),
      filter: `blur(${((1 - progress) * 2.8).toFixed(2)}px)`,
      transform: cssTranslate3d(0, (1 - progress) * 6),
      transition: options.readerBackDragging.value
        ? 'none'
        : 'opacity var(--motion-normal) var(--ease-standard), filter var(--motion-normal) var(--ease-standard), transform var(--motion-normal) var(--ease-emphasized)',
    }
  })

  const headerPreviousTextStyle = computed(() => {
    const progress = options.headerSwapProgress.value
    return {
      opacity: (1 - progress).toFixed(3),
      filter: `blur(${(progress * 3.2).toFixed(2)}px)`,
      transform: cssTranslate3d(0, progress * -6),
      transition: options.readerBackDragging.value
        ? 'none'
        : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-normal) var(--ease-standard), transform var(--motion-normal) var(--ease-emphasized)',
    }
  })

  const inlineSourceStyle = computed(() => ({
    opacity: options.sourceLabelOpacity.value.toFixed(3),
    filter: `blur(${options.sourceLabelBlur.value.toFixed(2)}px)`,
    transform: 'translate3d(0, 0, 0)',
    transition: options.readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-short) var(--ease-standard)',
  }))

  const morphSourceLabelStyle = computed(() => ({
    opacity: options.sourceLabelOpacity.value.toFixed(3),
    filter: `blur(${options.sourceLabelBlur.value.toFixed(2)}px)`,
    transition: options.readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-short) var(--ease-standard)',
  }))

  return {
    morphTextStyle,
    headerTitleStyle,
    headerCurrentTextStyle,
    headerPreviousTextStyle,
    inlineSourceStyle,
    morphSourceLabelStyle,
  }
}
