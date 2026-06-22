import { computed } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

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
  const topChromeProgress = computed(() => clampProgress(options.topChromeProgress.value))
  const detailHeaderVisible = computed(
    () => options.detailChromeVisible.value && topChromeProgress.value > 0.04,
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
    feedCornerHidden,
    detailHeaderVisible,
    headerDetailLayoutActive,
  }
}
