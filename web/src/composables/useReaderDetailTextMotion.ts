import { computed, type Ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

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
  const morphState = computed(() => {
    const progress = clampProgress(options.surfaceProgress.value)
    const committedListReturn = options.committedListReturn()
    const summaryOpacity = clampProgress((0.56 - progress) / 0.18)
    const textOpacity = clampProgress((0.74 - progress) / 0.18)
    return {
      progress,
      committedListReturn,
      summaryOpacity,
      textOpacity,
    }
  })

  const morphTextStyle = computed(() => {
    const state = morphState.value
    return {
      opacity: state.committedListReturn ? '0' : '1',
      transform: cssTranslate3d(0, state.progress * -4),
      transition:
        options.readerBackDragging.value || state.committedListReturn ? 'none' : morphTextTransition,
    }
  })

  const morphMetaStyle = computed(() => ({
    opacity: morphState.value.textOpacity.toFixed(3),
  }))

  const morphTitleStyle = computed(() => ({
    fontSize: `${(18 + morphState.value.progress * 10).toFixed(2)}px`,
    opacity: morphState.value.textOpacity.toFixed(3),
  }))

  const morphSummaryStyle = computed(() => ({
    opacity: morphState.value.summaryOpacity.toFixed(3),
  }))

  const headerTitleStyle = computed(() => {
    const sourceListTitleProgress = clampProgress(options.sourceListTitleProgress.value)
    const opacity =
      sourceListTitleProgress > 0
        ? sourceListTitleProgress
        : clampProgress(options.headerFeedTitleProgress.value) *
          (1 - clampProgress(options.feedHeaderReturnProgress.value))
    return {
      opacity: opacity.toFixed(3),
      transform: cssTranslate3d(0, (1 - opacity) * 8),
      filter: `blur(${((1 - opacity) * 3.2).toFixed(2)}px)`,
      transition: options.readerBackDragging.value ? 'none' : undefined,
    }
  })

  const headerCurrentTextStyle = computed(() => {
    const progress = options.headerTitleSwapping.value ? clampProgress(options.headerSwapProgress.value) : 1
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
    const progress = clampProgress(options.headerSwapProgress.value)
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
    opacity: clampProgress(options.sourceLabelOpacity.value).toFixed(3),
    filter: `blur(${cssNumber(Math.max(0, options.sourceLabelBlur.value))}px)`,
    transform: 'translate3d(0, 0, 0)',
    transition: options.readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-short) var(--ease-standard)',
  }))

  const morphSourceLabelStyle = computed(() => ({
    opacity: clampProgress(options.sourceLabelOpacity.value).toFixed(3),
    filter: `blur(${cssNumber(Math.max(0, options.sourceLabelBlur.value))}px)`,
    pointerEvents: morphState.value.textOpacity > 0.12 ? ('auto' as const) : ('none' as const),
    transition: options.readerBackDragging.value
      ? 'none'
      : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-short) var(--ease-standard)',
  }))

  return {
    morphTextStyle,
    morphMetaStyle,
    morphTitleStyle,
    morphSummaryStyle,
    headerTitleStyle,
    headerCurrentTextStyle,
    headerPreviousTextStyle,
    inlineSourceStyle,
    morphSourceLabelStyle,
  }
}
