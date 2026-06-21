import { browserRouteFullPath, readerRouteMatchesCurrent } from '@/composables/useReaderRouteSync'
import { type ReaderSessionSnapshot, useReaderSession } from '@/composables/useReaderSession'

type AppReaderSessionOptions = {
  createSnapshot: () => ReaderSessionSnapshot
  getCurrentRouteFullPath: () => string
  restoreSnapshot: (snapshot: ReaderSessionSnapshot) => Promise<void> | void
  afterRestore: () => void
}

export function useAppReaderSession(options: AppReaderSessionOptions) {
  return useReaderSession<ReaderSessionSnapshot>({
    storageKey: 'messagefeed-reader-session-v1',
    maxAgeMS: 24 * 60 * 60 * 1000,
    saveDelayMS: 80,
    createSnapshot: options.createSnapshot,
    getCurrentRouteFullPath: options.getCurrentRouteFullPath,
    matchesCurrentRoute: (snapshotRouteFullPath) =>
      readerRouteMatchesCurrent(
        [options.getCurrentRouteFullPath(), browserRouteFullPath()],
        snapshotRouteFullPath,
      ),
    restoreSnapshot: options.restoreSnapshot,
    afterRestore: options.afterRestore,
  })
}
