import { clampProgress } from '@/composables/feedChromeMetrics'

type ChromeLayerStyleOptions = {
  shift?: number
  scaleStart?: number
  disableTransition?: boolean
  pointerEnabled?: boolean
}

type ChromeLayerMotionOptions = {
  isSettling?: () => boolean
}

type HeaderStylePayload = {
  progress: number
  headerHeight: number
  refreshing?: boolean
  elevated?: boolean
  inactive?: boolean
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

function cssRotate(degrees: number) {
  return `rotate(${cssNumber(degrees)}deg)`
}

export function useChromeLayerMotion(options: ChromeLayerMotionOptions = {}) {
  function headerStyle(payload: HeaderStylePayload) {
    const safeProgress = clampProgress(payload.progress)
    return {
      background: payload.refreshing ? 'var(--mf-header-refresh-bg)' : undefined,
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 ? ('auto' as const) : ('none' as const),
      transform: cssTranslate3d(0, (safeProgress - 1) * payload.headerHeight),
      transition: options.isSettling?.()
        ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) var(--ease-standard)'
        : undefined,
      visibility: payload.inactive ? ('hidden' as const) : undefined,
      zIndex: payload.elevated ? 130 : undefined,
    }
  }

  function sourceHeaderStyle(progress: number, headerHeight: number, settling: boolean) {
    const safeProgress = clampProgress(progress)
    return {
      opacity: safeProgress.toFixed(3),
      pointerEvents: safeProgress > 0.86 ? ('auto' as const) : ('none' as const),
      transform: cssTranslate3d(0, (safeProgress - 1) * headerHeight),
      transition: settling
        ? 'transform var(--motion-chrome) var(--ease-emphasized), opacity var(--motion-chrome) var(--ease-standard)'
        : undefined,
    }
  }

  function refreshStatusStyle(visible: boolean, progress: number) {
    return layerStyle(visible, progress, { shift: -10, scaleStart: 0.96 })
  }

  function refreshIconStyle(refreshing: boolean, progress: number) {
    const safeProgress = clampProgress(progress)
    return {
      transform: refreshing ? 'none' : cssRotate(safeProgress * 300),
    }
  }

  function feedTabsStyle(payload: {
    detailReaderOpen: boolean
    returnProgress: number
    readerBackDragging: boolean
    detailBlocksGestures: boolean
    feedPullActive: boolean
    headerProgress: number
  }) {
    if (payload.detailReaderOpen) {
      return layerStyle(payload.returnProgress > 0.001, payload.returnProgress, {
        shift: 7,
        scaleStart: 0.98,
        disableTransition: payload.readerBackDragging,
        pointerEnabled: !payload.detailBlocksGestures,
      })
    }

    return layerStyle(!payload.feedPullActive, payload.headerProgress, {
      shift: 0,
      scaleStart: 1,
    })
  }

  function feedTabsTargetStyle(payload: {
    visible: boolean
    feedPullActive: boolean
    headerProgress: number
    targetProgress: number
    dragging: boolean
  }) {
    return layerStyle(
      payload.visible && !payload.feedPullActive,
      payload.headerProgress * payload.targetProgress,
      {
        shift: 0,
        scaleStart: 1,
        disableTransition: payload.dragging,
        pointerEnabled: false,
      },
    )
  }

  function navOpenButtonStyle(progress: number, headerHeight: number, visible: boolean) {
    const safeProgress = clampProgress(visible ? progress : 0)
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
    refreshStatusStyle,
    refreshIconStyle,
    feedTabsStyle,
    feedTabsTargetStyle,
    layerStyle,
  }
}
