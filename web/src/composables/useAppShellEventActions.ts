type AppShellEventActionsOptions = {
  consumeClick: (event: MouseEvent) => void
  suppressNextClick: () => void
  closeNavigation: () => void
  syncViewportSize: () => void
}

export function useAppShellEventActions(options: AppShellEventActionsOptions) {
  function handleClickCapture(event: MouseEvent) {
    options.consumeClick(event)
  }

  function suppressFollowingClick() {
    options.suppressNextClick()
  }

  function handleKeydown(event: KeyboardEvent) {
    if (event.key === 'Escape') {
      options.closeNavigation()
    }
  }

  function handleResize() {
    options.syncViewportSize()
  }

  return {
    handleClickCapture,
    suppressFollowingClick,
    handleKeydown,
    handleResize,
  }
}
