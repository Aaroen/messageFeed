import type { RouteLocationNormalizedLoaded } from 'vue-router'

import { useAppInteractionTargetGuards } from '@/composables/useAppInteractionTargetGuards'
import { useAppPagePullInteractions } from '@/composables/useAppPagePullInteractions'
import { useAppPagePullState } from '@/composables/useAppPagePullState'
import { useAppRouteState } from '@/composables/useAppRouteState'

type PagePullInteractionOptions = Omit<
  Parameters<typeof useAppPagePullInteractions>[0],
  'pagePull' | 'isFeedRoute' | 'isControlTarget'
>

export function useAppRoutePageRuntime(route: RouteLocationNormalizedLoaded) {
  const routeState = useAppRouteState(route)
  const pagePullState = useAppPagePullState({ pageTitle: routeState.pageTitle })
  const interactionTargetGuards = useAppInteractionTargetGuards()

  function installPagePullInteractions(options: PagePullInteractionOptions) {
    return useAppPagePullInteractions({
      ...options,
      pagePull: pagePullState,
      isFeedRoute: routeState.isFeedRoute,
      isControlTarget: interactionTargetGuards.isPageTopPullControlTarget,
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
