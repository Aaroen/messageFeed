import { useAppLifecycle } from '@/composables/useAppLifecycle'
import { useAppWindowEventListeners } from '@/composables/useAppWindowEventListeners'

type AppLifecycleOptions = Parameters<typeof useAppLifecycle>[0]

type AppRuntimeEffectsOptions = {
  windowEvents: Parameters<typeof useAppWindowEventListeners>[0]
  lifecycle: Omit<AppLifecycleOptions, 'installWindowEventListeners' | 'uninstallWindowEventListeners'>
}

export function useAppRuntimeEffects(options: AppRuntimeEffectsOptions) {
  const windowEventListeners = useAppWindowEventListeners(options.windowEvents)

  useAppLifecycle({
    ...options.lifecycle,
    installWindowEventListeners: windowEventListeners.installWindowEventListeners,
    uninstallWindowEventListeners: windowEventListeners.uninstallWindowEventListeners,
  })
}
