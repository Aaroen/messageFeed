import { computed } from 'vue'

import { clampProgress } from '@/composables/feedChromeMetrics'

type ReadableRef<T> = {
  readonly value: T
}

type PullActivityStateOptions = {
  isFeedRoute: ReadableRef<boolean>
  pagePullRefreshing: ReadableRef<boolean>
  pagePullOffset: ReadableRef<number>
  sourceReaderOpen: ReadableRef<boolean>
  getFeedPullActive: () => boolean
  getFeedPullRefreshing: () => boolean
  getFeedPullOffset: () => number
  getFeedPullViewKey: () => string
}

function pullProgressFromOffset(offset: number) {
  return clampProgress(offset / 76)
}

export function usePullActivityState(options: PullActivityStateOptions) {
  const pullViewKey = computed(() => options.getFeedPullViewKey())
  const feedPullHasMotion = computed(
    () => options.getFeedPullActive() || options.getFeedPullRefreshing() || options.getFeedPullOffset() > 1,
  )
  const feedPullBelongsToSource = computed(() => pullViewKey.value.startsWith('source:'))
  const feedActive = computed(
    () =>
      options.isFeedRoute.value &&
      !options.sourceReaderOpen.value &&
      !feedPullBelongsToSource.value &&
      feedPullHasMotion.value,
  )
  const pageActive = computed(
    () => !options.isFeedRoute.value && (options.pagePullRefreshing.value || options.pagePullOffset.value > 1),
  )
  const sourceActive = computed(
    () =>
      options.sourceReaderOpen.value &&
      feedPullBelongsToSource.value &&
      feedPullHasMotion.value,
  )
  const feedOrSourceActive = computed(() => feedActive.value || sourceActive.value)
  const feedProgress = computed(() => pullProgressFromOffset(options.getFeedPullOffset()))
  const sourceProgress = computed(() => pullProgressFromOffset(options.getFeedPullOffset()))

  return {
    feedActive,
    pageActive,
    sourceActive,
    feedOrSourceActive,
    feedProgress,
    sourceProgress,
  }
}
