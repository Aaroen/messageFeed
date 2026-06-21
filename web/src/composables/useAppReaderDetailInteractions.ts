import { useReaderDetailMessageHandler } from '@/composables/useReaderDetailMessageHandler'
import { useReaderDetailProgressHandlers } from '@/composables/useReaderDetailProgressHandlers'
import { useReaderSettingsSync } from '@/composables/useReaderSettingsSync'

type AppReaderDetailInteractionsOptions = {
  progress: Parameters<typeof useReaderDetailProgressHandlers>[0]
  message: Omit<Parameters<typeof useReaderDetailMessageHandler>[0], 'syncDetailContainerMetrics'>
  settings: Parameters<typeof useReaderSettingsSync>[0]
}

export function useAppReaderDetailInteractions(options: AppReaderDetailInteractionsOptions) {
  const progressHandlers = useReaderDetailProgressHandlers(options.progress)
  const messageHandler = useReaderDetailMessageHandler({
    ...options.message,
    syncDetailContainerMetrics: progressHandlers.syncDetailContainerMetrics,
  })
  const settingsSync = useReaderSettingsSync(options.settings)

  return {
    syncDetailContainerMetrics: progressHandlers.syncDetailContainerMetrics,
    handleDetailProgressChange: progressHandlers.handleDetailProgressChange,
    handleDetailProgressDragStart: progressHandlers.handleDetailProgressDragStart,
    handleDetailProgressDragEnd: progressHandlers.handleDetailProgressDragEnd,
    handleDetailFrameLoad: progressHandlers.handleDetailFrameLoad,
    handleMessage: messageHandler.handleMessage,
    clearReaderDetailFrames: () => {
      progressHandlers.clearFrame()
      messageHandler.clearMetricsFrame()
    },
    loadReaderSettings: settingsSync.loadReaderSettings,
    handleReaderSettingsChanged: settingsSync.handleReaderSettingsChanged,
  }
}
