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

function finiteNumber(value: number) {
  return Number.isFinite(value) ? value : 0
}

export function useChromeLayerMotion(options: ChromeLayerMotionOptions = {}) {
  function headerStyle(progress: number, headerHeight: number) {
    const safeProgress = finiteNumber(progress)
    return {
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 ? ('auto' as const) : ('none' as const),
      transform: cssTranslate3d(0, (safeProgress - 1) * headerHeight),
    }
  }

  function sourceHeaderStyle(progress: number, headerHeight: number, settling: boolean) {
    const safeProgress = finiteNumber(progress)
    return {
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 ? ('auto' as const) : ('none' as const),
      transform: cssTranslate3d(0, (safeProgress - 1) * headerHeight),
      transition: settling
        ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) var(--ease-standard)'
        : undefined,
    }
  }

  function navOpenButtonStyle(progress: number, headerHeight: number, visible: boolean) {
    const safeProgress = finiteNumber(visible ? progress : 0)
    return {
      top: cssPx((headerHeight - 44) / 2),
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 && visible ? ('auto' as const) : ('none' as const),
      transform: `${cssTranslate3d(0, (safeProgress - 1) * headerHeight)} scale(${(
        0.92 +
        safeProgress * 0.08
      ).toFixed(3)})`,
      transition: options.isSettling?.()
        ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) var(--ease-standard), visibility var(--motion-chrome) var(--ease-standard), border-color var(--motion-fast) var(--ease-standard), background var(--motion-fast) var(--ease-standard)'
        : undefined,
      visibility: safeProgress > 0.01 && visible ? ('visible' as const) : ('hidden' as const),
    }
  }

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
    headerStyle,
    sourceHeaderStyle,
    navOpenButtonStyle,
    layerStyle,
  }
}
