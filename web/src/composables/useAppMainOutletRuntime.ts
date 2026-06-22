import { useAppMainOutletBindings } from '@/composables/useAppMainOutletBindings'
import { useAppTopChromeOutletState } from '@/composables/useAppTopChromeOutletState'

type AppTopChromeOutletOptions = Parameters<typeof useAppTopChromeOutletState>[0]
type AppMainOutletBindingOptions = Parameters<typeof useAppMainOutletBindings>[0]

type AppMainOutletRuntimeOptions = {
  topChrome: AppTopChromeOutletOptions
  mainOutlet: Omit<AppMainOutletBindingOptions, 'topChrome'>
}

export function useAppMainOutletRuntime(options: AppMainOutletRuntimeOptions) {
  const topChrome = useAppTopChromeOutletState(options.topChrome)
  const mainOutlet = useAppMainOutletBindings({
    ...options.mainOutlet,
    topChrome,
  })

  return {
    topChrome,
    props: mainOutlet.props,
    listeners: mainOutlet.listeners,
  }
}
