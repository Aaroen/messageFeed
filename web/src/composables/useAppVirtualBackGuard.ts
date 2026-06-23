import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

import { useAppVirtualBackActions } from '@/composables/useAppVirtualBackActions'
import { useVirtualBackGuard } from '@/composables/useVirtualBackGuard'

type AppVirtualBackActionsOptions = Parameters<typeof useAppVirtualBackActions>[0]

type AppVirtualBackGuardOptions = AppVirtualBackActionsOptions & {
  route: RouteLocationNormalizedLoaded
  router: Router
  getRouteFullPath: () => string
  canHandleNavigation: () => boolean
  onBackConsumed?: () => void
}

export function useAppVirtualBackGuard(options: AppVirtualBackGuardOptions) {
  const backActions = useAppVirtualBackActions(options)
  const guard = useVirtualBackGuard({
    route: options.route,
    router: options.router,
    getRouteFullPath: options.getRouteFullPath,
    getState: () => {
      if (options.route.meta.public) {
        return {
          shouldGuard: false,
          needsVirtualLayer: false,
          needsHomeGuard: false,
        }
      }
      const needsVirtualLayer = backActions.hasVirtualBackTarget()
      return {
        shouldGuard: needsVirtualLayer || options.isFeedRoute.value,
        needsVirtualLayer,
        needsHomeGuard: !needsVirtualLayer && options.isFeedRoute.value,
      }
    },
    canHandleNavigation: options.canHandleNavigation,
    consumeBack: backActions.runVirtualBackAnimation,
    onBackConsumed: options.onBackConsumed,
  })

  return {
    hasVirtualBackTarget: backActions.hasVirtualBackTarget,
    runVirtualBackAnimation: backActions.runVirtualBackAnimation,
    syncHistoryState: guard.syncHistoryState,
    handlePopState: guard.handlePopState,
    installRouterGuard: guard.installRouterGuard,
    uninstallRouterGuard: guard.uninstallRouterGuard,
  }
}
