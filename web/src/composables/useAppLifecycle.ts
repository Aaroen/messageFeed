import { onMounted, onUnmounted } from 'vue'

type AppLifecycleOptions = {
  loadReaderSettings: () => void
  loadTheme: () => void
  installVirtualBackGuard: () => void
  uninstallVirtualBackGuard: () => void
  waitForRouterReady: () => Promise<unknown>
  restoreReaderSession: () => Promise<void> | void
  markReaderSessionReady: () => void
  scheduleReaderURLAndHistorySync: () => void
  installWindowEventListeners: () => void
  uninstallWindowEventListeners: () => void
  saveReaderSessionNow: () => void
  clearRuntimeTimers: Array<() => void>
}

export function useAppLifecycle(options: AppLifecycleOptions) {
  onMounted(() => {
    options.loadReaderSettings()
    options.loadTheme()
    options.installVirtualBackGuard()
    void options
      .waitForRouterReady()
      .then(() => options.restoreReaderSession())
      .finally(() => {
        options.markReaderSessionReady()
        options.scheduleReaderURLAndHistorySync()
      })
    options.installWindowEventListeners()
  })

  onUnmounted(() => {
    options.saveReaderSessionNow()
    options.uninstallVirtualBackGuard()
    options.uninstallWindowEventListeners()
    for (const clearTimer of options.clearRuntimeTimers) {
      clearTimer()
    }
  })
}
