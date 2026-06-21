type AppWindowEventListenersOptions = {
  handleKeydown: (event: KeyboardEvent) => void
  handleResize: () => void
  handleMessage: (event: MessageEvent) => void
  handleReaderSettingsChanged: (event: Event) => void
  handlePopState: (event: PopStateEvent) => void
  saveReaderSessionNow: () => void
  handleWindowPointerDown: (event: PointerEvent) => void
  handleWindowPointerMove: (event: PointerEvent) => void
  handleWindowPointerUp: (event: PointerEvent) => void
  handleWindowPointerCancel: (event: PointerEvent) => void
  handleTouchStart: (event: TouchEvent) => void
  handleTouchMove: (event: TouchEvent) => void
  handleTouchEnd: (event: TouchEvent) => void
  handleTouchCancel: (event: TouchEvent) => void
}

export function useAppWindowEventListeners(options: AppWindowEventListenersOptions) {
  let installed = false

  const handleKeydown = (event: KeyboardEvent) => options.handleKeydown(event)
  const handleResize = () => options.handleResize()
  const handleMessage = (event: MessageEvent) => options.handleMessage(event)
  const handleReaderSettingsChanged = (event: Event) => options.handleReaderSettingsChanged(event)
  const handlePopState = (event: PopStateEvent) => options.handlePopState(event)
  const handleBeforeUnload = () => options.saveReaderSessionNow()
  const handleWindowPointerDown = (event: PointerEvent) => options.handleWindowPointerDown(event)
  const handleWindowPointerMove = (event: PointerEvent) => options.handleWindowPointerMove(event)
  const handleWindowPointerUp = (event: PointerEvent) => options.handleWindowPointerUp(event)
  const handleWindowPointerCancel = (event: PointerEvent) => options.handleWindowPointerCancel(event)
  const handleTouchStart = (event: TouchEvent) => options.handleTouchStart(event)
  const handleTouchMove = (event: TouchEvent) => options.handleTouchMove(event)
  const handleTouchEnd = (event: TouchEvent) => options.handleTouchEnd(event)
  const handleTouchCancel = (event: TouchEvent) => options.handleTouchCancel(event)

  function installWindowEventListeners() {
    if (installed) {
      return
    }

    window.addEventListener('keydown', handleKeydown)
    window.addEventListener('resize', handleResize)
    window.addEventListener('message', handleMessage)
    window.addEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
    window.addEventListener('popstate', handlePopState)
    window.addEventListener('beforeunload', handleBeforeUnload)
    window.addEventListener('pointerdown', handleWindowPointerDown, { passive: true })
    window.addEventListener('pointermove', handleWindowPointerMove, { passive: false })
    window.addEventListener('pointerup', handleWindowPointerUp, { passive: true })
    window.addEventListener('pointercancel', handleWindowPointerCancel, { passive: true })
    window.addEventListener('touchstart', handleTouchStart, { passive: true })
    window.addEventListener('touchmove', handleTouchMove, { passive: false })
    window.addEventListener('touchend', handleTouchEnd, { passive: true })
    window.addEventListener('touchcancel', handleTouchCancel, { passive: true })
    installed = true
  }

  function uninstallWindowEventListeners() {
    if (!installed) {
      return
    }

    window.removeEventListener('keydown', handleKeydown)
    window.removeEventListener('resize', handleResize)
    window.removeEventListener('message', handleMessage)
    window.removeEventListener('messagefeed-settings-changed', handleReaderSettingsChanged)
    window.removeEventListener('popstate', handlePopState)
    window.removeEventListener('beforeunload', handleBeforeUnload)
    window.removeEventListener('pointerdown', handleWindowPointerDown)
    window.removeEventListener('pointermove', handleWindowPointerMove)
    window.removeEventListener('pointerup', handleWindowPointerUp)
    window.removeEventListener('pointercancel', handleWindowPointerCancel)
    window.removeEventListener('touchstart', handleTouchStart)
    window.removeEventListener('touchmove', handleTouchMove)
    window.removeEventListener('touchend', handleTouchEnd)
    window.removeEventListener('touchcancel', handleTouchCancel)
    installed = false
  }

  return {
    installWindowEventListeners,
    uninstallWindowEventListeners,
  }
}
