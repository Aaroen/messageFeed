import { computed, ref, type Ref } from 'vue'

import { usePageContentMotion } from '@/composables/usePageContentMotion'
import { usePagePullStatus } from '@/composables/usePagePullStatus'
import { usePullRefresh } from '@/composables/usePullRefresh'

type AppPagePullStateOptions = {
  pageTitle: Ref<string>
  threshold?: number
}

export function useAppPagePullState(options: AppPagePullStateOptions) {
  const pullRefresh = usePullRefresh({ threshold: options.threshold ?? 52 })
  const completionRefreshing = ref(false)
  const statusRefreshing = computed(() => pullRefresh.refreshing.value || completionRefreshing.value)
  const contentMotion = usePageContentMotion({
    pullOffset: pullRefresh.offset,
    settling: pullRefresh.settling,
  })
  const status = usePagePullStatus({
    refreshing: statusRefreshing,
    progress: pullRefresh.distanceProgress,
    pageTitle: options.pageTitle,
  })

  function holdCompletionRefreshing() {
    completionRefreshing.value = true
  }

  function releaseCompletionRefreshing() {
    completionRefreshing.value = false
  }

  function reset() {
    releaseCompletionRefreshing()
    pullRefresh.reset()
    contentMotion.resetSideMotion()
    contentMotion.clearStretchAnchorIfIdle(false)
  }

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
    holdCompletionRefreshing,
    releaseCompletionRefreshing,
    settleOffset: pullRefresh.settleOffset,
    reset,
    resetMotion: pullRefresh.resetMotion,
    clearTimers: pullRefresh.clearTimers,
    setSideOffset: contentMotion.setSideOffset,
    setSideStretch: contentMotion.setSideStretch,
    resetSideMotion: contentMotion.resetSideMotion,
    clearStretchAnchorIfIdle: contentMotion.clearStretchAnchorIfIdle,
  }
}

export type AppPagePullState = ReturnType<typeof useAppPagePullState>
