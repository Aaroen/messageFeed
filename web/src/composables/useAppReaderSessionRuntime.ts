import { useAppReaderRouteSyncAction } from '@/composables/useAppReaderRouteSyncAction'
import { useAppReaderScrollMemoryActions } from '@/composables/useAppReaderScrollMemoryActions'
import { useAppReaderSession } from '@/composables/useAppReaderSession'
import { useAppReaderSessionPersistence } from '@/composables/useAppReaderSessionPersistence'
import { useAppReaderSessionSnapshots } from '@/composables/useAppReaderSessionSnapshots'

type ReaderSessionSnapshotOptions = Parameters<typeof useAppReaderSessionSnapshots>[0]
type ReaderSessionPersistenceOptions = Parameters<typeof useAppReaderSessionPersistence>[0]
type ReaderScrollMemoryOptions = Parameters<typeof useAppReaderScrollMemoryActions>[0]

type AppReaderSessionRuntimeOptions = Omit<
  ReaderSessionSnapshotOptions,
  'rememberSourceScrollTop' | 'rememberDetailScrollTop'
> & {
  rememberScrollTop: ReaderScrollMemoryOptions['rememberScrollTop']
  canSaveReaderSession: ReaderSessionPersistenceOptions['canSaveReaderSession']
}

export function useAppReaderSessionRuntime(options: AppReaderSessionRuntimeOptions) {
  const routeSyncAction = useAppReaderRouteSyncAction()
  const scrollMemoryActions = useAppReaderScrollMemoryActions({
    rememberScrollTop: options.rememberScrollTop,
  })
  const sessionSnapshots = useAppReaderSessionSnapshots({
    ...options,
    rememberSourceScrollTop: scrollMemoryActions.rememberSourceScrollTop,
    rememberDetailScrollTop: scrollMemoryActions.rememberDetailScrollTop,
  })
  const readerSession = useAppReaderSession({
    createSnapshot: sessionSnapshots.readerSessionSnapshot,
    getCurrentRouteFullPath: options.getRouteFullPath,
    restoreSnapshot: sessionSnapshots.applyReaderSessionSnapshot,
    afterRestore: routeSyncAction.scheduleReaderURLAndHistorySync,
  })
  const sessionPersistence = useAppReaderSessionPersistence({
    restoring: readerSession.restoring,
    canSaveReaderSession: options.canSaveReaderSession,
    saveNow: readerSession.saveNow,
    scheduleSave: readerSession.scheduleSave,
    restore: readerSession.restore,
  })

  return {
    bindReaderRouteSync: routeSyncAction.bindReaderRouteSync,
    scheduleReaderURLAndHistorySync: routeSyncAction.scheduleReaderURLAndHistorySync,
    rememberSourceScrollTop: scrollMemoryActions.rememberSourceScrollTop,
    rememberDetailScrollTop: scrollMemoryActions.rememberDetailScrollTop,
    clearReaderSessionScrollRestoreTimers: sessionSnapshots.clearScrollRestoreTimers,
    readerSession,
    ...sessionPersistence,
  }
}
