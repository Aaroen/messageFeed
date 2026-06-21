import { useFeedViewSwipeController } from '@/composables/useFeedViewSwipeController'

type AppFeedViewSwipeInteractionsOptions<TSurface extends string> = Parameters<
  typeof useFeedViewSwipeController<TSurface>
>[0]

export function useAppFeedViewSwipeInteractions<TSurface extends string>(
  options: AppFeedViewSwipeInteractionsOptions<TSurface>,
) {
  return useFeedViewSwipeController<TSurface>(options)
}
