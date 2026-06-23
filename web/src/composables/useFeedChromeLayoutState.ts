import { computed } from 'vue'

import {
  chromePhaseConsumesCollapsedLayout,
  clampProgress,
  feedContentTopOffset,
  feedHeaderHeightForWidth,
  feedTopScrollInset,
  feedVisibleContentTopOffset,
} from '@/composables/feedChromeMetrics'
import type { ChromePhase } from '@/composables/useChromeState'

type ReadableRef<T> = {
  readonly value: T
}

type FeedChromeLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  feedScrollTop: ReadableRef<number>
  topChromePhase: ReadableRef<ChromePhase>
  topChromeProgress: ReadableRef<number>
  viewSwipeActive: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  feedPullActive: ReadableRef<boolean>
  pullProgress: ReadableRef<number>
  feedTopPullStartedWithChrome: ReadableRef<boolean>
  refreshStartedWithChrome: ReadableRef<boolean>
  feedTopPulling: ReadableRef<boolean>
  feedContentCollapsed: ReadableRef<boolean>
  detailFeedHeaderReturnProgress: ReadableRef<number>
}

export function useFeedChromeLayoutState(options: FeedChromeLayoutStateOptions) {
  const headerHeight = computed(() => feedHeaderHeightForWidth(options.windowWidth.value))
  const contentTopOffset = computed(() => feedContentTopOffset(headerHeight.value))
  const visibleContentTopOffset = computed(() => feedVisibleContentTopOffset(headerHeight.value))
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const pullProgress = computed(() => clampProgress(options.pullProgress.value))
  const collapsedLayoutProgress = computed(() =>
    chromePhaseConsumesCollapsedLayout(options.topChromePhase.value) ? topChromeProgress.value : 0,
  )
  const feedAtVisibleTop = computed(() => options.feedScrollTop.value <= visibleContentTopOffset.value)
  const headerProgress = computed(() => {
    if (!options.isFeedRoute.value) {
      return topChromeProgress.value
    }

    if (options.feedPullActive.value) {
      return Math.max(topChromeProgress.value, pullProgress.value)
    }

    return topChromeProgress.value
  })
  const contentSpace = computed(() => {
    const collapsedRestoreSpace = headerHeight.value * collapsedLayoutProgress.value
    const pullRestoreSpace = headerHeight.value * Math.max(topChromeProgress.value, pullProgress.value)
    const feedLayoutChromeVisible = options.isFeedRoute.value && !options.feedContentCollapsed.value
    const feedTopInset = options.isFeedRoute.value
      ? feedTopScrollInset(options.feedScrollTop.value, headerHeight.value)
      : 0
    const visibleTopSpace =
      headerHeight.value + feedTopInset + visibleContentTopOffset.value - contentTopOffset.value
    const visibleHeaderAtFeedTop =
      options.isFeedRoute.value &&
      !options.detailReaderOpen.value &&
      !options.viewSwipeActive.value &&
      feedAtVisibleTop.value &&
      topChromeProgress.value > 0.04 &&
      !options.feedContentCollapsed.value
    const preserveVisibleTopSpace = (space: number) =>
      visibleHeaderAtFeedTop ? Math.max(space, visibleTopSpace) : space

    if (options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value) {
      return preserveVisibleTopSpace(headerHeight.value)
    }

    if (!options.feedTopPulling.value && !options.feedPullActive.value && feedLayoutChromeVisible) {
      return visibleTopSpace
    }

    if (options.feedTopPulling.value || options.feedPullActive.value) {
      return options.feedContentCollapsed.value ? pullRestoreSpace : preserveVisibleTopSpace(headerHeight.value)
    }

    if (options.feedContentCollapsed.value) {
      return collapsedRestoreSpace
    }

    return headerHeight.value
  })
  const freezeBodyDuringTopRefresh = computed(
    () => options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value,
  )
  const headerReturnProgress = computed(() =>
    options.isFeedRoute.value ? options.detailFeedHeaderReturnProgress.value : 0,
  )

  return {
    headerHeight,
    headerProgress,
    contentSpace,
    freezeBodyDuringTopRefresh,
    headerReturnProgress,
  }
}
