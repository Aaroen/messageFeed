import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type FeedChromeVisibilityStateOptions = {
  isFeedRoute: ReadableRef<boolean>
  topChromeProgress: ReadableRef<number>
  feedHeaderProgress: ReadableRef<number>
  feedPullActive: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  feedHeaderReturnProgress: ReadableRef<number>
  sourceReaderOpen: ReadableRef<boolean>
  detailChromeVisible: ReadableRef<boolean>
}

export function useFeedChromeVisibilityState(options: FeedChromeVisibilityStateOptions) {
  const feedTabsVisible = computed(() => options.isFeedRoute.value && options.topChromeProgress.value > 0.04)
  const feedTabsLayerHidden = computed(() => {
    if (!options.isFeedRoute.value || options.feedPullActive.value) {
      return true
    }
    if (options.detailReaderOpen.value) {
      return options.feedHeaderReturnProgress.value <= 0.001
    }
    return !feedTabsVisible.value
  })
  const detailHeaderVisible = computed(
    () => options.detailChromeVisible.value && options.topChromeProgress.value > 0.04,
  )
  const feedCornerHidden = computed(
    () =>
      (options.sourceReaderOpen.value && !options.detailChromeVisible.value) ||
      (options.detailChromeVisible.value && !detailHeaderVisible.value) ||
      (!options.detailChromeVisible.value &&
        options.isFeedRoute.value &&
        (options.feedPullActive.value || options.feedHeaderProgress.value <= 0.05)),
  )
  const headerDetailLayoutActive = computed(
    () =>
      options.detailChromeVisible.value ||
      (options.detailReaderOpen.value && options.isFeedRoute.value && options.feedHeaderReturnProgress.value > 0.001),
  )

  return {
    feedTabsVisible,
    feedTabsLayerHidden,
    feedCornerHidden,
    detailHeaderVisible,
    headerDetailLayoutActive,
  }
}
