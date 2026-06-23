import { describe, expect, it } from 'vitest'

import { resolveSourceTitleProgressState } from '@/composables/useReaderStackState'

describe('resolveSourceTitleProgressState', () => {
  it('keeps source title fully visible when detail/source transition is inactive', () => {
    expect(
      resolveSourceTitleProgressState({
        detailReaderOpen: false,
        sourceReaderVisible: true,
        detailCommittedListReturn: false,
        detailRestoringFromSourceReader: false,
        sourceNameMorphProgress: 0.2,
      }),
    ).toBe(1)
  })

  it('uses raw morph progress during normal source title morphing', () => {
    expect(
      resolveSourceTitleProgressState({
        detailReaderOpen: true,
        sourceReaderVisible: true,
        detailCommittedListReturn: false,
        detailRestoringFromSourceReader: false,
        sourceNameMorphProgress: 0.42,
      }),
    ).toBe(0.42)
  })

  it('delays title reveal while restoring detail from source reader', () => {
    expect(
      resolveSourceTitleProgressState({
        detailReaderOpen: true,
        sourceReaderVisible: true,
        detailCommittedListReturn: false,
        detailRestoringFromSourceReader: true,
        sourceNameMorphProgress: 0.74,
      }),
    ).toBe(0)
    expect(
      resolveSourceTitleProgressState({
        detailReaderOpen: true,
        sourceReaderVisible: true,
        detailCommittedListReturn: false,
        detailRestoringFromSourceReader: true,
        sourceNameMorphProgress: 0.87,
      }),
    ).toBeCloseTo(0.5)
    expect(
      resolveSourceTitleProgressState({
        detailReaderOpen: true,
        sourceReaderVisible: true,
        detailCommittedListReturn: false,
        detailRestoringFromSourceReader: true,
        sourceNameMorphProgress: 1,
      }),
    ).toBe(1)
  })
})
