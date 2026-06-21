import { useReaderRouteSync } from '@/composables/useReaderRouteSync'

type ReaderRouteSync = ReturnType<typeof useReaderRouteSync>

type AppReaderRouteSyncBindingOptions = Parameters<typeof useReaderRouteSync>[0] & {
  bindReaderRouteSync: (readerRouteSync: ReaderRouteSync) => void
}

export function useAppReaderRouteSyncBinding(options: AppReaderRouteSyncBindingOptions) {
  const { bindReaderRouteSync, ...routeSyncOptions } = options
  const readerRouteSync = useReaderRouteSync(routeSyncOptions)
  bindReaderRouteSync(readerRouteSync)

  return readerRouteSync
}
