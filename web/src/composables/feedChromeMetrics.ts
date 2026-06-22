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
