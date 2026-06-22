import type { RouteLocationNormalizedLoaded } from 'vue-router'

import { useAppPagePullInteractions } from '@/composables/useAppPagePullInteractions'
import { useAppPagePullState } from '@/composables/useAppPagePullState'
import { useAppRouteState } from '@/composables/useAppRouteState'

type PagePullInteractionOptions = Omit<
  Parameters<typeof useAppPagePullInteractions>[0],
  'pagePull' | 'isFeedRoute'
>

export function useAppRoutePageRuntime(route: RouteLocationNormalizedLoaded) {
  const routeState = useAppRouteState(route)
  const pagePullState = useAppPagePullState({ pageTitle: routeState.pageTitle })

  function installPagePullInteractions(options: PagePullInteractionOptions) {
    return useAppPagePullInteractions({
      ...options,
      pagePull: pagePullState,
      isFeedRoute: routeState.isFeedRoute,
    })
  }

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
    installPagePullInteractions,
  }
}
