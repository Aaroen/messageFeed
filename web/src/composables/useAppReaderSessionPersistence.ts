type ReadableRef<T> = {
  readonly value: T
}

type AppReaderSessionPersistenceOptions = {
  restoring: ReadableRef<boolean>
  canSaveReaderSession: (restoring: boolean) => boolean
  saveNow: () => void
  scheduleSave: () => void
  restore: () => Promise<void>
}

export function useAppReaderSessionPersistence(options: AppReaderSessionPersistenceOptions) {
  function saveReaderSessionNow() {
    if (!options.canSaveReaderSession(options.restoring.value)) {
      return
    }
    options.saveNow()
  }

  function scheduleReaderSessionSave() {
    if (!options.canSaveReaderSession(options.restoring.value)) {
      return
    }
    options.scheduleSave()
  }

  async function restoreReaderSession() {
    await options.restore()
  }

  return {
    saveReaderSessionNow,
    scheduleReaderSessionSave,
    restoreReaderSession,
  }
}
