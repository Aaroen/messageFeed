import { useAppRouteSessionWatchers } from '@/composables/useAppRouteSessionWatchers'

type RouteSessionWatcherOptions = Parameters<typeof useAppRouteSessionWatchers>[0]

export function useRouteRuntimeState() {
  let programmaticNavigation = false
  let readerSessionReady = false
  let programmaticNavigationTimer = 0
  let programmaticNavigationToken = 0

  function clearProgrammaticNavigationTimer() {
    if (typeof window !== 'undefined' && programmaticNavigationTimer !== 0) {
      window.clearTimeout(programmaticNavigationTimer)
    }
    programmaticNavigationTimer = 0
  }

  function setProgrammaticNavigation(active: boolean) {
    programmaticNavigationToken += 1
    clearProgrammaticNavigationTimer()
    programmaticNavigation = active
  }

  function finishProgrammaticNavigation() {
    programmaticNavigationToken += 1
    const token = programmaticNavigationToken
    clearProgrammaticNavigationTimer()
    programmaticNavigationTimer = window.setTimeout(() => {
      programmaticNavigationTimer = 0
      if (token !== programmaticNavigationToken) {
        return
      }
      programmaticNavigation = false
    }, 0)
  }

  function clearTimer() {
    programmaticNavigationToken += 1
    clearProgrammaticNavigationTimer()
    programmaticNavigation = false
  }

  async function runProgrammaticNavigation(action: () => Promise<unknown> | unknown) {
    setProgrammaticNavigation(true)
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

  function installRouteSessionWatchers(options: RouteSessionWatcherOptions) {
    useAppRouteSessionWatchers(options)
  }

  return {
    setProgrammaticNavigation,
    runProgrammaticNavigation,
    clearTimer,
    markReaderSessionReady,
    canHandleNavigation,
    canSyncReaderRoute,
    canSaveReaderSession,
    installRouteSessionWatchers,
  }
}
