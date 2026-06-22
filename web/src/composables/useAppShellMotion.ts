import { computed, type Ref } from 'vue'

type AppShellMotionOptions = {
  feedHeaderHeight: Ref<number>
  feedContentSpace: Ref<number>
  detailSurfaceProgress: Ref<number>
  feedRefreshSettling: Ref<boolean>
  feedChromeSettling: Ref<boolean>
  feedTopPulling: Ref<boolean>
  readerBackDragging: Ref<boolean>
  detailReaderOpen: Ref<boolean>
  detailReturningToFeed: Ref<boolean>
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

export function useAppShellMotion(options: AppShellMotionOptions) {
  const contentState = computed(() => {
    const detailUnderlayActive = options.detailReaderOpen.value && !options.detailReturningToFeed.value
    const underlayBlur = detailUnderlayActive ? options.detailSurfaceProgress.value * 7 : 0
    const underlayOpacity = detailUnderlayActive ? 1 - options.detailSurfaceProgress.value * 0.08 : 1
    const feedContentSpace = options.feedContentSpace.value
    const feedContentSettling =
      options.feedRefreshSettling.value && !options.feedTopPulling.value && !options.readerBackDragging.value
    const underlayTransition =
      'opacity var(--motion-fast) var(--ease-linear), filter var(--motion-fast) var(--ease-linear)'

    return {
      feedContentSpace,
      pageContentSpace: feedContentSpace,
      feedContentShift: feedContentSpace - options.feedHeaderHeight.value,
      feedContentShiftTransition: feedContentSettling
        ? 'transform var(--motion-chrome) var(--ease-emphasized)'
        : 'none',
      underlayBlur,
      underlayOpacity,
      transition: underlayTransition,
    }
  })

  const style = computed(() => ({
    '--feed-header-height': `${options.feedHeaderHeight.value}px`,
    '--feed-header-space': cssPx(options.feedContentSpace.value),
  }))

  const feedContentStyle = computed(() => {
    const state = contentState.value
    return {
      '--feed-content-shift': cssPx(state.feedContentShift),
      '--feed-content-shift-transition': state.feedContentShiftTransition,
      paddingTop: `calc(${cssPx(options.feedHeaderHeight.value)} + var(--app-content-top-offset, 10px))`,
      opacity: state.underlayOpacity.toFixed(3),
      filter: `blur(${state.underlayBlur.toFixed(2)}px)`,
      transition: state.transition,
    }
  })

  const pageContentStyle = computed(() => {
    const state = contentState.value
    return {
      paddingTop: `calc(${cssPx(state.pageContentSpace)} + var(--app-content-top-offset, 10px))`,
      opacity: state.underlayOpacity.toFixed(3),
      filter: `blur(${state.underlayBlur.toFixed(2)}px)`,
      transition: state.transition,
    }
  })

  return {
    style,
    feedContentStyle,
    pageContentStyle,
  }
}
