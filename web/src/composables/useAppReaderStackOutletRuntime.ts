import { useAppReaderDetailInteractions } from '@/composables/useAppReaderDetailInteractions'
import { useAppReaderStackOutletBindings } from '@/composables/useAppReaderStackOutletBindings'

type ReaderDetailOptions = Parameters<typeof useAppReaderDetailInteractions>[0]
type ReaderStackOutletOptions = Parameters<typeof useAppReaderStackOutletBindings>[0]

type DetailOutletHandlers =
  | 'handleDetailFrameLoad'
  | 'handleDetailProgressDragStart'
  | 'handleDetailProgressDragEnd'
  | 'handleDetailProgressChange'

type AppReaderStackOutletRuntimeOptions = {
  detail: ReaderDetailOptions
  outlet: Omit<ReaderStackOutletOptions, DetailOutletHandlers>
}

export function useAppReaderStackOutletRuntime(options: AppReaderStackOutletRuntimeOptions) {
  const detailInteractions = useAppReaderDetailInteractions(options.detail)
  const outletBindings = useAppReaderStackOutletBindings({
    ...options.outlet,
    handleDetailFrameLoad: detailInteractions.handleDetailFrameLoad,
    handleDetailProgressDragStart: detailInteractions.handleDetailProgressDragStart,
    handleDetailProgressDragEnd: detailInteractions.handleDetailProgressDragEnd,
    handleDetailProgressChange: detailInteractions.handleDetailProgressChange,
  })

  return {
    props: outletBindings.props,
    listeners: outletBindings.listeners,
    syncDetailContainerMetrics: detailInteractions.syncDetailContainerMetrics,
    handleMessage: detailInteractions.handleMessage,
    clearReaderDetailFrames: detailInteractions.clearReaderDetailFrames,
    loadReaderSettings: detailInteractions.loadReaderSettings,
    handleReaderSettingsChanged: detailInteractions.handleReaderSettingsChanged,
  }
}
