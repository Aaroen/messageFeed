export function useRouteRuntimeState() {
  let programmaticNavigation = false
  let readerSessionReady = false

  function setProgrammaticNavigation(active: boolean) {
    programmaticNavigation = active
  }

  function finishProgrammaticNavigation() {
    window.setTimeout(() => {
      programmaticNavigation = false
    }, 0)
  }

  async function runProgrammaticNavigation(action: () => Promise<unknown> | unknown) {
    programmaticNavigation = true
    try {
      await action()
    } finally {
      finishProgrammaticNavigation()
    }
  }

  function markReaderSessionReady() {
    readerSessionReady = true
  }

  function canHandleNavigation() {
    return readerSessionReady && !programmaticNavigation
  }

  function canSyncReaderRoute(restoring: boolean) {
    return readerSessionReady && !restoring
  }

  function canSaveReaderSession(restoring: boolean) {
    return readerSessionReady || restoring
  }

  return {
    setProgrammaticNavigation,
    runProgrammaticNavigation,
    markReaderSessionReady,
    canHandleNavigation,
    canSyncReaderRoute,
    canSaveReaderSession,
  }
}
