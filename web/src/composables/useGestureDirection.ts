type GestureDirectionOptions = {
  viewDragThreshold: number
}

export function useGestureDirection(options: GestureDirectionOptions) {
  const directionLockRatio = 1.25
  const navigationDragRatio = 1.1
  const viewDirectionLockRatio = 1.35
  const topPullDirectionLockRatio = 1.18

  function isHorizontalSwipe(deltaX: number, deltaY: number) {
    return Math.abs(deltaX) > Math.abs(deltaY) * directionLockRatio
  }

  function isViewHorizontalSwipe(deltaX: number, deltaY: number) {
    return Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
  }

  function isNavigationDrag(deltaX: number, deltaY: number) {
    return Math.abs(deltaX) > 8 && Math.abs(deltaX) > Math.abs(deltaY) * navigationDragRatio
  }

  function isBackHorizontalSwipe(deltaX: number, deltaY: number) {
    return Math.abs(deltaX) > options.viewDragThreshold && Math.abs(deltaX) > Math.abs(deltaY) * viewDirectionLockRatio
  }

  function shouldCancelTopPull(deltaX: number, deltaY: number) {
    return deltaY <= 0 || Math.abs(deltaX) > Math.abs(deltaY) * topPullDirectionLockRatio
  }

  function shouldWaitForTopPull(deltaX: number, deltaY: number) {
    return deltaY < 2 || Math.abs(deltaY) <= Math.abs(deltaX) * topPullDirectionLockRatio
  }

  return {
    isHorizontalSwipe,
    isViewHorizontalSwipe,
    isNavigationDrag,
    isBackHorizontalSwipe,
    shouldCancelTopPull,
    shouldWaitForTopPull,
  }
}
