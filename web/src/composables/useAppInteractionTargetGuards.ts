const pageTopPullControlSelector = 'button, a, input, textarea, select'

export function useAppInteractionTargetGuards() {
  function isPageTopPullControlTarget(target: EventTarget | null) {
    return target instanceof Element && Boolean(target.closest(pageTopPullControlSelector))
  }

  return {
    isPageTopPullControlTarget,
  }
}
