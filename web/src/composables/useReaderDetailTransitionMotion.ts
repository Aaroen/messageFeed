import { computed, type Ref } from 'vue'

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

export function useReaderDetailTransitionMotion(options: ReaderDetailTransitionMotionOptions) {
  const surfaceStyle = computed(() => {
    const origin = options.originRect.value
    const sourceExiting =
      options.restoringFromSourceReader.value ||
      (options.sourceExitProgress.value >= options.backExitProgress.value && options.sourceExitProgress.value > 0)
    const collapsedTarget = sourceExiting
      ? options.sourceItemTargetRect.value ?? options.fallbackTargetRect.value
      : origin
    const exitProgress = Math.max(options.backExitProgress.value, options.sourceExitProgress.value)
    const progress = options.surfaceProgress.value
    const surfaceMargin = options.surfaceMargin.value
    const expandedLeft = surfaceMargin
    const targetTop = options.expandedTop.value
    const expandedWidth = Math.max(1, options.windowWidth.value - surfaceMargin * 2)
    const targetHeight = Math.max(1, options.windowHeight.value - targetTop - surfaceMargin)
    const draggingToList =
      options.readerBackDragging.value &&
      (options.backExitProgress.value > 0 || options.sourceExitProgress.value > 0) &&
      !options.sourceReturnTargetPending.value
    const committedListReturn = options.committedListReturn()
    const suppressSourceReturnPreview = options.blockedBackSwipeActive.value

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
        opacity: suppressSourceReturnPreview ? '0' : clamp(opacity).toFixed(3),
        transform: cssTranslate3d(expandedLeft, targetTop + exitProgress * 18),
        transition: options.readerBackDragging.value ? 'none' : undefined,
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
      opacity: suppressSourceReturnPreview ? '0' : clamp(opacity).toFixed(3),
      transform: cssTranslate3d(x, y),
      borderRadius: cssPx(radius),
      transition: options.readerBackDragging.value ? 'none' : undefined,
    }
  })

  return {
    surfaceStyle,
  }
}
