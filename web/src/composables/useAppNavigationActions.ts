import type { Router } from 'vue-router'

type ReadableRef<T> = {
  readonly value: T
}

type ManagementItem = {
  key: string
  path: string
}

type GoHomeOptions = {
  closePanel?: boolean
  replace?: boolean
}

type AppNavigationActionsOptions = {
  router: Router
  routeRuntime: {
    runProgrammaticNavigation: (action: () => Promise<unknown> | unknown) => Promise<void>
  }
  navigationDrawer: {
    visible: ReadableRef<boolean>
    settle: (open: boolean) => void
    openPanel: () => void
    closePanel: () => void
  }
  feedPagerTransition: {
    reset: () => void
    beginProgrammaticNavigation: () => void
    beginProgrammaticFeedSwitch: (path: string) => { animated: boolean }
    settleProgrammaticNavigation: (delay: number) => void
    commitProgrammaticNavigation: (
      delay: number,
      commit: () => Promise<unknown> | unknown,
    ) => void
  }
  managementItems: readonly ManagementItem[]
  resetGestureTracking: () => void
  setChromeStableVisible: () => void
  motionDelay: (duration: number) => number
  motionNormalDuration: number
}

export function useAppNavigationActions(options: AppNavigationActionsOptions) {
  async function pushRoute(path: string) {
    await options.routeRuntime.runProgrammaticNavigation(() => options.router.push(path))
  }

  async function replaceRoute(path: string) {
    await options.routeRuntime.runProgrammaticNavigation(() => options.router.replace(path))
  }

  function settleNavigation(open: boolean) {
    options.navigationDrawer.settle(open)
  }

  function openNavigation() {
    options.resetGestureTracking()
    options.navigationDrawer.openPanel()
  }

  function closeNavigation() {
    options.navigationDrawer.closePanel()
  }

  function handleMenuClick(key: string) {
    const item = options.managementItems.find((navItem) => navItem.key === key)
    if (item) {
      void pushRoute(item.path)
      closeNavigation()
    }
  }

  function goHome(actionOptions: GoHomeOptions = {}) {
    const closePanel = actionOptions.closePanel ?? options.navigationDrawer.visible.value
    const navigate = actionOptions.replace ? replaceRoute : pushRoute
    void navigate('/recommendations')
    options.setChromeStableVisible()
    options.feedPagerTransition.reset()
    if (closePanel) {
      closeNavigation()
    }
  }

  function handleCornerButtonClick() {
    openNavigation()
  }

  function navigateTo(path: string) {
    if (options.router.currentRoute.value.path === path) {
      return
    }

    const settleDelay = options.motionDelay(options.motionNormalDuration)
    const feedSwitch = options.feedPagerTransition.beginProgrammaticFeedSwitch(path)
    if (feedSwitch.animated) {
      options.feedPagerTransition.commitProgrammaticNavigation(settleDelay, () => pushRoute(path))
      return
    }

    options.feedPagerTransition.beginProgrammaticNavigation()
    void pushRoute(path)
    options.feedPagerTransition.settleProgrammaticNavigation(settleDelay)
  }

  return {
    pushRoute,
    replaceRoute,
    settleNavigation,
    openNavigation,
    closeNavigation,
    handleMenuClick,
    goHome,
    handleCornerButtonClick,
    navigateTo,
  }
}
