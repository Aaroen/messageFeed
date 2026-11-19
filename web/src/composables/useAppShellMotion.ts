import { computed } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type AppShellMotionOptions = {
  feedHeaderHeight: ReadableRef<number>
  feedContentSpace: ReadableRef<number>
  detailSurfaceProgress: ReadableRef<number>
  feedRefreshSettling: ReadableRef<boolean>
  feedChromeSettling: ReadableRef<boolean>
  feedTopPulling: ReadableRef<boolean>
  readerBackDragging: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailReturningToFeed: ReadableRef<boolean>
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

const contentTopGapVar = 'var(--app-content-top-offset, var(--app-content-top-gap, 14px))'
const underlayTransition = 'opacity var(--motion-fast) var(--ease-linear)'

export function useAppShellMotion(options: AppShellMotionOptions) {
  const contentState = computed(() => {
    const detailSurfaceProgress = clampProgress(options.detailSurfaceProgress.value)
    const detailUnderlayActive = options.detailReaderOpen.value && !options.detailReturningToFeed.value
    const underlayOpacity = detailUnderlayActive ? 1 - detailSurfaceProgress * 0.08 : 1
    const feedContentSpace = options.feedContentSpace.value
    const refreshShiftSettling =
      options.feedRefreshSettling.value &&
      !options.feedTopPulling.value &&
      !options.readerBackDragging.value
    const chromeShiftSettling =
      options.feedChromeSettling.value &&
      !options.feedTopPulling.value &&
      !options.readerBackDragging.value
    return {
      feedContentSpace,
      pageContentSpace: feedContentSpace,
      feedContentShift: 0,
      pageContentShift: 0,
      feedContentShiftTransition: refreshShiftSettling
        ? 'transform var(--motion-refresh-complete) var(--ease-emphasized)'
        : chromeShiftSettling
        ? 'transform var(--motion-chrome) var(--ease-emphasized)'
        : 'none',
      pageContentShiftTransition: refreshShiftSettling
        ? 'transform var(--motion-refresh-complete) var(--ease-emphasized)'
        : chromeShiftSettling
        ? 'transform var(--motion-chrome) var(--ease-emphasized)'
        : 'none',
      underlayOpacity,
    }
  })

  const style = computed(() => ({
    '--feed-header-height': `${options.feedHeaderHeight.value}px`,
    '--feed-header-space': cssPx(options.feedHeaderHeight.value),
  }))

  const feedContentStyle = computed(() => {
    const state = contentState.value
    return {
      '--feed-content-shift': cssPx(state.feedContentShift),
      '--feed-content-shift-transition': state.feedContentShiftTransition,
      paddingTop: `calc(${cssPx(state.feedContentSpace)} + ${contentTopGapVar})`,
      opacity: state.underlayOpacity.toFixed(3),
      transition: underlayTransition,
    }
  })

  const pageContentStyle = computed(() => {
    const state = contentState.value
    return {
      '--page-content-shift': cssPx(state.pageContentShift),
      '--page-content-shift-transition': state.pageContentShiftTransition,
      paddingTop: `calc(${cssPx(state.pageContentSpace)} + ${contentTopGapVar})`,
      opacity: state.underlayOpacity.toFixed(3),
      transition: underlayTransition,
    }
  })

  return {
    style,
    feedContentStyle,
    pageContentStyle,
  }
}
