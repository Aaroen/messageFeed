type ChromeLayerStyleOptions = {
  shift?: number
  scaleStart?: number
  disableTransition?: boolean
  pointerEnabled?: boolean
}

type ChromeLayerMotionOptions = {
  isSettling?: () => boolean
}

function clampProgress(value: number) {
  if (!Number.isFinite(value)) {
    return 0
  }
  return Math.min(Math.max(value, 0), 1)
}

function cssNumber(value: number, precision = 2) {
  return (Number.isFinite(value) ? value : 0).toFixed(precision)
}

function cssPx(value: number) {
  return `${cssNumber(value)}px`
}

function cssTranslate3d(x: number, y: number, z = 0) {
  return `translate3d(${cssPx(x)}, ${cssPx(y)}, ${cssPx(z)})`
}

export function useChromeLayerMotion(options: ChromeLayerMotionOptions = {}) {
  function layerStyle(
    visible: boolean,
    progress: number,
    styleOptions: ChromeLayerStyleOptions = {},
  ) {
    const safeProgress = clampProgress(visible ? progress : 0)
    const shift = styleOptions.shift ?? -8
    const scaleStart = styleOptions.scaleStart ?? 0.96
    const pointerEnabled = styleOptions.pointerEnabled ?? true
    return {
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 && pointerEnabled ? ('auto' as const) : ('none' as const),
      transform: `${cssTranslate3d(0, (1 - safeProgress) * shift)} scale(${(
        scaleStart +
        safeProgress * (1 - scaleStart)
      ).toFixed(3)})`,
      transition: styleOptions.disableTransition
        ? 'none'
        : options.isSettling?.()
          ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) var(--ease-standard), visibility var(--motion-chrome) var(--ease-standard)'
          : undefined,
      visibility: safeProgress > 0.01 ? ('visible' as const) : ('hidden' as const),
    }
  }

  return {
    layerStyle,
  }
}
