import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type AppMainClassStateOptions = {
  isFeedRoute: ReadableRef<boolean>
  feedChromeHidden: ReadableRef<boolean>
  feedPullActive: ReadableRef<boolean>
  pagePullActive: ReadableRef<boolean>
  detailChromeVisible: ReadableRef<boolean>
}

export function useAppMainClassState(options: AppMainClassStateOptions) {
  const mainClass = computed(() => ({
    'app-main--feed': options.isFeedRoute.value,
    'app-main--page': !options.isFeedRoute.value,
    'app-main--tabs-hidden': options.feedChromeHidden.value,
    'app-main--refreshing': options.feedPullActive.value || options.pagePullActive.value,
    'app-main--detail-chrome': options.detailChromeVisible.value,
  }))

  return {
    mainClass,
  }
}
