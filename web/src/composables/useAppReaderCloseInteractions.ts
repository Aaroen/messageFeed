import { useReaderBackSwipeResetAction } from '@/composables/useReaderBackSwipeResetAction'
import { useReaderItemCloseAction } from '@/composables/useReaderItemCloseAction'
import { useReaderRestoreActions } from '@/composables/useReaderRestoreActions'

type BackSwipeResetOptions = Parameters<typeof useReaderBackSwipeResetAction>[0]
type ItemCloseOptions = Parameters<typeof useReaderItemCloseAction>[0]
type RestoreActionsOptions = Parameters<typeof useReaderRestoreActions>[0]

type AppReaderCloseInteractionsOptions = {
  backSwipeReset: BackSwipeResetOptions
  itemClose: ItemCloseOptions
  restoreActions: Omit<RestoreActionsOptions, 'resetBackSwipeOffset'>
}

export function useAppReaderCloseInteractions(options: AppReaderCloseInteractionsOptions) {
  const backSwipeResetAction = useReaderBackSwipeResetAction(options.backSwipeReset)
  const restoreActions = useReaderRestoreActions({
    ...options.restoreActions,
    resetBackSwipeOffset: backSwipeResetAction.resetBackSwipeOffset,
  })
  const itemCloseAction = useReaderItemCloseAction(options.itemClose)

  return {
    resetBackSwipeOffset: backSwipeResetAction.resetBackSwipeOffset,
    restoreParkedSourceReader: restoreActions.restoreParkedSourceReader,
    restoreItemReaderExpansion: restoreActions.restoreItemReaderExpansion,
    restoreDetailFromSourceSwipe: restoreActions.restoreDetailFromSourceSwipe,
    completeDetailToSourceReader: restoreActions.completeDetailToSourceReader,
    finishCommittedListReturnForGesture: itemCloseAction.finishCommittedListReturnForGesture,
    closeItemReader: itemCloseAction.closeItemReader,
    collapseItemReader: itemCloseAction.collapseItemReader,
  }
}
