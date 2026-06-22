import { computed } from 'vue'

import {
  clampProgress,
  feedContentTopOffset,
  feedHeaderHeightForWidth,
  feedTopScrollInset,
} from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type FeedChromeLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
  feedScrollTop: ReadableRef<number>
  topChromeProgress: ReadableRef<number>
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
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const pullProgress = computed(() => clampProgress(options.pullProgress.value))
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
    const collapsedRestoreSpace = headerHeight.value * topChromeProgress.value
    const pullRestoreSpace = headerHeight.value * Math.max(topChromeProgress.value, pullProgress.value)
    const feedAtTop = options.isFeedRoute.value && options.feedScrollTop.value <= contentTopOffset.value
    const feedTopChromeVisible = headerProgress.value > 0.04 || !options.feedContentCollapsed.value
    const feedTopInset = options.isFeedRoute.value
      ? feedTopScrollInset(options.feedScrollTop.value, headerHeight.value)
      : 0
    const visibleTopSpace = headerHeight.value + feedTopInset

    if (options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value) {
      return headerHeight.value
    }

    if (!options.feedTopPulling.value && !options.feedPullActive.value && feedAtTop && feedTopChromeVisible) {
      return visibleTopSpace
    }

    if (options.feedTopPulling.value || options.feedPullActive.value) {
      return options.feedContentCollapsed.value ? pullRestoreSpace : headerHeight.value
    }

    if (options.feedContentCollapsed.value) {
      return collapsedRestoreSpace
    }

    return headerHeight.value
  })
  const freezeBodyDuringTopRefresh = computed(
    () => options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value,
  )
  const topChromeIsVisiblyOpen = computed(
    () => !options.feedContentCollapsed.value || topChromeProgress.value > 0.04,
  )
  const headerReturnProgress = computed(() =>
    options.isFeedRoute.value ? options.detailFeedHeaderReturnProgress.value : 0,
  )

  return {
    headerHeight,
    headerProgress,
    contentSpace,
    freezeBodyDuringTopRefresh,
    topChromeIsVisiblyOpen,
    headerReturnProgress,
  }
}
