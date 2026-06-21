import { computed, type Ref } from 'vue'

type AppShellMotionOptions = {
  feedHeaderHeight: Ref<number>
  feedContentSpace: Ref<number>
  detailSurfaceProgress: Ref<number>
  freezeFeedBodyDuringTopRefresh: Ref<boolean>
  feedRefreshSettling: Ref<boolean>
  feedChromeSettling: Ref<boolean>
  readerBackDragging: Ref<boolean>
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

export function useAppShellMotion(options: AppShellMotionOptions) {
  const style = computed(() => {
    const feedContentSpace = options.freezeFeedBodyDuringTopRefresh.value
      ? options.feedHeaderHeight.value
      : options.feedContentSpace.value
    const feedContentSettling =
      !options.freezeFeedBodyDuringTopRefresh.value &&
      (options.feedRefreshSettling.value || options.feedChromeSettling.value) &&
      !options.readerBackDragging.value
    const pageContentSettling = options.feedChromeSettling.value && !options.readerBackDragging.value

    return {
      '--feed-header-height': `${options.feedHeaderHeight.value}px`,
      '--feed-header-space': cssPx(options.feedContentSpace.value),
      '--feed-content-padding-space': cssPx(feedContentSpace),
      '--page-content-padding-space': cssPx(options.feedContentSpace.value),
      '--feed-content-padding-transition': feedContentSettling
        ? 'padding-top var(--motion-chrome) var(--ease-emphasized)'
        : 'none',
      '--page-content-padding-transition': pageContentSettling
        ? 'padding-top var(--motion-chrome) var(--ease-emphasized)'
        : 'none',
      '--detail-underlay-blur': `${(options.detailSurfaceProgress.value * 7).toFixed(2)}px`,
      '--detail-underlay-opacity': (1 - options.detailSurfaceProgress.value * 0.08).toFixed(3),
    }
  })

  return {
    style,
  }
}
