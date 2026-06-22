import { computed } from 'vue'

type ReadableRef<T> = {
  readonly value: T
}

type AppMainClassStateOptions = {
  isFeedRoute: ReadableRef<boolean>
}

export function useAppMainClassState(options: AppMainClassStateOptions) {
  const mainClass = computed(() => ({
    'app-main--feed': options.isFeedRoute.value,
    'app-main--page': !options.isFeedRoute.value,
  }))

  return {
    mainClass,
  }
}
