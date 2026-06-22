import { storeToRefs } from 'pinia'
import { computed } from 'vue'

import { useFeedInteractionStore } from '@/stores/feedInteraction'

type ReadableRef<T> = {
  readonly value: T
}

type AppFeedPullInteractionRuntimeOptions = {
  feedTopPulling: ReadableRef<boolean>
}

export function useAppFeedPullInteractionRuntime(options: AppFeedPullInteractionRuntimeOptions) {
  const feedInteraction = useFeedInteractionStore()
  const {
    pullViewKey,
    pullActive,
    pullOffset,
    pullRefreshing,
    statusText: pullStatusText,
    statusMeta: pullStatusMeta,
  } = storeToRefs(feedInteraction)
  const feedPullBusy = computed(
    () => pullActive.value || pullRefreshing.value || pullOffset.value > 1 || options.feedTopPulling.value,
  )

  return {
    pullViewKey,
    pullActive,
    pullOffset,
    pullRefreshing,
    pullStatusText,
    pullStatusMeta,
    feedPullBusy,
    getPullViewKey: () => pullViewKey.value,
    getPullActive: () => pullActive.value,
    getPullOffset: () => pullOffset.value,
    getPullRefreshing: () => pullRefreshing.value,
  }
}
