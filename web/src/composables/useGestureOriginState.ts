export function useGestureOriginState() {
  let startX = 0
  let startY = 0
  let startNavigationProgress = 0
  let activePointerId: number | null = null

  function begin(clientX: number, clientY: number, navigationProgress = 0) {
    startX = clientX
    startY = clientY
    startNavigationProgress = navigationProgress
  }

  function delta(clientX: number, clientY: number) {
    return {
      deltaX: clientX - startX,
      deltaY: clientY - startY,
    }
  }

  function originX() {
    return startX
  }

  function navigationProgress() {
    return startNavigationProgress
  }

  function setActivePointerId(pointerId: number | null) {
    activePointerId = pointerId
  }

  function isActivePointer(pointerId: number) {
    return activePointerId === pointerId
  }

  function clearActivePointer() {
    activePointerId = null
  }

  return {
    begin,
    delta,
    originX,
    navigationProgress,
    setActivePointerId,
    isActivePointer,
    clearActivePointer,
  }
}
