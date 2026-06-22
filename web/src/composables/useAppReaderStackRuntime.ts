import { watch } from 'vue'

import { useAppReaderSourceSubscription } from '@/composables/useAppReaderSourceSubscription'
import { useReaderStackState } from '@/composables/useReaderStackState'

export function useAppReaderStackRuntime() {
  const readerStackState = useReaderStackState()
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
