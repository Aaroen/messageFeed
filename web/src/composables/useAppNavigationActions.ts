import type { Router } from 'vue-router'

type ReadableRef<T> = {
  readonly value: T
}

type ManagementItem = {
  key: string
  path: string
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
    settleProgrammaticNavigation: (delay: number) => void
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

  function goHome(closePanel = options.navigationDrawer.visible.value) {
    void pushRoute('/recommendations')
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
    options.feedPagerTransition.beginProgrammaticNavigation()
    void pushRoute(path)
    options.feedPagerTransition.settleProgrammaticNavigation(options.motionDelay(options.motionNormalDuration))
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
