import type { Ref } from 'vue'

import { usePageContentMotion } from '@/composables/usePageContentMotion'
import { usePagePullStatus } from '@/composables/usePagePullStatus'
import { usePullRefresh } from '@/composables/usePullRefresh'

type AppPagePullStateOptions = {
  pageTitle: Ref<string>
  threshold?: number
}

export function useAppPagePullState(options: AppPagePullStateOptions) {
  const pullRefresh = usePullRefresh({ threshold: options.threshold ?? 52 })
  const contentMotion = usePageContentMotion({ pullOffset: pullRefresh.offset })
  const status = usePagePullStatus({
    refreshing: pullRefresh.refreshing,
    progress: pullRefresh.distanceProgress,
    pageTitle: options.pageTitle,
  })

  return {
    pullRefresh,
    contentMotion,
    offset: pullRefresh.offset,
    settling: pullRefresh.settling,
    refreshing: pullRefresh.refreshing,
    progress: pullRefresh.distanceProgress,
    sideStretch: contentMotion.sideStretch,
    contentStyle: contentMotion.contentStyle,
    statusText: status.text,
    statusMeta: status.meta,
    settleOffset: pullRefresh.settleOffset,
    resetMotion: pullRefresh.resetMotion,
    clearTimers: pullRefresh.clearTimers,
    setSideOffset: contentMotion.setSideOffset,
    setSideStretch: contentMotion.setSideStretch,
    resetSideMotion: contentMotion.resetSideMotion,
    clearStretchAnchorIfIdle: contentMotion.clearStretchAnchorIfIdle,
  }
}

export type AppPagePullState = ReturnType<typeof useAppPagePullState>
