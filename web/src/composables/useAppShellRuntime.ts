import { useAppShellEventActions } from '@/composables/useAppShellEventActions'
import { useClickSuppression } from '@/composables/useClickSuppression'

type AppShellRuntimeOptions = {
  clickSuppressionDuration: number
  closeNavigation: () => void
  syncViewportSize: () => void
}

export function useAppShellRuntime(options: AppShellRuntimeOptions) {
  const clickSuppression = useClickSuppression(options.clickSuppressionDuration)
  const shellEventActions = useAppShellEventActions({
    consumeClick: (event) => clickSuppression.consume(event),
    suppressNextClick: () => clickSuppression.suppressNext(),
    closeNavigation: options.closeNavigation,
    syncViewportSize: options.syncViewportSize,
  })

  return {
    ...shellEventActions,
    clearClickSuppressionTimer: clickSuppression.clearTimer,
  }
}
