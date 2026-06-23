import { useAppGestureResetAction } from '@/composables/useAppGestureResetAction'
import { useAppNavigationActions } from '@/composables/useAppNavigationActions'
import { useAppNavigationConfig } from '@/composables/useAppNavigationConfig'

type NavigationActionOptions = Parameters<typeof useAppNavigationActions>[0]

type AppNavigationRuntimeOptions = {
  router: NavigationActionOptions['router']
  routeRuntime: NavigationActionOptions['routeRuntime']
  navigationDrawer: NavigationActionOptions['navigationDrawer']
  feedPagerTransition: NavigationActionOptions['feedPagerTransition']
  clearGesturePointer: () => void
  resetNavigationGesture: () => void
  resetFeedViewSwipeTracking: () => void
  resetFeedViewPointerTracking: () => void
  clearFeedViewStartedWithHiddenChrome: () => void
  resetReaderBackSwipeCandidate: () => void
  setChromeStableVisible: () => void
  motionDelay: NavigationActionOptions['motionDelay']
  motionNormalDuration: NavigationActionOptions['motionNormalDuration']
}

export function useAppNavigationRuntime(options: AppNavigationRuntimeOptions) {
  const { resetGestureTracking } = useAppGestureResetAction({
    clearGesturePointer: options.clearGesturePointer,
    resetNavigationGesture: options.resetNavigationGesture,
    resetFeedViewSwipeTracking: options.resetFeedViewSwipeTracking,
    resetFeedViewPointerTracking: options.resetFeedViewPointerTracking,
    clearFeedViewStartedWithHiddenChrome: options.clearFeedViewStartedWithHiddenChrome,
    resetReaderBackSwipeCandidate: options.resetReaderBackSwipeCandidate,
  })

  const { managementItems, feedTabs } = useAppNavigationConfig()
  const navigationActions = useAppNavigationActions({
    router: options.router,
    routeRuntime: options.routeRuntime,
    navigationDrawer: options.navigationDrawer,
    feedPagerTransition: options.feedPagerTransition,
    managementItems,
    resetGestureTracking,
    setChromeStableVisible: options.setChromeStableVisible,
    motionDelay: options.motionDelay,
    motionNormalDuration: options.motionNormalDuration,
  })

  return {
    managementItems,
    feedTabs,
    resetGestureTracking,
    ...navigationActions,
  }
}
