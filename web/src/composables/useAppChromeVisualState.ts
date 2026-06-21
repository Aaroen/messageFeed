import { useAppChromeLayerState } from '@/composables/useAppChromeLayerState'
import { useAppMainClassState } from '@/composables/useAppMainClassState'

type AppChromeVisualStateOptions = {
  layer: Parameters<typeof useAppChromeLayerState>[0]
  mainClass: Parameters<typeof useAppMainClassState>[0]
}

export function useAppChromeVisualState(options: AppChromeVisualStateOptions) {
  const layerState = useAppChromeLayerState(options.layer)
  const mainClassState = useAppMainClassState(options.mainClass)

  return {
    pullStatusStyle: layerState.pullStatusStyle,
    pullIconStyle: layerState.pullIconStyle,
    pagePullStatusStyle: layerState.pagePullStatusStyle,
    pagePullIconStyle: layerState.pagePullIconStyle,
    feedTabsLayerStyle: layerState.feedTabsLayerStyle,
    feedTabsTargetLayerStyle: layerState.feedTabsTargetLayerStyle,
    sourcePullStatusStyle: layerState.sourcePullStatusStyle,
    sourcePullIconStyle: layerState.sourcePullIconStyle,
    sourceHeaderStyle: layerState.sourceHeaderStyle,
    detailHeaderLayerStyle: layerState.detailHeaderLayerStyle,
    pageTitleLayerStyle: layerState.pageTitleLayerStyle,
    sourceMainLayerStyle: layerState.sourceMainLayerStyle,
    headerClass: layerState.headerClass,
    headerStyle: layerState.headerStyle,
    navOpenButtonStyle: layerState.navOpenButtonStyle,
    mainClass: mainClassState.mainClass,
  }
}
