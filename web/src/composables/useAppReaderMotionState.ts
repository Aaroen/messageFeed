import { useReaderDetailContentMotion } from '@/composables/useReaderDetailContentMotion'
import { useReaderDetailFrame } from '@/composables/useReaderDetailFrame'
import { useReaderDetailProgressMotion } from '@/composables/useReaderDetailProgressMotion'
import { useReaderDetailSurfaceMotion } from '@/composables/useReaderDetailSurfaceMotion'
import { useReaderDetailTextMotion } from '@/composables/useReaderDetailTextMotion'
import { useReaderDetailTransitionMotion } from '@/composables/useReaderDetailTransitionMotion'
import { useReaderLayoutState } from '@/composables/useReaderLayoutState'
import { useReaderSourceSurfaceMotion } from '@/composables/useReaderSourceSurfaceMotion'
import { useReaderSourceTitleMotion } from '@/composables/useReaderSourceTitleMotion'
import { useSourceContentMotion } from '@/composables/useSourceContentMotion'

type AppReaderMotionStateOptions = {
  layout: Parameters<typeof useReaderLayoutState>[0]
  sourceSurface: Omit<Parameters<typeof useReaderSourceSurfaceMotion>[0], 'headerSpace'>
  sourceContent: Omit<Parameters<typeof useSourceContentMotion>[0], 'headerSpace'>
  detailSurface: Parameters<typeof useReaderDetailSurfaceMotion>[0]
  detailContent: Omit<Parameters<typeof useReaderDetailContentMotion>[0], 'frameMinHeight'>
  detailProgress: Omit<Parameters<typeof useReaderDetailProgressMotion>[0], 'surfaceMargin' | 'expandedTop'>
  detailText: Parameters<typeof useReaderDetailTextMotion>[0]
  sourceTitle: Parameters<typeof useReaderSourceTitleMotion>[0]
  detailTransition: Omit<
    Parameters<typeof useReaderDetailTransitionMotion>[0],
    'fallbackTargetRect' | 'surfaceMargin' | 'expandedTop'
  >
  detailFrame: Parameters<typeof useReaderDetailFrame>[0]
}

export function useAppReaderMotionState(options: AppReaderMotionStateOptions) {
  const layout = useReaderLayoutState(options.layout)
  const sourceSurfaceMotion = useReaderSourceSurfaceMotion({
    ...options.sourceSurface,
    headerSpace: layout.sourceHeaderSpace,
  })
  const sourceContentMotion = useSourceContentMotion({
    ...options.sourceContent,
    headerSpace: layout.sourceHeaderSpace,
  })
  const detailSurfaceMotion = useReaderDetailSurfaceMotion(options.detailSurface)
  const detailContentMotion = useReaderDetailContentMotion({
    ...options.detailContent,
    frameMinHeight: layout.detailFrameMinHeight,
  })
  const detailProgressMotion = useReaderDetailProgressMotion({
    ...options.detailProgress,
    surfaceMargin: layout.detailSurfaceMargin,
    expandedTop: layout.detailExpandedTop,
  })
  const detailTextMotion = useReaderDetailTextMotion(options.detailText)
  const sourceTitleMotion = useReaderSourceTitleMotion(options.sourceTitle)
  const detailTransitionMotion = useReaderDetailTransitionMotion({
    ...options.detailTransition,
    fallbackTargetRect: layout.detailSourceFallbackTargetRect,
    surfaceMargin: layout.detailSurfaceMargin,
    expandedTop: layout.detailExpandedTop,
  })
  const detailFrame = useReaderDetailFrame(options.detailFrame)

  return {
    sourceReaderStyle: sourceSurfaceMotion.surfaceStyle,
    sourceContentStyle: sourceContentMotion.contentStyle,
    settleSourceContentAfterRefresh: sourceContentMotion.settleAfterRefresh,
    clearSourceContentTimer: sourceContentMotion.clearTimer,
    detailReaderStyle: detailSurfaceMotion.readerStyle,
    detailContentStyle: detailContentMotion.contentStyle,
    detailInlineMetaStyle: detailContentMotion.inlineMetaStyle,
    detailFrameStyle: detailContentMotion.frameStyle,
    detailActionsStyle: detailContentMotion.actionsStyle,
    detailProgressStyle: detailProgressMotion.railStyle,
    detailProgressFillStyle: detailProgressMotion.fillStyle,
    detailProgressThumbStyle: detailProgressMotion.thumbStyle,
    detailMorphTextStyle: detailTextMotion.morphTextStyle,
    detailMorphMetaStyle: detailTextMotion.morphMetaStyle,
    detailMorphTitleStyle: detailTextMotion.morphTitleStyle,
    detailMorphSummaryStyle: detailTextMotion.morphSummaryStyle,
    detailHeaderTitleStyle: detailTextMotion.headerTitleStyle,
    detailHeaderCurrentTextStyle: detailTextMotion.headerCurrentTextStyle,
    detailHeaderPreviousTextStyle: detailTextMotion.headerPreviousTextStyle,
    detailInlineSourceStyle: detailTextMotion.inlineSourceStyle,
    detailMorphSourceLabelStyle: detailTextMotion.morphSourceLabelStyle,
    sourceTitleRevealVisible: sourceTitleMotion.revealVisible,
    sourceNameMorphStyle: sourceTitleMotion.nameMorphStyle,
    sourceTitleLayerStyle: sourceTitleMotion.titleLayerStyle,
    sourceTitleTextStyle: sourceTitleMotion.titleTextStyle,
    sourceTitleRevealStyle: sourceTitleMotion.revealStyle,
    detailTransitionSurfaceStyle: detailTransitionMotion.surfaceStyle,
    detailPreviewSummary: detailFrame.previewSummary,
    detailDisplayDate: detailFrame.displayDate,
    detailFrameId: detailFrame.frameId,
    detailSrcdoc: detailFrame.srcdoc,
  }
}
