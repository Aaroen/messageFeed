import { useAppShellMotion } from '@/composables/useAppShellMotion'
import { useFeedChromeLayoutState } from '@/composables/useFeedChromeLayoutState'
import { useFeedChromeVisibilityState } from '@/composables/useFeedChromeVisibilityState'
import { usePullActivityState } from '@/composables/usePullActivityState'

type AppFeedChromeStateOptions = {
  pullActivity: Parameters<typeof usePullActivityState>[0]
  layout: Omit<
    Parameters<typeof useFeedChromeLayoutState>[0],
    'feedPullActive' | 'pagePullActive' | 'pullProgress'
  >
  shellMotion: Omit<Parameters<typeof useAppShellMotion>[0], 'feedHeaderHeight' | 'feedContentSpace'>
  visibility: Omit<
    Parameters<typeof useFeedChromeVisibilityState>[0],
    'feedHeaderProgress' | 'feedPullActive' | 'feedHeaderReturnProgress'
  >
}

export function useAppFeedChromeState(options: AppFeedChromeStateOptions) {
  const pullActivity = usePullActivityState(options.pullActivity)
  const layout = useFeedChromeLayoutState({
    ...options.layout,
    feedPullActive: pullActivity.feedActive,
    pagePullActive: pullActivity.pageActive,
    pullProgress: pullActivity.feedProgress,
  })
  const shellMotion = useAppShellMotion({
    ...options.shellMotion,
    feedHeaderHeight: layout.headerHeight,
    feedContentSpace: layout.contentSpace,
  })
  const visibility = useFeedChromeVisibilityState({
    ...options.visibility,
    feedHeaderProgress: layout.headerProgress,
    feedPullActive: pullActivity.feedActive,
    feedHeaderReturnProgress: layout.headerReturnProgress,
  })

  return {
    feedPullActive: pullActivity.feedActive,
    pagePullActive: pullActivity.pageActive,
    sourcePullActive: pullActivity.sourceActive,
    feedOrSourcePullActive: pullActivity.feedOrSourceActive,
    pullProgress: pullActivity.feedProgress,
    sourcePullProgress: pullActivity.sourceProgress,
    feedHeaderHeight: layout.headerHeight,
    feedHeaderProgress: layout.headerProgress,
    freezeFeedBodyDuringTopRefresh: layout.freezeBodyDuringTopRefresh,
    feedHeaderReturnProgress: layout.headerReturnProgress,
    mainStyle: shellMotion.style,
    feedContentStyle: shellMotion.feedContentStyle,
    pageContentStyle: shellMotion.pageContentStyle,
    feedCornerHidden: visibility.feedCornerHidden,
    detailHeaderVisible: visibility.detailHeaderVisible,
    headerDetailLayoutActive: visibility.headerDetailLayoutActive,
  }
}
