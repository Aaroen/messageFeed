import { computed, type Ref } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'
import type { RectSnapshot } from '@/composables/useReaderSession'

type ReaderDetailTransitionMotionOptions = {
  originRect: Ref<RectSnapshot | null>
  sourceItemTargetRect: Ref<RectSnapshot | null>
  fallbackTargetRect: Ref<RectSnapshot>
  restoringFromSourceReader: Ref<boolean>
  sourceExitProgress: Ref<number>
  backExitProgress: Ref<number>
  surfaceProgress: Ref<number>
  surfaceMargin: Ref<number>
  expandedTop: Ref<number>
  windowWidth: Ref<number>
  windowHeight: Ref<number>
  darkTheme: Ref<boolean>
  readerBackDragging: Ref<boolean>
  sourceReturnTargetPending: Ref<boolean>
  blockedBackSwipeActive: Ref<boolean>
  returningToFeed: Ref<boolean>
  entrySettling: Ref<boolean>
  chromeSettling: Ref<boolean>
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

const chromeSettlingTransition =
  'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-normal) var(--ease-standard)'

const entrySettlingTransition = [
  'transform var(--motion-reader) var(--ease-standard)',
  'height var(--motion-reader) var(--ease-standard)',
  'width var(--motion-reader) var(--ease-standard)',
  'opacity var(--motion-normal) var(--ease-standard)',
  'border-radius var(--motion-reader) var(--ease-standard)',
].join(', ')

function surfaceTransition(dragging: boolean, entrySettling: boolean, chromeSettling: boolean) {
  if (dragging) {
    return 'none'
  }
  if (chromeSettling) {
    return chromeSettlingTransition
  }
  if (entrySettling) {
    return entrySettlingTransition
  }
  return undefined
}

export function useReaderDetailTransitionMotion(options: ReaderDetailTransitionMotionOptions) {
  const surfaceStyle = computed(() => {
    const origin = options.originRect.value
    const backExitProgress = clampProgress(options.backExitProgress.value)
    const sourceExitProgress = clampProgress(options.sourceExitProgress.value)
    const progress = clampProgress(options.surfaceProgress.value)
    const sourceExiting =
      options.restoringFromSourceReader.value ||
      (sourceExitProgress >= backExitProgress && sourceExitProgress > 0)
    const collapsedTarget = sourceExiting
      ? options.sourceItemTargetRect.value ?? options.fallbackTargetRect.value
      : origin
    const exitProgress = Math.max(backExitProgress, sourceExitProgress)
    const surfaceMargin = options.surfaceMargin.value
    const expandedLeft = surfaceMargin
    const targetTop = options.expandedTop.value
    const expandedWidth = Math.max(1, options.windowWidth.value - surfaceMargin * 2)
    const targetHeight = Math.max(1, options.windowHeight.value - targetTop - surfaceMargin)
    const draggingToList =
      options.readerBackDragging.value &&
      (backExitProgress > 0 || sourceExitProgress > 0) &&
      !options.sourceReturnTargetPending.value
    const committedListReturn = options.committedListReturn()
    const suppressSourceReturnPreview = options.blockedBackSwipeActive.value
    const transition = surfaceTransition(
      options.readerBackDragging.value,
      options.entrySettling.value,
      options.chromeSettling.value,
    )

    if (!collapsedTarget) {
      const opacity =
        draggingToList
          ? 1
          : committedListReturn || options.returningToFeed.value
            ? progress
            : 1 - exitProgress * 0.28
      return {
        width: cssPx(expandedWidth),
        height: cssPx(targetHeight),
        opacity: suppressSourceReturnPreview ? '0' : clampProgress(opacity).toFixed(3),
        transform: cssTranslate3d(expandedLeft, targetTop + exitProgress * 18),
        transition,
        borderRadius: cssPx(16 - exitProgress * 4),
      }
    }

    const width = collapsedTarget.width + (expandedWidth - collapsedTarget.width) * progress
    const height = collapsedTarget.height + (targetHeight - collapsedTarget.height) * progress
    const x = collapsedTarget.left + (expandedLeft - collapsedTarget.left) * progress
    const y = collapsedTarget.top + (targetTop - collapsedTarget.top) * progress
    const radius = 12 + 4 * progress
    const minimumSurfaceOpacity = options.darkTheme.value ? 0.64 : 0.36
    const opacity =
      draggingToList
        ? 1
        : committedListReturn || options.returningToFeed.value
          ? progress
          : minimumSurfaceOpacity + progress * (1 - minimumSurfaceOpacity)

    return {
      width: cssPx(width),
      height: cssPx(height),
      opacity: suppressSourceReturnPreview ? '0' : clampProgress(opacity).toFixed(3),
      transform: cssTranslate3d(x, y),
      borderRadius: cssPx(radius),
      transition,
    }
  })

  return {
    surfaceStyle,
  }
}
