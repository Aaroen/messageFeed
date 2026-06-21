import { useReaderSourceSubscription } from '@/composables/useReaderSourceSubscription'
import type { ReaderSource } from '@/composables/useReaderSession'

type ReadableRef<T> = {
  readonly value: T
}

type AppReaderSourceSubscriptionOptions = Omit<
  Parameters<typeof useReaderSourceSubscription>[0],
  'getReaderSource'
> & {
  readerSource: ReadableRef<ReaderSource | null>
}

export function useAppReaderSourceSubscription(options: AppReaderSourceSubscriptionOptions) {
  return useReaderSourceSubscription({
    ...options,
    getReaderSource: () => options.readerSource.value,
  })
}
