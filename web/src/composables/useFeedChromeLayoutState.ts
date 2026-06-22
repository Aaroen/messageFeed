import { computed } from 'vue'

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
  const headerHeight = computed(() => (options.windowWidth.value <= 720 ? 78 : 86))
  const contentTopOffset = computed(() => (headerHeight.value <= 78 ? 8 : 10))
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
    const collapsedRestoreSpace = headerHeight.value * options.topChromeProgress.value
    const pullRestoreSpace = headerHeight.value * Math.max(
      options.topChromeProgress.value,
      options.pullProgress.value,
    )
    const feedAtTop = options.isFeedRoute.value && options.feedScrollTop.value <= contentTopOffset.value
    const feedTopChromeVisible = headerProgress.value > 0.04 || !options.feedContentCollapsed.value

    if (options.feedTopPullStartedWithChrome.value || options.refreshStartedWithChrome.value) {
      return headerHeight.value
    }

    if (!options.feedTopPulling.value && !options.feedPullActive.value && feedAtTop && feedTopChromeVisible) {
      return headerHeight.value
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
