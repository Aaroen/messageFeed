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

function combinedTransition(...transitions: string[]) {
  const activeTransitions = transitions.filter((transition) => transition && transition !== 'none')
  return activeTransitions.length ? activeTransitions.join(', ') : 'none'
}

export function useAppShellMotion(options: AppShellMotionOptions) {
  const contentState = computed(() => {
    const detailSurfaceProgress = clampProgress(options.detailSurfaceProgress.value)
    const detailUnderlayActive = options.detailReaderOpen.value && !options.detailReturningToFeed.value
    const underlayBlur = detailUnderlayActive ? detailSurfaceProgress * 7 : 0
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
    const underlayTransition =
      'opacity var(--motion-fast) var(--ease-linear), filter var(--motion-fast) var(--ease-linear)'
    const contentSpaceTransition = refreshShiftSettling
      ? 'padding-top var(--motion-refresh-complete) var(--ease-emphasized)'
      : chromeShiftSettling
        ? 'padding-top var(--motion-chrome) var(--ease-emphasized)'
        : 'none'

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
      underlayBlur,
      underlayOpacity,
      transition: underlayTransition,
      contentSpaceTransition,
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
      paddingTop: `calc(${cssPx(state.feedContentSpace)} + var(--app-content-top-offset, 10px))`,
      opacity: state.underlayOpacity.toFixed(3),
      filter: `blur(${state.underlayBlur.toFixed(2)}px)`,
      transition: combinedTransition(state.transition, state.contentSpaceTransition),
    }
  })

  const pageContentStyle = computed(() => {
    const state = contentState.value
    return {
      '--page-content-shift': cssPx(state.pageContentShift),
      '--page-content-shift-transition': state.pageContentShiftTransition,
      paddingTop: `calc(${cssPx(state.pageContentSpace)} + var(--app-content-top-offset, 10px))`,
      opacity: state.underlayOpacity.toFixed(3),
      filter: `blur(${state.underlayBlur.toFixed(2)}px)`,
      transition: combinedTransition(state.transition, state.contentSpaceTransition),
    }
  })

  return {
    style,
    feedContentStyle,
    pageContentStyle,
  }
}
