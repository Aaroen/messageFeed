import { computed, type Ref } from 'vue'

type AppShellMotionOptions = {
  feedHeaderHeight: Ref<number>
  feedContentSpace: Ref<number>
  detailSurfaceProgress: Ref<number>
  freezeFeedBodyDuringTopRefresh: Ref<boolean>
  feedRefreshSettling: Ref<boolean>
  feedChromeSettling: Ref<boolean>
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
  const style = computed(() => {
    const detailUnderlayActive = options.detailReaderOpen.value && !options.detailReturningToFeed.value
    const underlayBlur = detailUnderlayActive ? options.detailSurfaceProgress.value * 7 : 0
    const underlayOpacity = detailUnderlayActive ? 1 - options.detailSurfaceProgress.value * 0.08 : 1
    const feedContentSpace = options.freezeFeedBodyDuringTopRefresh.value
      ? options.feedHeaderHeight.value
      : options.feedContentSpace.value
    const feedContentSettling =
      !options.freezeFeedBodyDuringTopRefresh.value &&
      (options.feedRefreshSettling.value || options.feedChromeSettling.value) &&
      !options.readerBackDragging.value
    const pageContentSettling = options.feedChromeSettling.value && !options.readerBackDragging.value
    const underlayTransition =
      'opacity var(--motion-fast) var(--ease-linear), filter var(--motion-fast) var(--ease-linear)'

    return {
      '--feed-header-height': `${options.feedHeaderHeight.value}px`,
      '--feed-header-space': cssPx(options.feedContentSpace.value),
      '--feed-content-padding-space': cssPx(feedContentSpace),
      '--page-content-padding-space': cssPx(options.feedContentSpace.value),
      '--feed-content-transition': [
        feedContentSettling
          ? 'padding-top var(--motion-chrome) var(--ease-emphasized)'
          : 'padding-top 0ms var(--ease-standard)',
        underlayTransition,
      ].join(', '),
      '--page-content-transition': [
        pageContentSettling
          ? 'padding-top var(--motion-chrome) var(--ease-emphasized)'
          : 'padding-top 0ms var(--ease-standard)',
        underlayTransition,
      ].join(', '),
      '--app-content-underlay-blur': `${underlayBlur.toFixed(2)}px`,
      '--app-content-underlay-opacity': underlayOpacity.toFixed(3),
    }
  })

  return {
    style,
  }
}
