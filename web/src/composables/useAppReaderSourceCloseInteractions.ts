import { useReaderParkedDetailRestoreAction } from '@/composables/useReaderParkedDetailRestoreAction'
import { useReaderSourceCloseAction } from '@/composables/useReaderSourceCloseAction'

type ParkedDetailRestoreOptions = Parameters<typeof useReaderParkedDetailRestoreAction>[0]
type SourceCloseOptions = Parameters<typeof useReaderSourceCloseAction>[0]

type AppReaderSourceCloseInteractionsOptions = {
  parkedRestore: Omit<ParkedDetailRestoreOptions, 'closeSourceReader'>
  sourceClose: Omit<SourceCloseOptions, 'restoreDetailFromParkedSource'>
}

export function useAppReaderSourceCloseInteractions(options: AppReaderSourceCloseInteractionsOptions) {
  let closeSourceReader: () => void = () => undefined
  const parkedRestoreAction = useReaderParkedDetailRestoreAction({
    ...options.parkedRestore,
    closeSourceReader: () => closeSourceReader(),
  })
  const sourceCloseAction = useReaderSourceCloseAction({
    ...options.sourceClose,
    restoreDetailFromParkedSource: parkedRestoreAction.restoreDetailFromParkedSource,
  })
  closeSourceReader = sourceCloseAction.closeSourceReader

  return {
    restoreDetailFromParkedSource: parkedRestoreAction.restoreDetailFromParkedSource,
    restoreSourceReaderBackTarget: sourceCloseAction.restoreSourceReaderBackTarget,
    closeSourceReader: sourceCloseAction.closeSourceReader,
  }
}
