import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type FeedChromeLayoutStateOptions = {
  windowWidth: ReadableRef<number>
  isFeedRoute: ReadableRef<boolean>
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
  const headerHeight = computed(() => (options.windowWidth.value <= 720 ? 78 : 86))
  const headerProgress = computed(() => {
    if (!options.isFeedRoute.value) {
      return options.topChromeProgress.value
    }

    if (options.feedPullActive.value) {
      return Math.max(options.topChromeProgress.value, options.pullProgress.value)
    }

    return options.topChromeProgress.value
  })
  const contentSpace = computed(() => {
    if (options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value) {
      return headerHeight.value
    }

    if (options.feedTopPulling.value) {
      return headerHeight.value * (options.feedPullActive.value ? options.pullProgress.value : options.topChromeProgress.value)
    }

    if (options.feedPullActive.value) {
      return headerHeight.value * Math.max(options.topChromeProgress.value, options.pullProgress.value)
    }

    if (options.feedContentCollapsed.value) {
      return 0
    }

    return headerHeight.value
  })
  const freezeBodyDuringTopRefresh = computed(
    () => options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value,
  )
  const topChromeIsVisiblyOpen = computed(
    () => !options.feedContentCollapsed.value || options.topChromeProgress.value > 0.04,
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
