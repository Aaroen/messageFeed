import { storeToRefs } from 'pinia'
import { computed } from 'vue'

import { useTopPullState } from '@/composables/useTopPullState'
import { useFeedInteractionStore } from '@/stores/feedInteraction'

export function useAppFeedPullInteractionRuntime() {
  const feedInteraction = useFeedInteractionStore()
  const topPull = useTopPullState()
  const topPulling = topPull.pulling
  const topPullStartedWithChrome = topPull.startedWithChrome
  const {
    pullViewKey,
    pullActive,
    pullOffset,
    pullRefreshing,
    statusText: pullStatusText,
    statusMeta: pullStatusMeta,
  } = storeToRefs(feedInteraction)
  const feedPullBusy = computed(
    () => pullActive.value || pullRefreshing.value || pullOffset.value > 1 || topPulling.value,
  )

  return {
    topPull,
    topPulling,
    topPullStartedWithChrome,
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
    finishTopPull: topPull.finish,
  }
}
