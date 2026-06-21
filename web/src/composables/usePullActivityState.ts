import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type PullActivityStateOptions = {
  isFeedRoute: ReadableRef<boolean>
  pagePullRefreshing: ReadableRef<boolean>
  pagePullOffset: ReadableRef<number>
  sourceReaderOpen: ReadableRef<boolean>
  getFeedPullActive: () => boolean
  getFeedPullOffset: () => number
  getFeedPullViewKey: () => string
}

function pullProgressFromOffset(offset: number) {
  return Math.min(offset / 76, 1)
}

export function usePullActivityState(options: PullActivityStateOptions) {
  const feedActive = computed(
    () => options.isFeedRoute.value && (options.getFeedPullActive() || options.getFeedPullOffset() > 1),
  )
  const pageActive = computed(
    () => !options.isFeedRoute.value && (options.pagePullRefreshing.value || options.pagePullOffset.value > 1),
  )
  const sourceActive = computed(
    () =>
      options.sourceReaderOpen.value &&
      options.getFeedPullViewKey().startsWith('source:') &&
      (options.getFeedPullActive() || options.getFeedPullOffset() > 1),
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
