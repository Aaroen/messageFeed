type ReaderRouteSyncScheduler = {
  scheduleSync: (forcePush?: boolean) => void
}

export function useAppReaderRouteSyncAction() {
  let scheduler: ReaderRouteSyncScheduler | null = null
  let pendingForcePush: boolean | null = null

  function bindReaderRouteSync(nextScheduler: ReaderRouteSyncScheduler) {
    scheduler = nextScheduler
    if (pendingForcePush === null) {
      return
    }

    const forcePush = pendingForcePush
    pendingForcePush = null
    scheduler.scheduleSync(forcePush)
  }

  function scheduleReaderURLAndHistorySync(forcePush = false) {
    if (!scheduler) {
      pendingForcePush = Boolean(pendingForcePush || forcePush)
      return
    }

    scheduler.scheduleSync(forcePush)
  }

  return {
    bindReaderRouteSync,
    scheduleReaderURLAndHistorySync,
  }
}
