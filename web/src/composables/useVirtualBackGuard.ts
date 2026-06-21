import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

type VirtualBackState = {
  shouldGuard: boolean
  needsVirtualLayer: boolean
  needsHomeGuard: boolean
}

type VirtualBackGuardOptions = {
  route: RouteLocationNormalizedLoaded
  router: Router
  getRouteFullPath: () => string
  getState: () => VirtualBackState
  canHandleNavigation: () => boolean
  consumeBack: () => boolean
  onBackConsumed?: () => void
}

function currentHistoryStateHasVirtualGuard() {
  if (typeof window === 'undefined') {
    return false
  }
  const currentState = window.history.state || {}
  return Boolean(currentState.messagefeedVirtualLayer || currentState.messagefeedHomeGuard)
}

export function useVirtualBackGuard(options: VirtualBackGuardOptions) {
  let virtualBackHandledAt = 0
  let removeRouterGuard: (() => void) | null = null

  function syncHistoryState(forcePush = false) {
    if (typeof window === 'undefined' || !options.route.name) {
      return
    }

    const currentState = window.history.state || {}
    const state = options.getState()
    if (!state.shouldGuard) {
      if (currentState.messagefeedVirtualLayer || currentState.messagefeedHomeGuard) {
        const {
          messagefeedVirtualLayer: _messagefeedVirtualLayer,
          messagefeedHomeGuard: _messagefeedHomeGuard,
          ...restState
        } = currentState
        window.history.replaceState(restState, '', options.getRouteFullPath())
      }
      return
    }

    if (
      !forcePush &&
      ((state.needsVirtualLayer && currentState.messagefeedVirtualLayer) ||
        (state.needsHomeGuard && currentState.messagefeedHomeGuard))
    ) {
      return
    }

    window.history.pushState(
      {
        ...currentState,
        messagefeedVirtualLayer: state.needsVirtualLayer || undefined,
        messagefeedHomeGuard: state.needsHomeGuard || undefined,
      },
      '',
      options.getRouteFullPath(),
    )
  }

  function consumeSystemBack() {
    const handled = options.consumeBack()
    if (!handled) {
      return false
    }

    virtualBackHandledAt = Date.now()
    options.onBackConsumed?.()
    return true
  }

  function handlePopState() {
    if (!options.getState().shouldGuard) {
      return
    }

    if (Date.now() - virtualBackHandledAt < 80) {
      syncHistoryState()
      return
    }

    consumeSystemBack()
  }

  function installRouterGuard() {
    removeRouterGuard?.()
    removeRouterGuard = options.router.beforeEach(() => {
      if (
        !options.canHandleNavigation() ||
        !currentHistoryStateHasVirtualGuard() ||
        !options.getState().shouldGuard
      ) {
        return true
      }

      if (Date.now() - virtualBackHandledAt < 120) {
        return false
      }

      return consumeSystemBack() ? false : true
    })
    return removeRouterGuard
  }

  function uninstallRouterGuard() {
    removeRouterGuard?.()
    removeRouterGuard = null
  }

  return {
    syncHistoryState,
    handlePopState,
    installRouterGuard,
    uninstallRouterGuard,
  }
}
