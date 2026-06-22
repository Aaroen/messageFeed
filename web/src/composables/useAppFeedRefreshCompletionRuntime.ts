import { useRefreshCompletionState } from '@/composables/useRefreshCompletionState'

type ReadableRef<T> = {
  readonly value: T
}

type AppFeedRefreshCompletionRuntimeOptions = {
  isFeedRoute: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  sourceReaderOpen: ReadableRef<boolean>
  sourceReaderVisible: ReadableRef<boolean>
  sourceReaderUnderDetail: ReadableRef<boolean>
  navigationVisible: ReadableRef<boolean>
}

type ApplyCompletionEffectsPayload = {
  wasSource: boolean
}

export function useAppFeedRefreshCompletionRuntime(
  options: AppFeedRefreshCompletionRuntimeOptions,
) {
  const refreshCompletion = useRefreshCompletionState()

  function canApplyCompletionEffects(payload: ApplyCompletionEffectsPayload) {
    if (payload.wasSource) {
      return (
        options.sourceReaderVisible.value &&
        !options.sourceReaderUnderDetail.value &&
        !options.navigationVisible.value
      )
    }

    return (
      options.isFeedRoute.value &&
      !options.detailReaderOpen.value &&
      !options.sourceReaderOpen.value &&
      !options.navigationVisible.value
    )
  }

  return {
    refreshCompletion,
    refreshStartedWithChrome: refreshCompletion.startedWithChrome,
    feedRefreshSettling: refreshCompletion.settling,
    recordRefreshStartedWithChrome: refreshCompletion.recordStartedWithChrome,
    resetRefreshCompletion: refreshCompletion.reset,
    canApplyCompletionEffects,
  }
}
