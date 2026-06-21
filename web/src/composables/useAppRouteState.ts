import { computed } from 'vue'
import type { RouteLocationNormalizedLoaded } from 'vue-router'

export function useAppRouteState(route: RouteLocationNormalizedLoaded) {
  const selectedKeys = computed(() => [route.name?.toString() ?? 'subscriptions'])
  const pageTitle = computed(() => route.meta.title?.toString() ?? '订阅')
  const isFeedRoute = computed(() => ['subscriptions', 'recommendations'].includes(route.name?.toString() ?? ''))
  const cornerButtonLabel = computed(() => '打开导航')

  return {
    selectedKeys,
    pageTitle,
    isFeedRoute,
    cornerButtonLabel,
  }
}
