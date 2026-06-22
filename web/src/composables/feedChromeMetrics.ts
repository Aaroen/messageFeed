export function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

export function feedHeaderHeightForWidth(windowWidth: number) {
  return windowWidth <= 720 ? 78 : 86
}

export function feedContentTopOffset(headerHeight: number) {
  return headerHeight <= 78 ? 8 : 10
}

export function feedVisibleContentTopOffset(headerHeight: number) {
  return feedContentTopOffset(headerHeight) * 2
}

export function chromePhaseConsumesCollapsedLayout(phase: string) {
  return phase === 'hiding' || phase === 'refreshing'
}

export function chromePhaseNeedsVisibleTopClearance(phase: string) {
  return phase === 'visible' || phase === 'revealing' || phase === 'gesture-returning'
}

export function chromeNeedsVisibleTopClearance(phase: string, progress: number) {
  return chromePhaseNeedsVisibleTopClearance(phase) || (!chromePhaseConsumesCollapsedLayout(phase) && progress > 0.86)
}

export function sourceContentTopOffset() {
  return 14
}

export function topScrollInset(scrollTop: number, topOffset: number) {
  if (!Number.isFinite(scrollTop)) {
    return 0
  }
  return Math.min(Math.max(scrollTop, 0), Math.max(topOffset, 0))
}

export function feedTopScrollInset(scrollTop: number, headerHeight: number) {
  return topScrollInset(scrollTop, feedContentTopOffset(headerHeight))
}
