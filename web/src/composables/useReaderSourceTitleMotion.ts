import { computed, type Ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'
import type { RectSnapshot } from '@/composables/useReaderSession'

type ReaderSourceTitleMotionOptions = {
  revealReady: Ref<boolean>
  pullActive: Ref<boolean>
  titleProgress: Ref<number>
  revealProgress: Ref<number>
  nameOriginRect: Ref<RectSnapshot | null>
  nameTargetRect: Ref<RectSnapshot | null>
  nameMorphProgress: Ref<number>
  windowWidth: Ref<number>
  headerHeight: Ref<number>
  readerBackDragging: Ref<boolean>
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

export function useReaderSourceTitleMotion(options: ReaderSourceTitleMotionOptions) {
  const revealVisible = computed(() => options.revealReady.value && !options.pullActive.value)

  const nameMorphStyle = computed(() => {
    const origin = options.nameOriginRect.value
    const target = options.nameTargetRect.value
    const progress = clampProgress(options.nameMorphProgress.value)
    if (!origin || !target) {
      return {
        opacity: '0',
        filter: 'blur(0)',
        transform: 'translate3d(0, 0, 0)',
      }
    }

    const left = origin.left + (target.left - origin.left) * progress
    const top = origin.top + (target.top - origin.top) * progress
    const width = Math.max(origin.width, target.width, origin.width + (target.width - origin.width) * progress) + 18
    const size = 13 + (12 - 13) * progress
    const fadeOut = clampProgress((progress - 0.62) / 0.28)
    const opacity = clampProgress(1 - fadeOut)
    const blur = Math.sin(progress * Math.PI) * 1.6 + fadeOut * 2.2

    return {
      left: cssPx(left),
      top: cssPx(top),
      width: cssPx(width),
      opacity: opacity.toFixed(3),
      fontSize: `${size.toFixed(2)}px`,
      filter: `blur(${blur.toFixed(2)}px)`,
      transform: 'translate3d(0, 0, 0)',
      transition: options.readerBackDragging.value
        ? 'none'
        : 'left var(--motion-reader) var(--ease-standard), top var(--motion-reader) var(--ease-standard), width var(--motion-reader) var(--ease-standard), font-size var(--motion-reader) var(--ease-standard), opacity var(--motion-quick) var(--ease-standard), filter var(--motion-quick) var(--ease-standard)',
    }
  })

  const titleLayerStyle = computed(() => {
    const activeRevealProgress = revealVisible.value ? clampProgress(options.revealProgress.value) : 0
    const opacity = clampProgress(options.titleProgress.value) * (revealVisible.value ? 1 - activeRevealProgress : 1)

    return {
      opacity: opacity.toFixed(3),
      transform: 'translate3d(0, 0, 0)',
      filter: `blur(${(activeRevealProgress * 2).toFixed(2)}px)`,
      transition: options.readerBackDragging.value
        ? 'none'
        : 'opacity var(--motion-short) var(--ease-standard), filter var(--motion-short) var(--ease-standard), transform var(--motion-short) var(--ease-standard)',
    }
  })

  const titleTextStyle = computed(() => ({
    display: 'inline-block',
  }))

  const revealStyle = computed(() => {
    const progress = clampProgress(options.revealProgress.value)
    const left = options.windowWidth.value <= 720 ? 72 : 80
    const right = options.windowWidth.value <= 720 ? 104 : 120
    const top = (options.headerHeight.value - 44) / 2
    return {
      top: cssPx(top),
      left: cssPx(left),
      width: `calc(100vw - ${left + right}px)`,
      opacity: progress.toFixed(3),
      transform: `${cssTranslate3d(0, (1 - progress) * 12)} scale(${(
        0.965 +
        progress * 0.035
      ).toFixed(3)})`,
      filter: `blur(${((1 - progress) * 2.4).toFixed(2)}px)`,
      transition: options.readerBackDragging.value
        ? 'none'
        : 'opacity var(--motion-slow) var(--ease-standard), transform var(--motion-slow) var(--ease-emphasized), filter var(--motion-slow) var(--ease-standard)',
    }
  })

  return {
    revealVisible,
    nameMorphStyle,
    titleLayerStyle,
    titleTextStyle,
    revealStyle,
  }
}
