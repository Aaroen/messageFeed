import type { RouteLocationNormalizedLoaded } from 'vue-router'

import { useAppPagePullState } from '@/composables/useAppPagePullState'
import { useAppRouteState } from '@/composables/useAppRouteState'

export function useAppRoutePageRuntime(route: RouteLocationNormalizedLoaded) {
  const routeState = useAppRouteState(route)
  const pagePullState = useAppPagePullState({ pageTitle: routeState.pageTitle })

  return {
    selectedKeys: routeState.selectedKeys,
    pageTitle: routeState.pageTitle,
    isFeedRoute: routeState.isFeedRoute,
    cornerButtonLabel: routeState.cornerButtonLabel,
    pagePullState,
    pagePullOffset: pagePullState.offset,
    pagePullRefreshing: pagePullState.refreshing,
    pagePullProgress: pagePullState.progress,
    pageContentInnerStyle: pagePullState.contentStyle,
    pagePullStatusText: pagePullState.statusText,
    pagePullStatusMeta: pagePullState.statusMeta,
  }
}
