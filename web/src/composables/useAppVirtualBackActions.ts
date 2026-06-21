type ReadableRef<T> = {
  readonly value: T
}

type DoubleBackGuard = {
  reset: () => void
  shouldConsumeBack: () => boolean
}

type AppVirtualBackActionsOptions = {
  navigationVisible: ReadableRef<boolean>
  sourceReaderOpen: ReadableRef<boolean>
  detailReaderOpen: ReadableRef<boolean>
  detailOpenedFromSourceReader: ReadableRef<boolean>
  isFeedRoute: ReadableRef<boolean>
  homeBackGuard: DoubleBackGuard
  hasParkedDetailSourceState: () => boolean
  detailCommittedListReturn: () => boolean
  sourceReaderShouldReturnToDetail: () => boolean
  closeNavigation: () => void
  collapseItemReader: () => void
  restoreSourceReaderBackTarget: () => void
  closeSourceReader: () => void
  goHome: (replace?: boolean) => void
}

export function useAppVirtualBackActions(options: AppVirtualBackActionsOptions) {
  function hasVirtualBackTarget() {
    return (
      options.navigationVisible.value ||
      options.hasParkedDetailSourceState() ||
      options.sourceReaderOpen.value ||
      options.detailReaderOpen.value ||
      (!options.isFeedRoute.value && !options.navigationVisible.value)
    )
  }

  function resetHomeGuard() {
    options.homeBackGuard.reset()
  }

  function runVirtualBackAnimation() {
    if (options.navigationVisible.value) {
      resetHomeGuard()
      options.closeNavigation()
      return true
    }

    if (
      options.detailReaderOpen.value &&
      options.detailOpenedFromSourceReader.value &&
      !options.detailCommittedListReturn()
    ) {
      resetHomeGuard()
      options.collapseItemReader()
      return true
    }

    if (options.sourceReaderShouldReturnToDetail()) {
      resetHomeGuard()
      options.restoreSourceReaderBackTarget()
      return true
    }

    if (options.sourceReaderOpen.value && !options.detailReaderOpen.value) {
      resetHomeGuard()
      options.closeSourceReader()
      return true
    }

    if (options.detailReaderOpen.value) {
      resetHomeGuard()
      options.collapseItemReader()
      return true
    }

    if (!options.isFeedRoute.value && !options.navigationVisible.value) {
      resetHomeGuard()
      options.goHome(false)
      return true
    }

    if (options.isFeedRoute.value) {
      return options.homeBackGuard.shouldConsumeBack()
    }

    return false
  }

  return {
    hasVirtualBackTarget,
    runVirtualBackAnimation,
  }
}
