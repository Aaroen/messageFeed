import { useReaderDetailSourceTransitionRects } from '@/composables/useReaderDetailSourceTransitionRects'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderTransitionRectsOptions = Omit<
  Parameters<typeof useReaderDetailSourceTransitionRects>[0],
  'findFeedItemElement'
> & {
  activeFeedIndex: ReadableRef<number>
  findFeedItemElement: (itemID: number | undefined, activeFeedIndex: number) => Element | null
}

export function useAppReaderTransitionRects(options: AppReaderTransitionRectsOptions) {
  return useReaderDetailSourceTransitionRects({
    ...options,
    findFeedItemElement: (itemID) => options.findFeedItemElement(itemID, options.activeFeedIndex.value),
  })
}
