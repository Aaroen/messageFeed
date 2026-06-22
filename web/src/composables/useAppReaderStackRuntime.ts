import { watch } from 'vue'

import { useAppReaderSourceSubscription } from '@/composables/useAppReaderSourceSubscription'
import { useReaderStackState } from '@/composables/useReaderStackState'
import { useFeedListCacheStore } from '@/stores/feedListCache'

export function useAppReaderStackRuntime() {
  const readerStackState = useReaderStackState()
  const feedListCache = useFeedListCacheStore()
  function invalidateSubscriptionFeedCaches(sourceID: number) {
    feedListCache.invalidate('subscriptions:subscriptions:0')
    if (sourceID > 0) {
      feedListCache.invalidate(`source:subscriptions:${sourceID}`)
    }
  }

  const sourceSubscriptionControls = useAppReaderSourceSubscription({
    readerSource: readerStackState.readerSource,
    sourceCatalogEntry: readerStackState.sourceCatalogEntry,
    sourceSubscription: readerStackState.sourceSubscription,
    sourceSubscriptionLoading: readerStackState.sourceSubscriptionLoading,
    sourceNotice: readerStackState.sourceNotice,
    setSourceCatalogEntry: readerStackState.setSourceCatalogEntryState,
    setSourceSubscription: readerStackState.setSourceSubscriptionState,
    setSourceSubscriptionLoading: readerStackState.setSourceSubscriptionLoadingState,
    setSourceNotice: readerStackState.setSourceNoticeState,
    canShowNotice: () =>
      readerStackState.sourceReaderVisible.value && !readerStackState.sourceReaderUnderDetail.value,
    onSubscriptionContentChanged: invalidateSubscriptionFeedCaches,
  })

  watch(
    () => readerStackState.sourceReaderVisible.value && !readerStackState.sourceReaderUnderDetail.value,
    (sourceInteractive) => {
      if (!sourceInteractive) {
        readerStackState.setSourceNoticeState(null)
      }
    },
  )

  return {
    ...readerStackState,
    ...sourceSubscriptionControls,
  }
}
