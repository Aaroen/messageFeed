import { useReaderItemOpenAction } from '@/composables/useReaderItemOpenAction'
import { useReaderSourceOpenAction } from '@/composables/useReaderSourceOpenAction'
import { useReaderSourceRevealAction } from '@/composables/useReaderSourceRevealAction'

type SourceOpenOptions = Parameters<typeof useReaderSourceOpenAction>[0]
type SourceRevealOptions = Parameters<typeof useReaderSourceRevealAction>[0]
type ItemOpenOptions = Parameters<typeof useReaderItemOpenAction>[0]

type AppReaderOpenInteractionsOptions = {
  sourceOpen: SourceOpenOptions
  sourceReveal: Omit<SourceRevealOptions, 'openSourceReader'>
  itemOpen: Omit<ItemOpenOptions, 'openSourceReader'>
}

export function useAppReaderOpenInteractions(options: AppReaderOpenInteractionsOptions) {
  const sourceOpenAction = useReaderSourceOpenAction(options.sourceOpen)
  const sourceRevealAction = useReaderSourceRevealAction({
    ...options.sourceReveal,
    openSourceReader: sourceOpenAction.openSourceReader,
  })
  const itemOpenAction = useReaderItemOpenAction({
    ...options.itemOpen,
    openSourceReader: sourceOpenAction.openSourceReader,
  })

  return {
    openSourceReader: sourceOpenAction.openSourceReader,
    showSourceReaderUnderDetail: sourceRevealAction.showSourceReaderUnderDetail,
    openItemReader: itemOpenAction.openItemReader,
  }
}
